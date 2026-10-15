package document

import (
	"cmp"
	"errors"
	"fmt"
	"html"
	"image"
	"regexp"
	"strconv"
	"strings"

	nt3dmath "nimbleterminal.dev/nt3d/math"
	"nimbleterminal.dev/nt3d/mesh"
)

// Model3MF adapts nt3d's decoded 3MF model to Gloss's document and part
// selection interfaces. The encoded bytes are retained so a part selection
// can be decoded without first assembling the full model.
type Model3MF struct {
	Mesh      *Mesh // Nil when the model has more triangles than can be shown.
	Thumbnail image.Image
	Triangles int
	Objects   int
	Parts     []Part
	Unit      string
	metadata  map[string]string
	bounds    nt3dmath.AABB
	area      float64
	data      []byte
	limit     error // Why the model was not measured, when only its thumbnail is shown.
}

// errUncountable identifies nt3d's bounded summary pass stopping before a
// complete triangle count is available.
var errUncountable = errors.New("3MF has too many triangles to count")

// Part is one thing a 3MF build places: an object, with whatever it holds.
type Part = mesh.Part

func (m *Model3MF) triangles() int {
	if m == nil {
		return 0
	}
	return m.Triangles
}

func (m *Model3MF) tooLarge() string {
	if m.limit != nil {
		reason := strings.TrimPrefix(strings.TrimPrefix(m.limit.Error(), "nt3dmesh: "), "3MF ")
		return "the model " + reason
	}
	return fmt.Sprintf("the model has %s triangles, and the limit for a mesh is %s", grouped(m.Triangles), grouped(maxTriangles))
}

func Parse3MF(data []byte) (*Model3MF, error) {
	decoded, err := mesh.Decode3MF(data, &mesh.Options{MaxTriangles: maxTriangles, MaxBytes: MaxFileBytes, Color: meshColor})
	tooMany := errors.Is(err, mesh.ErrTooManyTriangles)
	tooLarge := errors.Is(err, mesh.ErrTooLarge)
	uncountable := tooMany && strings.Contains(err.Error(), "3MF has more than")
	if err != nil && !tooMany && !tooLarge {
		return nil, err
	}
	if (tooLarge || uncountable) && decoded.Thumbnail == nil {
		return nil, err
	}

	out := &Model3MF{
		Thumbnail: decoded.Thumbnail,
		Parts:     decoded.Parts,
		Unit:      decoded.Unit,
		metadata:  decoded.Metadata,
		data:      append([]byte(nil), data...),
	}
	if decoded.Summary != nil {
		out.Triangles = decoded.Summary.Triangles
		out.Objects = decoded.Summary.Meshes
		out.bounds = decoded.Summary.Bounds
		out.area = decoded.Summary.Area
	}
	if err == nil {
		out.Mesh = meshFromObjects(decoded.Objects)
		out.Triangles = out.Mesh.Triangles()
	}
	if uncountable {
		out.limit = errUncountable
	} else if tooLarge {
		out.limit = err
	}
	return out, nil
}

func (m *Model3MF) Assemble(shown []bool) (*Mesh, error) {
	decoded, err := mesh.Decode3MF(m.data, &mesh.Options{MaxTriangles: maxTriangles, MaxBytes: MaxFileBytes, Color: meshColor, Parts: shown})
	if err != nil {
		return nil, err
	}
	assembled := meshFromObjects(decoded.Objects)
	if assembled == nil || assembled.Triangles() == 0 {
		return nil, fmt.Errorf("no parts shown")
	}
	return assembled, nil
}

// meshFromObjects flattens decoded object geometries into the single geometry
// Gloss's mesh viewer expects, preserving face colors and plain-color flags.
func meshFromObjects(objects []mesh.Object) *Mesh {
	out := &Mesh{}
	for _, object := range objects {
		base := uint32(len(out.geometry.Vertices))
		out.geometry.Vertices = append(out.geometry.Vertices, object.Geometry.Vertices...)
		for _, index := range object.Geometry.Indices {
			out.geometry.Indices = append(out.geometry.Indices, base+index)
		}
		if object.Geometry.Bounds.IsValid() {
			out.geometry.Bounds.Include(object.Geometry.Bounds.Min)
			out.geometry.Bounds.Include(object.Geometry.Bounds.Max)
		}
		out.plain = append(out.plain, object.Plain...)
	}
	return out
}

var markup = regexp.MustCompile(`<[^>]*>`)

func (m *Model3MF) fields(shown string) []Field {
	number := func(v float64) string { return strconv.FormatFloat(v, 'g', 6, 64) }
	unit := map[string]string{"micron": "µm", "millimeter": "mm", "centimeter": "cm", "inch": "in", "foot": "ft", "meter": "m"}[m.Unit]
	meta := func(name string) string {
		value := m.metadata[name]
		// Sites that host models put HTML, escaped twice over, in descriptions.
		value = markup.ReplaceAllString(html.UnescapeString(html.UnescapeString(value)), " ")
		value = strings.Join(strings.Fields(strings.ReplaceAll(value, " ", " ")), " ")
		if value == "[]" {
			return ""
		}
		return strings.NewReplacer(" .", ".", " ,", ",").Replace(value)
	}
	extent := m.bounds.Max.Sub(m.bounds.Min)
	thumbnail := ""
	if m.Thumbnail != nil {
		thumbnail = fmt.Sprintf("%d × %d", m.Thumbnail.Bounds().Dx(), m.Thumbnail.Bounds().Dy())
	}
	area := number(m.area)
	if unit != "" {
		area += " " + unit + "²"
	}
	objects, triangles, size := grouped(m.Objects), grouped(m.Triangles), strings.TrimSpace(number(float64(extent.X))+" × "+number(float64(extent.Y))+" × "+number(float64(extent.Z))+" "+unit)
	if m.limit != nil {
		objects, triangles, size, area = "", "", "", ""
	}
	return Section("Model", Field{"Format", "3MF"}, Field{"Title", meta("Title")}, Field{"Designer", meta("Designer")}, Field{"Description", meta("Description")},
		Field{"Application", meta("Application")}, Field{"Created", meta("CreationDate")}, Field{"Changed", meta("ModificationDate")},
		Field{"License", cmp.Or(meta("LicenseTerms"), meta("License"))}, Field{"Copyright", meta("Copyright")},
		Field{"Objects", objects}, Field{"Parts", partNames(m.Parts, nil)}, Field{"Triangles", triangles},
		Field{"Extent", size},
		Field{"Surface area", area}, Field{"Thumbnail", thumbnail}, Field{"Shown", shown})
}

// partNames lists the parts marked in shown, all of them when it is nil,
// by name, up to a sensible number.
func partNames(parts []Part, shown []bool) string {
	var names []string
	for i, part := range parts {
		if shown == nil || (i < len(shown) && shown[i]) {
			names = append(names, part.Name)
		}
	}
	if len(parts) < 2 && shown == nil {
		return ""
	}
	if len(names) > 12 {
		return strings.Join(names[:12], ", ") + fmt.Sprintf(", and %d more", len(names)-12)
	}
	return strings.Join(names, ", ")
}

// partsShown says which parts a filter kept, for the info box.
func partsShown(parts []Part, shown []bool) string {
	n := 0
	for _, s := range shown {
		if s {
			n++
		}
	}
	return fmt.Sprintf("%s, %d of %d parts", partNames(parts, shown), n, len(parts))
}
