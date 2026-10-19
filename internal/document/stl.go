package document

import (
	"errors"
	"fmt"
	"image/color"
	"math"

	"nimbleterminal.dev/nt3d"
	math3d "nimbleterminal.dev/nt3d/math"
	"nimbleterminal.dev/nt3d/mesh"
)

// MaxTriangles is the most faces a mesh may have: as many as nt3d
// draws, which is as many as any GPU is sure to hold. Each face has vertices
// of its own, so that its flat normal is preserved.
const MaxTriangles = nt3d.MaxMeshTriangles

// maxTriangles is the limit in force. Tests lower it.
var maxTriangles = MaxTriangles

type Mesh struct {
	geometry nt3d.Geometry
	plain    []bool       // Per vertex: the file gave the face no color.
	saved    []color.RGBA // The file's colors, while all faces are painted over.

	// For OBJ and PLY: what the file was, its object count, and what its
	// decoder passed over.
	format      string
	objects     int
	warnings    []string
	pointRadius float32 // A point cloud's disc radius, in the file's units.
}

func (m *Mesh) Geometry() nt3d.Geometry { return m.geometry }
func (m *Mesh) Triangles() int          { return len(m.geometry.Indices) / 3 }
func (m *Mesh) Points() int             { return len(m.geometry.Points) }

// meshColor is given to faces whose file names none.
var meshColor = color.RGBA{R: 100, G: 180, B: 230, A: 255}

func triangleArea(v [3]math3d.Vec3) float64 {
	n := v[1].Sub(v[0]).Cross(v[2].Sub(v[0]))
	return math.Sqrt(float64(n.X)*float64(n.X)+float64(n.Y)*float64(n.Y)+float64(n.Z)*float64(n.Z)) / 2
}

func (m *Mesh) area() float64 {
	total, g := 0.0, m.geometry
	for i := 0; i+2 < len(g.Indices); i += 3 {
		total += triangleArea([3]math3d.Vec3{g.Vertices[g.Indices[i]].Position, g.Vertices[g.Indices[i+1]].Position, g.Vertices[g.Indices[i+2]].Position})
	}
	return total
}

func ParseSTL(data []byte) (*Mesh, error) {
	decoded, err := nt3dmesh.DecodeSTL(data, &nt3dmesh.Options{MaxTriangles: maxTriangles, Color: meshColor})
	if err != nil {
		if errors.Is(err, nt3dmesh.ErrTooManyTriangles) {
			return nil, fmt.Errorf("mesh exceeds %s triangles", grouped(maxTriangles))
		}
		return nil, err
	}
	m := &Mesh{}
	for _, object := range decoded.Objects {
		base := uint32(len(m.geometry.Vertices))
		m.geometry.Vertices = append(m.geometry.Vertices, object.Geometry.Vertices...)
		for _, index := range object.Geometry.Indices {
			m.geometry.Indices = append(m.geometry.Indices, base+index)
		}
		m.geometry.Bounds.Include(object.Geometry.Bounds.Min)
		m.geometry.Bounds.Include(object.Geometry.Bounds.Max)
		m.plain = append(m.plain, object.Plain...)
	}
	if m.Triangles() == 0 {
		return nil, fmt.Errorf("STL contains no triangles")
	}
	return m, nil
}

// add appends a flat-shaded face. The parser now delegates to nt3d/mesh;
// this helper remains for Gloss's geometry-focused tests and adapters.
func (m *Mesh) add(v [3]math3d.Vec3, shade color.RGBA) error {
	if m.Triangles() >= maxTriangles {
		return fmt.Errorf("mesh exceeds %s triangles", grouped(maxTriangles))
	}
	for _, p := range v {
		for _, x := range []float32{p.X, p.Y, p.Z} {
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) || math.Abs(float64(x)) > 1e15 {
				return fmt.Errorf("mesh vertex is non-finite or exceeds coordinate limits")
			}
		}
	}
	n := v[1].Sub(v[0]).Cross(v[2].Sub(v[0])).Normalize()
	for _, p := range v {
		m.geometry.Indices = append(m.geometry.Indices, uint32(len(m.geometry.Vertices)))
		m.geometry.Vertices = append(m.geometry.Vertices, nt3d.Vertex{Position: p, Normal: n, Color: shade})
		m.plain = append(m.plain, shade == meshColor)
		m.geometry.Bounds.Include(p)
	}
	return nil
}
