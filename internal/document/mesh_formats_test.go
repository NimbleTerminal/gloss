package document

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"nimbleterminal.dev/nt3d/mesh"
)

func TestDetectOBJAndPLY(t *testing.T) {
	for name, data := range map[string]string{
		"model.obj": "v 0 0 0\n",
		"scan.PLY":  "ply\nformat ascii 1.0\n",
		"-":         "ply\r\nformat ascii 1.0\n",
	} {
		want := "obj"
		if strings.HasPrefix(data, "ply") {
			want = "ply"
		}
		if kind, err := Detect(name, []byte(data), ""); err != nil || kind != want {
			t.Errorf("Detect(%q) = %q, %v; want %q", name, kind, err, want)
		}
	}
}

const triangleOBJ = "mtllib %s\nv 0 0 0\nv 1 0 0\nv 0 1 0\nvt 0 0\nvt 1 0\nvt 0 1\nusemtl red\nf 1/1 2/2 3/3\n"

func pngOf(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// hugePNG is only a PNG header declaring w × h, enough for DecodeConfig.
func hugePNG(w, h uint32) []byte {
	ihdr := make([]byte, 17)
	copy(ihdr, "IHDR")
	binary.BigEndian.PutUint32(ihdr[4:], w)
	binary.BigEndian.PutUint32(ihdr[8:], h)
	ihdr[12], ihdr[13] = 8, 6 // 8-bit RGBA.
	out := append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0d"), ihdr...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(ihdr))
}

// An OBJ's materials color its faces and its PNG or JPEG textures are drawn;
// a texture too large, of another kind, or outside the model's folder is not
// read, and says so.
func TestOBJMaterials(t *testing.T) {
	red, green := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 200, 0, 255}
	mtl := func(texture string) []byte { return []byte("newmtl red\nKd 1 0 0\nmap_Kd " + texture + "\n") }
	obj := func(library string) []byte { return []byte(strings.Replace(triangleOBJ, "%s", library, 1)) }
	files := fstest.MapFS{
		"models/tri.obj":   {Data: obj("tri.mtl")},
		"models/tri.mtl":   {Data: mtl("skin.png")},
		"models/skin.png":  {Data: pngOf(t, 4, 4, green)},
		"models/big.obj":   {Data: obj("big.mtl")},
		"models/big.mtl":   {Data: mtl("big.png")},
		"models/big.png":   {Data: hugePNG(8192, 8192)},
		"models/tga.obj":   {Data: obj("tga.mtl")},
		"models/tga.mtl":   {Data: mtl("skin.tga")},
		"models/skin.tga":  {Data: []byte("not read")},
		"models/up.obj":    {Data: obj("../secret.mtl")},
		"secret.mtl":       {Data: []byte("newmtl red\nKd 1 0 0\n")},
		"models/plain.obj": {Data: []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")},
	}
	l := &Loader{Files: files}
	defer l.Close()
	r := l.Load(Request{Path: "models/tri.obj", Page: 1})
	if r.Err != nil || r.Kind != "obj" || r.Mesh == nil {
		t.Fatalf("load: %q %v", r.Kind, r.Err)
	}
	if len(r.Mesh.textured) != 1 || r.Mesh.Triangles() != 1 || !r.Mesh.HasColors() {
		t.Fatalf("textured parts %d, triangles %d", len(r.Mesh.textured), r.Mesh.Triangles())
	}
	if !strings.Contains(fieldText(r.Info), "Textured parts=1") {
		t.Errorf("info %s does not count the textured part", fieldText(r.Info))
	}
	img, err := ExportImage(r, 64)
	if err != nil {
		t.Fatal(err)
	}
	if c := color.RGBAModel.Convert(img.At(20, 40)).(color.RGBA); c.G < c.R || c.G < 60 {
		t.Fatalf("the texture is not drawn: %v", c)
	}
	// Painting every face leaves the texture as it is.
	r.Mesh.Recolor(color.RGBA{0, 0, 255, 255}, true)
	img, err = ExportImage(r, 64)
	if err != nil {
		t.Fatal(err)
	}
	if c := color.RGBAModel.Convert(img.At(20, 40)).(color.RGBA); c.G < c.B {
		t.Fatalf("painting covered the texture: %v", c)
	}
	for path, note := range map[string]string{"models/big.obj": "megapixels", "models/tga.obj": "only PNG and JPEG", "models/up.obj": "only files beside the model"} {
		r := l.Load(Request{Path: path, Page: 1})
		if r.Err != nil || len(r.Mesh.textured) != 0 || !strings.Contains(fieldText(r.Info), note) {
			t.Errorf("%s: %v %s", path, r.Err, fieldText(r.Info))
		}
	}
	if big := l.Load(Request{Path: "models/big.obj", Page: 1}); big.Mesh.Geometry().Vertices[0].Color != red {
		t.Error("a face whose texture was refused lost its material color")
	}
	if stdin := l.Load(Request{Path: "models/tri.obj", Page: 1, Stdin: true}); stdin.Err != nil || stdin.Mesh.HasColors() {
		t.Fatal("standard input read a side file")
	}
	if plain := l.Load(Request{Path: "models/plain.obj", Page: 1}); plain.Err != nil || plain.Mesh.Triangles() != 1 {
		t.Fatalf("plain OBJ: %v", plain.Err)
	}
}

func fieldText(fields []Field) string {
	var b strings.Builder
	for _, f := range fields {
		b.WriteString(f.Label + "=" + f.Value + ";")
	}
	return b.String()
}

// OBJ and PLY say Y is up; gloss turns them Z-up like STL and 3MF.
func TestOBJIsTurnedZUp(t *testing.T) {
	m, err := ParseOBJ("tall.obj", []byte("v 0 0 0\nv 1 0 0\nv 0 5 0\nf 1 2 3\n"), nil, true, nt3dmesh.YUp)
	if err != nil {
		t.Fatal(err)
	}
	if d := m.geometry.Bounds.Max.Sub(m.geometry.Bounds.Min); d.Z != 5 || d.Y != 0 {
		t.Fatalf("extent %v: Y-up height should become Z", d)
	}
}

// A PLY without faces is a cloud of points, drawn as discs that about touch.
func TestPLYPointCloud(t *testing.T) {
	var b strings.Builder
	b.WriteString("ply\nformat ascii 1.0\nelement vertex 100\nproperty float x\nproperty float y\nproperty float z\nend_header\n")
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			b.WriteString(strconv.Itoa(i) + " 0 " + strconv.Itoa(j) + "\n")
		}
	}
	m, err := ParsePLY([]byte(b.String()), nt3dmesh.YUp)
	if err != nil {
		t.Fatal(err)
	}
	if m.Points() != 100 || m.Triangles() != 0 {
		t.Fatalf("points %d triangles %d", m.Points(), m.Triangles())
	}
	// Spacing is 1 over a 9 × 9 square: discs of about half that.
	if m.pointRadius < .4 || m.pointRadius > .7 {
		t.Fatalf("radius %v", m.pointRadius)
	}
	img, err := ExportImage(Result{Kind: "ply", Mesh: m}, 64)
	if err != nil {
		t.Fatalf("the cloud does not render: %v", err)
	}
	if r, g, b, _ := img.At(32, 32).RGBA(); r>>8 > 200 && g>>8 > 200 && b>>8 > 200 {
		t.Fatal("the middle of the cloud is empty: its discs do not touch")
	}
	if !strings.Contains(fieldText(meshFields(nil, m, Locale{})), "Points=100") {
		t.Fatal("info does not count points")
	}
	if _, err := ParsePLY([]byte("ply\nformat ascii 1.0\nelement vertex 0\nproperty float x\nend_header\n"), nt3dmesh.YUp); err == nil {
		t.Fatal("an empty PLY loaded")
	}
}

