package document

import (
	"image"
	"image/color"
	"image/draw"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	monoOnce sync.Once
	monoFont *opentype.Font
	monoErr  error
)

// Pixel size of one terminal cell in a chart export.
const ChartCellWidth, ChartCellHeight = 12, 24

// ChartGrid chooses the cell grid for an export edge: 40 to 120 columns
// and rows at a 24:7 ratio, so the picture is shaped like a terminal, with
// cells at least ChartCellWidth wide and big enough that the picture
// reaches the edge before export scales it.
func ChartGrid(maxEdge int) (cols, rows, cellW, cellH int) {
	cols = max(40, min(120, maxEdge/ChartCellWidth))
	cellW = max(ChartCellWidth, (maxEdge+cols-1)/cols)
	return cols, max(8, cols*7/24), cellW, 2 * cellW
}

// cellRun is a run of cells drawn with the same colours.
type cellRun struct {
	runes  []rune
	fg, bg color.RGBA
	fgSet  bool
	bgSet  bool
}

// chartFrame splits a frame into rows of coloured runs, reading the SGR
// sequences ntcharts and lipgloss write and dropping any other escape.
func chartFrame(frame string) [][]cellRun {
	var rows [][]cellRun
	for _, line := range strings.Split(frame, "\n") {
		var runs []cellRun
		run := cellRun{}
		flush := func() {
			if len(run.runes) > 0 {
				runs = append(runs, run)
			}
			run = cellRun{fg: run.fg, bg: run.bg, fgSet: run.fgSet, bgSet: run.bgSet}
		}
		for i := 0; i < len(line); {
			if line[i] == 0x1b {
				end := i + 1
				switch {
				case end < len(line) && line[end] == '[':
					end++
					for end < len(line) && (line[end] < 0x40 || line[end] > 0x7e) {
						end++
					}
					if end < len(line) && line[end] == 'm' {
						flush()
						applySGR(line[i+2:end], &run)
					}
					end++
				case end < len(line) && line[end] == ']': // OSC, to BEL or ST.
					for end < len(line) && line[end] != 0x07 && !(line[end] == 0x1b && end+1 < len(line) && line[end+1] == '\\') {
						end++
					}
					if end < len(line) && line[end] == 0x1b {
						end++
					}
					end++
				default:
					end++
				}
				i = min(end, len(line))
				continue
			}
			r, size := rune(line[i]), 1
			if r >= utf8.RuneSelf {
				r, size = utf8.DecodeRuneInString(line[i:])
			}
			if r >= 0x20 {
				run.runes = append(run.runes, r)
			}
			i += size
		}
		flush()
		rows = append(rows, runs)
	}
	return rows
}

var ansiBasic = [16]color.RGBA{{0, 0, 0, 255}, {205, 49, 49, 255}, {13, 188, 121, 255}, {229, 229, 16, 255}, {36, 114, 200, 255}, {188, 63, 188, 255}, {17, 168, 205, 255}, {229, 229, 229, 255}, {102, 102, 102, 255}, {241, 76, 76, 255}, {35, 209, 139, 255}, {245, 245, 67, 255}, {59, 142, 234, 255}, {214, 112, 214, 255}, {41, 184, 219, 255}, {255, 255, 255, 255}}

// ansi256 is the xterm 256-colour palette entry n.
func ansi256(n int) color.RGBA {
	switch {
	case n < 16:
		return ansiBasic[n]
	case n < 232:
		n -= 16
		steps := [6]uint8{0, 95, 135, 175, 215, 255}
		return color.RGBA{steps[n/36], steps[n/6%6], steps[n%6], 255}
	default:
		v := uint8(8 + (n-232)*10)
		return color.RGBA{v, v, v, 255}
	}
}

// applySGR reads one m-sequence's parameters into run.
func applySGR(params string, run *cellRun) {
	if params == "" {
		*run = cellRun{}
		return
	}
	fields := strings.Split(params, ";")
	for i := 0; i < len(fields); i++ {
		n, _ := strconv.Atoi(fields[i])
		switch {
		case n == 0:
			*run = cellRun{}
		case n == 39:
			run.fgSet = false
		case n == 49:
			run.bgSet = false
		case n >= 30 && n <= 37:
			run.fg, run.fgSet = ansiBasic[n-30], true
		case n >= 90 && n <= 97:
			run.fg, run.fgSet = ansiBasic[n-90+8], true
		case n >= 40 && n <= 47:
			run.bg, run.bgSet = ansiBasic[n-40], true
		case n >= 100 && n <= 107:
			run.bg, run.bgSet = ansiBasic[n-100+8], true
		case (n == 38 || n == 48) && i+1 < len(fields):
			var c color.RGBA
			switch fields[i+1] {
			case "2":
				if i+4 >= len(fields) {
					return
				}
				r, _ := strconv.Atoi(fields[i+2])
				g, _ := strconv.Atoi(fields[i+3])
				b, _ := strconv.Atoi(fields[i+4])
				c = color.RGBA{uint8(r), uint8(g), uint8(b), 255}
				i += 4
			case "5":
				if i+2 >= len(fields) {
					return
				}
				idx, _ := strconv.Atoi(fields[i+2])
				c = ansi256(max(0, min(255, idx)))
				i += 2
			default:
				return
			}
			if n == 38 {
				run.fg, run.fgSet = c, true
			} else {
				run.bg, run.bgSet = c, true
			}
		}
	}
}

