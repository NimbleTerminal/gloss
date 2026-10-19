package document

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"nimbleterminal.dev/nt3d"
	math3d "nimbleterminal.dev/nt3d/math"
)

// ExportImage produces an opaque image bounded by maxEdge on both axes.
// Raster sources are never enlarged; vector documents render at the target.
func ExportImage(r Result, maxEdge int) (image.Image, error) {
	if maxEdge < 1 || maxEdge > 4096 {
		return nil, fmt.Errorf("max edge must be 1..4096")
	}
	if r.Err != nil {
		return nil, r.Err
	}
	if r.Markdown != nil {
		return nil, fmt.Errorf("Markdown PNG export is not supported; supply its text directly to the model")
	}
	if r.Mesh != nil {
		return exportMesh(r, maxEdge)
	}
	if r.Image == nil || r.Image.Bounds().Empty() {
		return nil, fmt.Errorf("document has no image")
	}
	b := r.Image.Bounds()
	w, h, err := VisionSize(b.Dx(), b.Dy(), maxEdge)
	if err != nil {
		return nil, err
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(out, out.Bounds(), r.Image, b, draw.Over, nil)
	return out, nil
}

// exportMesh draws the mesh from the viewer's camera, or from each view
// asked for. Several views make a sheet: square tiles, as many across as
// down or one more, each named in its corner.
func exportMesh(r Result, maxEdge int) (image.Image, error) {
	shots, names := []shot{{nt3d.DefaultCamera(), worldLight}}, []string{""}
	named := false
	switch {
	case r.Camera != nil:
		shots[0].camera = *r.Camera
	case len(r.Views) > 0:
		shots, names, named = nil, nil, true
		for _, v := range r.Views {
			shots, names = append(shots, shot{v.Camera(r.Mesh), v.light()}), append(names, v.Name)
		}
	}
	cols := int(math.Ceil(math.Sqrt(float64(len(shots)))))
	rows := (len(shots) + cols - 1) / cols
	// The sheet as a whole keeps to the edge.
	tile := maxEdge / cols
	w, h, err := VisionSize(cols*tile, rows*tile, maxEdge)
	if err != nil {
		return nil, err
	}
	if tile = min(w/cols, h/rows); tile < 1 {
		return nil, fmt.Errorf("%d views do not fit %d pixels", len(shots), maxEdge)
	}
	var tiles []image.Image
	var depths [][]float32
	if !r.CPU {
		if drawn, ok := gpuSnapshot(r.Mesh, tile, shots); ok && len(drawn) == len(shots) {
			tiles = drawn
		}
	}
	if tiles == nil {
		for _, s := range shots {
			frame, err := softwareSnapshot(r.Mesh, tile, s.camera, s.light)
			if err != nil {
				return nil, err
			}
			tiles = append(tiles, frame.Image)
			depths = append(depths, frame.Depth)
		}
	}
	// Faces that look the same way are shaded alike, so that from straight
	// ahead a hole, or the floor of a box seen from the top, vanishes into
	// what is behind it. Views asked for by name have their edges drawn.
	// A cloud of points has no surface to find edges in.
	if named && r.Mesh.Triangles() > 0 {
		if depths == nil {
			for _, s := range shots {
				frame, err := softwareSnapshot(r.Mesh, tile, s.camera, s.light)
				if err != nil {
					return nil, err
				}
				depths = append(depths, frame.Depth)
			}
		}
		for i, img := range tiles {
			tiles[i] = outline(img, depths[i], tile)
		}
	}
	if len(tiles) == 1 {
		return tiles[0], nil
	}
	sheet := image.NewRGBA(image.Rect(0, 0, cols*tile, rows*tile))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for i, img := range tiles {
		at := image.Rect(0, 0, tile, tile).Add(image.Pt(i%cols*tile, i/cols*tile))
		draw.Draw(sheet, at, img, img.Bounds().Min, draw.Src)
		label(sheet, at, names[i])
	}
	return sheet, nil
}

// label names a tile in its top-left corner, in letters large enough to be
// read in a sheet shown small.
func label(sheet *image.RGBA, tile image.Rectangle, name string) {
	face := basicfont.Face7x13
	text := image.NewRGBA(image.Rect(0, 0, len(name)*face.Advance, face.Height))
	(&font.Drawer{Dst: text, Src: image.NewUniform(color.RGBA{R: 40, G: 40, B: 40, A: 255}), Face: face, Dot: fixed.P(0, face.Ascent)}).DrawString(name)
	scale := max(1, tile.Dx()/200)
	margin := max(2, tile.Dx()/40)
	at := image.Rect(0, 0, text.Bounds().Dx()*scale, text.Bounds().Dy()*scale).Add(tile.Min).Add(image.Pt(margin, margin)).Intersect(tile)
	xdraw.NearestNeighbor.Scale(sheet, at, text, text.Bounds(), draw.Over, nil)
}

// shot is one picture of a mesh: where the camera is, and the light.
type shot struct {
	camera nt3d.Camera
	light  math3d.Vec3 // Toward the light.
}

// worldLight is where the viewer has its light: nt3d's own.
var worldLight = (math3d.Vec3{X: 1, Y: -1, Z: 2}).Normalize()

// gpuSnapshot draws the mesh alone, on white, with nt3d's GPU renderer:
// once for each shot, from one upload of the mesh. It reports false where
// there is no GPU to draw with. It is a variable so that tests can stand in
// for the hardware.
var gpuSnapshot = func(mesh *Mesh, size int, shots []shot) ([]image.Image, bool) {
	scene, err := mesh.Scene()
	if err != nil {
		return nil, false
	}
	renderer := nt3d.NewGPURenderer()
	defer renderer.Close()
	var out []image.Image
	for _, s := range shots {
		frame, err := scene.Frame(s.camera, size, size)
		if err != nil {
			return nil, false
		}
		frame.Background = color.RGBA{255, 255, 255, 255}
		frame.Lighting = nt3d.DefaultLighting()
		frame.Lighting.Lights[0].Direction = s.light
		img, mode, err := nt3d.Snapshot(frame, nt3d.WebGPU, renderer)
		if err != nil || mode != nt3d.WebGPU {
			return nil, false
		}
		out = append(out, img)
	}
	return out, true
}

// softwareSnapshot uses nt3d's renderer and depth buffer for CPU exports.
// Large square contact-sheet tiles render at the largest square within its
// frame-pixel budget, then scale back to the requested output size.
func softwareSnapshot(mesh *Mesh, size int, camera nt3d.Camera, light math3d.Vec3) (nt3d.SoftwareResult, error) {
	scene, err := mesh.Scene()
	if err != nil {
		return nt3d.SoftwareResult{}, err
	}
	renderSize := min(size, int(math.Sqrt(float64(nt3d.MaxFramePixels))))
	frame, err := scene.Frame(camera, renderSize, renderSize)
	if err != nil {
		return nt3d.SoftwareResult{}, err
	}
	frame.Background = color.RGBA{255, 255, 255, 255}
	frame.Lighting = nt3d.DefaultLighting()
	frame.Lighting.Lights[0].Direction = light
	frame.Complete = true
	result, err := nt3d.RenderSoftware(frame)
	if err != nil || renderSize == size {
		return result, err
	}
	up := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(up, up.Bounds(), result.Image, result.Image.Bounds(), draw.Src, nil)
	depth := make([]float32, size*size)
	for y := range size {
		for x := range size {
			depth[y*size+x] = result.Depth[(y*renderSize/size)*renderSize+x*renderSize/size]
		}
	}
	return nt3d.SoftwareResult{Image: up, Depth: depth, Width: size, Height: size}, nil
}

// renderMesh keeps camera and light policy at the Gloss export boundary.
func renderMesh(mesh *Mesh, size int, camera nt3d.Camera, light math3d.Vec3) (image.Image, error) {
	result, err := softwareSnapshot(mesh, size, camera, light)
	if err != nil {
		return nil, err
	}
	return result.Image, nil
}

// outline darkens the edges of a mesh's picture, in the colors of its faces,
// found in its depth: where
// the mesh meets the background, and where the depth breaks or folds, as at
// the rim of a hole or of a box. A slope, however steep, changes depth
// evenly, and is not an edge.
func outline(src image.Image, depth []float32, size int) image.Image {
	shaded := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(shaded, shaded.Bounds(), src, src.Bounds().Min, draw.Src)
	img := image.NewRGBA(shaded.Bounds())
	copy(img.Pix, shaded.Pix)
	near, far := float32(2), float32(-1)
	for _, z := range depth {
		if z <= 1 {
			near, far = min(near, z), max(far, z)
		}
	}
	if near > far {
		return img
	}
	step := max((far-near)*.02, 1e-5)
	at := func(x, y int) float32 {
		if x < 0 || y < 0 || x >= size || y >= size {
			return 2
		}
		return depth[y*size+x]
	}
	edge := func(x, y int) bool {
		z := at(x, y)
		if z > 1 {
			return false
		}
		l, r, u, d := at(x-1, y), at(x+1, y), at(x, y-1), at(x, y+1)
		if max(l, r, u, d) > 1 {
			return true
		}
		return abs32(l+r-2*z) > step || abs32(u+d-2*z) > step
	}
	width := max(1, size/500)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !edge(x, y) {
				continue
			}
			for dy := 0; dy < width; dy++ {
				for dx := 0; dx < width; dx++ {
					if x+dx < size && y+dy < size && at(x+dx, y+dy) <= 1 {
						c := shaded.RGBAAt(x+dx, y+dy)
						img.SetRGBA(x+dx, y+dy, color.RGBA{R: c.R / 4, G: c.G / 4, B: c.B / 4, A: 255})
					}
				}
			}
		}
	}
	return img
}

func abs32(v float32) float32 { return max(v, -v) }
