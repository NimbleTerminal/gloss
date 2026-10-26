package document

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"math"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"nimbleterminal.dev/nt3d"
	"nimbleterminal.dev/nt3d/mesh"
)

// OBJ and PLY are read by nt3d's decoders and flattened into the one
// geometry the viewer shows, as 3MF parts are, except textured OBJ parts,
// which keep their own geometry and material. Both formats leave their up
// axis unsaid; Y-up is what most tools write, so it is turned to the Z-up the
// named views assume. A PLY's texture is not read.

const (
	// An OBJ's material libraries and textures are read from beside the model
	// or below it: at most this many files, libraries at most this large,
	// textures at most these, in bytes and decoded pixels.
	maxSideFiles     = 32
	maxSideFileSize  = 1 << 20
	maxTextureSize   = 16 << 20
	maxTexturePixels = 16 << 20
	maxTexturesTotal = 64 << 20 // Decoded pixels of every texture together.
)

// ParseOBJ reads a Wavefront OBJ. Its material libraries are read from
// beside it when it is a file; nothing else is.
func ParseOBJ(name string, data []byte, files fs.FS, stdin bool, up nt3dmesh.Up) (*Mesh, error) {
	o := &nt3dmesh.Options{MaxTriangles: maxTriangles, Color: meshColor, Up: up}
	if !stdin {
		o.Open = sideFiles(filepath.Dir(name), files)
	}
	return parseMeshFormat("OBJ", nt3dmesh.DecodeOBJ, data, o)
}

// ParsePLY reads a Stanford PLY: faces, a range scan's surface, or a cloud of
// points. A texture it names is not read.
func ParsePLY(data []byte, up nt3dmesh.Up) (*Mesh, error) {
	return parseMeshFormat("PLY", nt3dmesh.DecodePLY, data, &nt3dmesh.Options{MaxTriangles: maxTriangles, Color: meshColor, Up: up})
}

// upAxis is the up axis asked for, else the format's usual one.
func upAxis(asked, usual string) nt3dmesh.Up {
	if asked == "" {
		asked = usual
	}
	if asked == "y" {
		return nt3dmesh.YUp
	}
	return nt3dmesh.ZUp
}

func parseMeshFormat(format string, decode func([]byte, *nt3dmesh.Options) (*nt3dmesh.Model, error), data []byte, o *nt3dmesh.Options) (*Mesh, error) {
	decoded, err := decode(data, o)
	if err != nil {
		if errors.Is(err, nt3dmesh.ErrTooManyTriangles) {
			return nil, fmt.Errorf("%s exceeds %s triangles or %s points", format, grouped(maxTriangles), grouped(nt3d.MaxPoints))
		}
		return nil, errors.New(strings.TrimPrefix(err.Error(), "nt3dmesh: "))
	}
	m := meshFromObjects(decoded.Objects)
	m.format, m.objects, m.up = format, len(decoded.Objects), map[nt3dmesh.Up]string{nt3dmesh.YUp: "Y", nt3dmesh.ZUp: "Z"}[o.Up]
	m.warnings = decoded.Warnings
	if m.Triangles() == 0 && len(m.geometry.Points) == 0 && len(m.geometry.Lines) == 0 {
		return nil, fmt.Errorf("%s contains nothing to draw", format)
	}
	m.pointRadius = cloudRadius(m.geometry)
	return m, nil
}

// cloudRadius sizes a point cloud's discs so that neighbors about touch and
// the cloud reads as a surface: points are taken to spread over the two
// largest sides of their bounds.
func cloudRadius(g nt3d.Geometry) float32 {
	if len(g.Points) == 0 || !g.Bounds.IsValid() {
		return 0
	}
	d := g.Bounds.Max.Sub(g.Bounds.Min)
	sides := []float64{float64(d.X), float64(d.Y), float64(d.Z)}
	slices.Sort(sides)
	area := max(sides[1]*sides[2], 1e-12)
	return float32(.6 * math.Sqrt(area/float64(len(g.Points))))
}

// sideFiles opens an OBJ's material libraries and PNG or JPEG textures:
// relative names inside the model's folder, bounded in number, size, and
// decoded pixels before nt3d decodes them.
func sideFiles(dir string, files fs.FS) func(string) ([]byte, error) {
	read, pixels := 0, 0
	return func(name string) ([]byte, error) {
		clean := path.Clean(strings.ReplaceAll(name, `\`, "/"))
		ext := strings.ToLower(path.Ext(clean))
		texture := ext == ".png" || ext == ".jpg" || ext == ".jpeg"
		switch {
		case ext != ".mtl" && !texture:
			return nil, errors.New("only PNG and JPEG textures are shown")
		case path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") || filepath.VolumeName(clean) != "":
			return nil, errors.New("only files beside the model are read")
		case read >= maxSideFiles:
			return nil, fmt.Errorf("side files are limited to %d", maxSideFiles)
		}
		read++
		full := filepath.Join(dir, filepath.FromSlash(clean))
		if files != nil {
			full = filepath.ToSlash(full)
		}
		data, err := readFileFrom(files, full)
		if err != nil {
			return nil, err
		}
		if !texture {
			if len(data) > maxSideFileSize {
				return nil, fmt.Errorf("larger than %d KiB", maxSideFileSize>>10)
			}
			return data, nil
		}
		if len(data) > maxTextureSize {
			return nil, fmt.Errorf("larger than %d MiB", maxTextureSize>>20)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		n := cfg.Width * cfg.Height
		switch {
		case n > maxTexturePixels:
			return nil, fmt.Errorf("%d × %d is more than %d megapixels", cfg.Width, cfg.Height, maxTexturePixels>>20)
		case pixels+n > maxTexturesTotal:
			return nil, fmt.Errorf("textures together exceed %d megapixels", maxTexturesTotal>>20)
		}
		pixels += n
		return data, nil
	}
}

// meshFormatFields describes an OBJ or PLY model.
func meshFormatFields(m *Mesh, number func(float64) string, loc Locale) []Field {
	g := m.geometry
	extent := g.Bounds.Max.Sub(g.Bounds.Min)
	count := func(n int) string {
		if n == 0 {
			return ""
		}
		return loc.Count(n)
	}
	fields := []Field{{"Format", m.format}, {"Objects", count(m.objects)}, {"Triangles", count(m.Triangles())}, {"Textured parts", count(len(m.textured))}, {"Up axis", m.up},
		{"Points", count(len(g.Points))}, {"Lines", count(len(g.Lines) / 2)},
		{"Extent", number(float64(extent.X)) + " × " + number(float64(extent.Y)) + " × " + number(float64(extent.Z))}}
	if m.Triangles() > 0 {
		fields = append(fields, Field{"Surface area", number(m.area())})
	}
	// The file's own words, cleaned and few.
	for i, w := range m.warnings {
		if i == 3 {
			fields = append(fields, Field{"Note", fmt.Sprintf("and %d more", len(m.warnings)-3)})
			break
		}
		fields = append(fields, Field{"Note", strings.Map(printable, w)})
	}
	return Section("Mesh", fields...)
}

// printable drops control characters from text a file supplied.
func printable(r rune) rune {
	if unicode.IsPrint(r) {
		return r
	}
	return -1
}