// ChartImage draws a terminal chart frame as an image cols by rows cells of
// cellW by cellH pixels, fg and bg being the colours of cells the frame
// does not colour. Braille, box-drawing and block runes are drawn as
// shapes, so they need no font; the rest is set in Go Mono.
func ChartImage(frame string, cols, rows, cellW, cellH int, fg, bg color.RGBA) *image.RGBA {
	cols, rows, cellW, cellH = max(1, cols), max(1, rows), max(2, cellW), max(4, cellH)
	img := image.NewRGBA(image.Rect(0, 0, cols*cellW, rows*cellH))
	draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	face := monoFace(cellH)
	for y, runs := range chartFrame(frame) {
		if y >= rows {
			break
		}
		x := 0
		for _, run := range runs {
			ink, paper := fg, bg
			if run.fgSet {
				ink = run.fg
			}
			if run.bgSet {
				paper = run.bg
			}
			for _, r := range run.runes {
				if x >= cols {
					break
				}
				cell := image.Rect(x*cellW, y*cellH, (x+1)*cellW, (y+1)*cellH)
				if run.bgSet {
					draw.Draw(img, cell, image.NewUniform(paper), image.Point{}, draw.Src)
				}
				drawCell(img, cell, r, ink, face)
				x++
			}
		}
	}
	return img
}

func monoFace(cellH int) font.Face {
	monoOnce.Do(func() { monoFont, monoErr = opentype.Parse(gomono.TTF) })
	if monoErr != nil {
		return nil
	}
	face, err := opentype.NewFace(monoFont, &opentype.FaceOptions{Size: float64(cellH) * 0.72, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil
	}
	return face
}

// drawCell draws one rune into its cell.
func drawCell(img *image.RGBA, cell image.Rectangle, r rune, ink color.RGBA, face font.Face) {
	paint := image.NewUniform(ink)
	fill := func(rect image.Rectangle) { draw.Draw(img, rect.Intersect(cell), paint, image.Point{}, draw.Src) }
	w, h := cell.Dx(), cell.Dy()
	switch {
	case r == ' ':
		return
	case r >= 0x2800 && r <= 0x28FF:
		bits := int(r - 0x2800)
		dw, dh := max(1, w/2), max(1, h/4)
		// Dot bits: 1,2,3 left column rows 0-2; 4,5,6 right column rows 0-2; 7 left row 3; 8 right row 3.
		for i, at := range [8][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}} {
			if bits&(1<<i) != 0 {
				x0, y0 := cell.Min.X+at[0]*dw+dw/4, cell.Min.Y+at[1]*dh+dh/4
				fill(image.Rect(x0, y0, x0+max(1, dw/2), y0+max(1, dh/2)))
			}
		}
	case r >= 0x2581 && r <= 0x2588: // Lower blocks, one to eight eighths.
		eighths := int(r - 0x2580)
		fill(image.Rect(cell.Min.X, cell.Max.Y-h*eighths/8, cell.Max.X, cell.Max.Y))
	case r >= 0x2589 && r <= 0x258F: // Left blocks, seven to one eighths.
		eighths := 8 - int(r-0x2588)
		fill(image.Rect(cell.Min.X, cell.Min.Y, cell.Min.X+w*eighths/8, cell.Max.Y))
	case r == 0x2580: // Upper half.
		fill(image.Rect(cell.Min.X, cell.Min.Y, cell.Max.X, cell.Min.Y+h/2))
	case r >= 0x2500 && r <= 0x257F:
		drawBox(fill, cell, r)
	default:
		if face == nil || !hasGlyph(face, r) {
			t := max(1, w/8)
			fill(image.Rect(cell.Min.X+1, cell.Min.Y+2, cell.Min.X+1+t, cell.Max.Y-2))
			fill(image.Rect(cell.Max.X-1-t, cell.Min.Y+2, cell.Max.X-1, cell.Max.Y-2))
			fill(image.Rect(cell.Min.X+1, cell.Min.Y+2, cell.Max.X-1, cell.Min.Y+2+t))
			fill(image.Rect(cell.Min.X+1, cell.Max.Y-2-t, cell.Max.X-1, cell.Max.Y-2))
			return
		}
		metrics := face.Metrics()
		baseline := cell.Min.Y + (h+metrics.Ascent.Ceil()-metrics.Descent.Ceil())/2
		advance, _ := face.GlyphAdvance(r)
		x := cell.Min.X + (w-advance.Ceil())/2
		d := font.Drawer{Dst: img, Src: paint, Face: face, Dot: fixed.P(x, baseline)}
		d.DrawString(string(r))
	}
}

