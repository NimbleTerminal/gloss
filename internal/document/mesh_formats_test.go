package document

import (
	"image/color"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
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

const triangleOBJ = "mtllib %s\nv 0 0 0\nv 1 0 0\nv 0 1 0\nusemtl red\nf 1 2 3\n"

// An OBJ's materials color its faces; a texture or a file outside the
// model's folder is not read, and says so.
func TestOBJMaterials(t *testing.T) {
	red := color.RGBA{255, 0, 0, 255}
	files := fstest.MapFS{
		"models/tri.obj":   {Data: []byte(strings.Replace(triangleOBJ, "%s", "tri.mtl", 1))},
		"models/tri.mtl":   {Data: []byte("newmtl red\nKd 1 0 0\nmap_Kd skin.png\n")},
		"models/up.obj":    {Data: []byte(strings.Replace(triangleOBJ, "%s", "../secret.mtl", 1))},
		"secret.mtl":       {Data: []byte("newmtl red\nKd 1 0 0\n")},
		"models/skin.png":  {Data: []byte("not read")},
		"models/plain.obj": {Data: []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")},
	}
	l := &Loader{Files: files}
	defer l.Close()
	r := l.Load(Request{Path: "models/tri.obj", Page: 1})
	if r.Err != nil || r.Kind != "obj" || r.Mesh == nil {
		t.Fatalf("load: %q %v", r.Kind, r.Err)
	}
	if c := r.Mesh.Geometry().Vertices[0].Color; c != red || !r.Mesh.HasColors() {
		t.Fatalf("material color %v, want red", c)
	}
	info := fieldText(r.Info)
	for _, want := range []string{"Format=OBJ", "Triangles=1", "textures are not shown"} {
		if !strings.Contains(info, want) {
			t.Errorf("info %s lacks %q", info, want)
		}
	}
	up := l.Load(Request{Path: "models/up.obj", Page: 1})
	if up.Err != nil || up.Mesh.HasColors() || !strings.Contains(fieldText(up.Info), "only files beside the model") {
		t.Fatalf("a library outside the folder was read: %v %s", up.Err, fieldText(up.Info))
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
	m, err := ParseOBJ("tall.obj", []byte("v 0 0 0\nv 1 0 0\nv 0 5 0\nf 1 2 3\n"), nil, true)
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
	m, err := ParsePLY([]byte(b.String()))
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
	if !strings.Contains(fieldText(meshFields(nil, m)), "Points=100") {
		t.Fatal("info does not count points")
	}
	if _, err := ParsePLY([]byte("ply\nformat ascii 1.0\nelement vertex 0\nproperty float x\nend_header\n")); err == nil {
		t.Fatal("an empty PLY loaded")
	}
}

func TestExampleMeshes(t *testing.T) {
	l := &Loader{}
	defer l.Close()
	for path, want := range map[string]string{"../../examples/cottage.obj": "Objects=4", "../../examples/dunes.ply": "Points=2,304"} {
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