func TestExampleMeshes(t *testing.T) {
	l := &Loader{}
	defer l.Close()
	for path, want := range map[string]string{"../../examples/cottage.obj": "Textured parts=1", "../../examples/dunes.ply": "Points=2,304"} {
		r := l.Load(Request{Path: path, Page: 1})
		if r.Err != nil || r.Mesh == nil || !strings.Contains(fieldText(r.Info), want) {
			t.Fatalf("%s: %v %s", path, r.Err, fieldText(r.Info))
		}
		if _, err := ExportImage(r, 128); err != nil {
			t.Fatalf("%s export: %v", path, err)
		}
	}
	cottage := l.Load(Request{Path: "../../examples/cottage.obj", Page: 1})
	if !cottage.Mesh.HasColors() {
		t.Fatal("the cottage lost its materials")
	}
}

// --up z keeps a Z-up OBJ standing; --up y stands up a Y-up STL.
func TestUpAxisOption(t *testing.T) {
	tall := []byte("v 0 0 0\nv 1 0 0\nv 0 0 5\nf 1 2 3\n")
	stl := []byte("solid t\nfacet normal 0 0 0\nouter loop\nvertex 0 0 0\nvertex 1 0 0\nvertex 0 5 0\nendloop\nendfacet\nendsolid t\n")
	l := &Loader{Files: fstest.MapFS{"tall.obj": {Data: tall}, "tall.stl": {Data: stl}}}
	defer l.Close()
	height := func(path, up string) float32 {
		r := l.Load(Request{Path: path, Page: 1, Up: up})
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		return r.Mesh.geometry.Bounds.Max.Z - r.Mesh.geometry.Bounds.Min.Z
	}
	if h := height("tall.obj", ""); h != 0 {
		t.Fatalf("a Z-up OBJ read as Y-up should lie flat, height %v", h)
	}
	if h := height("tall.obj", "z"); h != 5 {
		t.Fatalf("--up z: height %v, want 5", h)
	}
	if h := height("tall.stl", ""); h != 0 {
		t.Fatalf("an STL is Z-up by default, height %v", h)
	}
	if h := height("tall.stl", "y"); h != 5 {
		t.Fatalf("--up y: STL height %v, want 5", h)
	}
	if r := l.Load(Request{Path: "tall.obj", Page: 1, Up: "z"}); !strings.Contains(fieldText(r.Info), "Up axis=Z") {
		t.Fatalf("info does not say the up axis: %s", fieldText(r.Info))
	}
}
