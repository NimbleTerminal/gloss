// Regenerate the original raster and PDF fixtures: go run ./scripts/gen-assets.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"strings"

	"github.com/gen2brain/h265/heic"
	"github.com/gen2brain/jxl"
)

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, 640, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 640; x++ {
			t := float64(y) / 400
			c := color.NRGBA{uint8(25 + 70*t), uint8(34 + 80*t), uint8(80 + 90*t), 255}
			if math.Hypot(float64(x-470), float64(y-110)) < 48 {
				c = color.NRGBA{255, 194, 110, 255}
			}
			for k := 0; k < 3; k++ {
				ridge := 220 + float64(k*45) + 45*math.Sin(float64(x)/90+float64(k)*2) + 25*math.Cos(float64(x)/43+float64(k))
				if float64(y) > ridge {
					c = []color.NRGBA{{78, 85, 131, 255}, {41, 64, 101, 255}, {17, 39, 63, 255}}[k]
				}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	write("examples/motion.gif", func(b *bytes.Buffer) error { return motionGIF(b) })
	// Go has no WebP encoder: the animated WebP is the GIF converted by libwebp.
	if _, err := exec.LookPath("gif2webp"); err != nil {
		fmt.Fprintln(os.Stderr, "gif2webp not found; examples/motion.webp left as is")
	} else if out, err := exec.Command("gif2webp", "-quiet", "-min_size", "-metadata", "none", "examples/motion.gif", "-o", "examples/motion.webp").CombinedOutput(); err != nil {
		panic(fmt.Sprintf("gif2webp: %v\n%s", err, out))
	}
	write("examples/landscape.png", func(b *bytes.Buffer) error { return png.Encode(b, img) })
	write("examples/landscape.heic", func(b *bytes.Buffer) error { return heic.Encode(b, desert(), heic.EncodeOptions{Quality: 85}) })
	// The same hills as the PNG, losslessly, so the two must decode alike.
	write("examples/landscape.jxl", func(b *bytes.Buffer) error { return jxl.Encode(b, img, jxl.EncodeOptions{Lossless: true}) })
	if err := os.WriteFile("examples/field-guide.pdf", pdf(), 0644); err != nil {
		panic(err)
	}
	if err := os.WriteFile("examples/gloss.stl", blockWord(), 0644); err != nil {
		panic(err)
	}
	write("examples/cottage-roof.png", func(b *bytes.Buffer) error { return png.Encode(b, shingles()) })
	obj, mtl := cottage()
	for name, data := range map[string][]byte{"examples/cottage.obj": obj, "examples/cottage.mtl": mtl, "examples/dunes.ply": dunes(), "examples/islands.pmtiles": islands()} {
		if err := os.WriteFile(name, data, 0644); err != nil {
			panic(err)
		}
	}
}

// cottage is a small house in OBJ, Y-up as most tools write it: an object
// per part, each with a material from cottage.mtl.
func cottage() (obj, mtl []byte) {
	var b bytes.Buffer
	b.WriteString("# An original cottage for gloss's examples.\nmtllib cottage.mtl\n")
	n := 0
	box := func(name, material string, x0, y0, z0, x1, y1, z1 float64) {
		fmt.Fprintf(&b, "o %s\nusemtl %s\n", name, material)
		for _, c := range [][3]float64{{x0, y0, z0}, {x1, y0, z0}, {x1, y0, z1}, {x0, y0, z1}, {x0, y1, z0}, {x1, y1, z0}, {x1, y1, z1}, {x0, y1, z1}} {
			fmt.Fprintf(&b, "v %g %g %g\n", c[0], c[1], c[2])
		}
		for _, f := range [][4]int{{1, 2, 3, 4}, {5, 8, 7, 6}, {1, 5, 6, 2}, {2, 6, 7, 3}, {3, 7, 8, 4}, {4, 8, 5, 1}} {
			fmt.Fprintf(&b, "f %d %d %d %d\n", n+f[0], n+f[1], n+f[2], n+f[3])
		}
		n += 8
	}
	box("walls", "plaster", -2, 0, -1.5, 2, 2, 1.5)
	box("door", "wood", -.4, 0, 1.5, .4, 1.3, 1.6)
	box("chimney", "brick", 1, 2, -.8, 1.5, 3.4, -.3)
	// The roof is a prism: two slopes, shingled by a texture that repeats
	// along them, and two plaster gables.
	b.WriteString("o roof\n")
	for _, c := range [][3]float64{{-2.3, 2, -1.8}, {2.3, 2, -1.8}, {2.3, 2, 1.8}, {-2.3, 2, 1.8}, {-2.3, 3.2, 0}, {2.3, 3.2, 0}} {
		fmt.Fprintf(&b, "v %g %g %g\n", c[0], c[1], c[2])
	}
	b.WriteString("vt 0 0\nvt 4 0\nvt 4 2\nvt 0 2\nusemtl tile\n")
	for _, f := range [][4]int{{4, 3, 6, 5}, {2, 1, 5, 6}} {
		fmt.Fprintf(&b, "f %d/1 %d/2 %d/3 %d/4\n", n+f[0], n+f[1], n+f[2], n+f[3])
	}
	b.WriteString("usemtl plaster\n")
	for _, f := range [][]int{{1, 4, 5}, {3, 2, 6}, {1, 2, 3, 4}} {
		b.WriteString("f")
		for _, i := range f {
			fmt.Fprintf(&b, " %d", n+i)
		}
		b.WriteString("\n")
	}
	mtl = []byte("# Materials for cottage.obj.\n" +
		"newmtl plaster\nKd 0.93 0.89 0.80\n" +
		"newmtl wood\nKd 0.45 0.28 0.16\n" +
		"newmtl brick\nKd 0.62 0.25 0.20\n" +
		"newmtl tile\nKd 0.30 0.36 0.48\nmap_Kd cottage-roof.png\n")
	return b.Bytes(), mtl
}

// shingles is the cottage roof's texture: staggered rows of slate tiles,
// each a little different in shade.
func shingles() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		row := y / 16
		for x := 0; x < 64; x++ {
			col := (x + row%2*8) / 16
			shade := 40 + 18*((row*3+col*5)%4)
			c := color.NRGBA{uint8(shade), uint8(shade + 12), uint8(shade + 40), 255}
			if y%16 >= 14 || (x+row%2*8)%16 == 0 {
				c = color.NRGBA{22, 26, 36, 255} // The gaps between tiles.
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// dunes is a cloud of points sampled over sand dunes, in binary PLY with a
// color for each point, Y-up.
func dunes() []byte {
	const side = 48
	var b bytes.Buffer
	fmt.Fprintf(&b, "ply\nformat binary_little_endian 1.0\ncomment An original point cloud for gloss's examples.\nelement vertex %d\nproperty float x\nproperty float y\nproperty float z\nproperty uchar red\nproperty uchar green\nproperty uchar blue\nend_header\n", side*side)
	for i := 0; i < side; i++ {
		for j := 0; j < side; j++ {
			x, z := float64(i)/side*8-4, float64(j)/side*8-4
			y := .6*math.Sin(x*.9+math.Sin(z*.5)) + .25*math.Cos(z*1.3)
			t := (y + .85) / 1.7
			for _, v := range []float32{float32(x), float32(y), float32(z)} {
				_ = binary.Write(&b, binary.LittleEndian, v)
			}
			b.Write([]byte{uint8(150 + 90*t), uint8(95 + 90*t), uint8(55 + 60*t)})
		}
	}
	return b.Bytes()
}

// A warm desert scene contrasts with the PNG's blue hills at dusk.
func desert() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 640, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 640; x++ {
			c := color.NRGBA{35, 155, 163, 255}
			if math.Hypot(float64(x-155), float64(y-115)) < 65 {
				c = color.NRGBA{255, 226, 158, 255}
			}
			for k, sand := range []color.NRGBA{{239, 176, 107, 255}, {215, 115, 69, 255}, {156, 66, 49, 255}} {
				ridge := 245 + float64(k*55) + 35*math.Sin(float64(x)/130+float64(k)*2)
				if float64(y) > ridge {
					c = sand
				}
			}
			// Saguaro silhouette with two raised arms.
			if (x >= 458 && x < 482 && y >= 155 && y < 337) ||
				(x >= 420 && x < 443 && y >= 188 && y < 252) ||
				(x >= 420 && x < 470 && y >= 232 && y < 252) ||
				(x >= 500 && x < 522 && y >= 172 && y < 224) ||
				(x >= 470 && x < 522 && y >= 204 && y < 224) {
				c = color.NRGBA{23, 70, 64, 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// Each pixel in an original 5x7 alphabet becomes a closed raised block.
// Separate blocks leave narrow seams that make the sculpture's depth visible.
func blockWord() []byte {
	letters := []string{
		"01110/10001/10000/10111/10001/10001/01110",
		"10000/10000/10000/10000/10000/10000/11111",
		"01110/10001/10001/10001/10001/10001/01110",
		"01111/10000/10000/01110/00001/00001/11110",
		"01111/10000/10000/01110/00001/00001/11110",
	}
	var b bytes.Buffer
	b.WriteString("solid gloss_blocks\n")
	faces := [][4]int{{0, 3, 2, 1}, {4, 5, 6, 7}, {0, 1, 5, 4}, {3, 7, 6, 2}, {0, 4, 7, 3}, {1, 2, 6, 5}}
	for letter, glyph := range letters {
		for row, line := range strings.Split(glyph, "/") {
			for col, bit := range line {
				if bit != '1' {
					continue
				}
				x, y := float64(letter*6+col)-14.5, float64(6-row)-3.5
				// A little variation in relief catches the directional light.
				z := 1.3 + .15*float64((row+col)%3)
				v := [][3]float64{{x + .06, y + .06, 0}, {x + .94, y + .06, 0}, {x + .94, y + .94, 0}, {x + .06, y + .94, 0}, {x + .06, y + .06, z}, {x + .94, y + .06, z}, {x + .94, y + .94, z}, {x + .06, y + .94, z}}
				for _, face := range faces {
					for _, tri := range [][3]int{{face[0], face[1], face[2]}, {face[0], face[2], face[3]}} {
						b.WriteString("facet normal 0 0 0\nouter loop\n")
						for _, i := range tri {
							fmt.Fprintf(&b, "vertex %.3f %.3f %.3f\n", -math.Sin(math.Pi/180*25)*v[i][0]+math.Cos(math.Pi/180*25)*v[i][2], math.Cos(math.Pi/180*25)*v[i][0]+math.Sin(math.Pi/180*25)*v[i][2], v[i][1])
						}
						b.WriteString("endloop\nendfacet\n")
					}
				}
			}
		}
	}
	b.WriteString("endsolid gloss_blocks\n")
	return b.Bytes()
}

func write(path string, encode func(*bytes.Buffer) error) {
	var b bytes.Buffer
	if err := encode(&b); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0644); err != nil {
		panic(err)
	}
}

func pdf() []byte {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 640 400] /Resources << /Font << /F1 5 0 R >> >> /Contents 6 0 R >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 640 400] /Resources << /Font << /F1 5 0 R >> >> /Contents 7 0 R >>", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"}
	for i, title := range []string{"A FIELD GUIDE TO GLOSS", "SMALL FILES. MANY VIEWS."} {
		stream := fmt.Sprintf("0.07 0.10 0.16 rg 0 0 640 400 re f\n0.4 0.85 0.7 rg 40 330 70 5 re f\n1 1 1 rg BT /F1 28 Tf 40 280 Td (%s) Tj ET\n0.7 0.75 0.85 rg BT /F1 16 Tf 40 245 Td (Images, vectors, documents and meshes.) Tj ET\n0.4 0.85 0.7 rg 40 100 140 90 re f\n0.45 0.55 0.95 rg 205 100 160 90 re f\n1 0.65 0.4 rg 390 100 210 90 re f\n1 1 1 rg BT /F1 12 Tf 40 40 Td (PAGE %d / 2  -  Press n and p to turn pages) Tj ET\n", title, i+1)
		objects = append(objects, fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream))
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, off := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return b.Bytes()
}