func hasGlyph(face font.Face, r rune) bool {
	_, _, _, _, ok := face.Glyph(fixed.Point26_6{}, r)
	return ok
}

// heavyBox reports whether a box-drawing rune is a heavy or double line.
func heavyBox(r rune) bool {
	switch {
	case r == 0x2501, r == 0x2503, r == 0x2505, r == 0x2507, r == 0x2509, r == 0x250B, r == 0x254D, r == 0x254F:
		return true
	case r >= 0x2550 && r <= 0x256C:
		return true
	case r >= 0x250C && r <= 0x254B:
		// Corners, tees and crosses come in groups of four: light first.
		return (r-0x250C)%4 != 0
	case r >= 0x2578 && r <= 0x257B:
		return true
	}
	return false
}

// drawBox draws a box-drawing rune as lines through the cell's centre.
// Heavy and double lines are drawn twice as thick.
func drawBox(fill func(image.Rectangle), cell image.Rectangle, r rune) {
	w, h := cell.Dx(), cell.Dy()
	cx, cy := cell.Min.X+w/2, cell.Min.Y+h/2
	t := max(1, w/8)
	up, down, left, right := false, false, false, false
	switch {
	case r == 0x2500 || r == 0x2501 || (r >= 0x2504 && r <= 0x2505) || (r >= 0x2508 && r <= 0x2509) || r == 0x254C || r == 0x254D || r == 0x2550:
		left, right = true, true
	case r == 0x2502 || r == 0x2503 || (r >= 0x2506 && r <= 0x2507) || (r >= 0x250A && r <= 0x250B) || r == 0x254E || r == 0x254F || r == 0x2551:
		up, down = true, true
	case (r >= 0x250C && r <= 0x250F) || (r >= 0x2552 && r <= 0x2554) || r == 0x256D:
		down, right = true, true
	case (r >= 0x2510 && r <= 0x2513) || (r >= 0x2555 && r <= 0x2557) || r == 0x256E:
		down, left = true, true
	case (r >= 0x2514 && r <= 0x2517) || (r >= 0x2558 && r <= 0x255A) || r == 0x2570:
		up, right = true, true
	case (r >= 0x2518 && r <= 0x251B) || (r >= 0x255B && r <= 0x255D) || r == 0x256F:
		up, left = true, true
	case (r >= 0x251C && r <= 0x2523) || (r >= 0x255E && r <= 0x2560):
		up, down, right = true, true, true
	case (r >= 0x2524 && r <= 0x252B) || (r >= 0x2561 && r <= 0x2563):
		up, down, left = true, true, true
	case (r >= 0x252C && r <= 0x2533) || (r >= 0x2564 && r <= 0x2566):
		down, left, right = true, true, true
	case (r >= 0x2534 && r <= 0x253B) || (r >= 0x2567 && r <= 0x2569):
		up, left, right = true, true, true
	case (r >= 0x253C && r <= 0x254B) || (r >= 0x256A && r <= 0x256C):
		up, down, left, right = true, true, true, true
	case r == 0x2574 || r == 0x2578:
		left = true
	case r == 0x2575 || r == 0x2579:
		up = true
	case r == 0x2576 || r == 0x257A:
		right = true
	case r == 0x2577 || r == 0x257B:
		down = true
	default:
		return
	}
	if heavyBox(r) {
		t *= 2
	}
	if left {
		fill(image.Rect(cell.Min.X, cy-t/2, cx+t/2+1, cy-t/2+t))
	}
	if right {
		fill(image.Rect(cx-t/2, cy-t/2, cell.Max.X, cy-t/2+t))
	}
	if up {
		fill(image.Rect(cx-t/2, cell.Min.Y, cx-t/2+t, cy+t/2+1))
	}
	if down {
		fill(image.Rect(cx-t/2, cy-t/2, cx-t/2+t, cell.Max.Y))
	}
}
