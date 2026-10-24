package document

import (
	"image/color"
	"testing"
)

var (
	ink   = color.RGBA{0, 0, 0, 255}
	paper = color.RGBA{255, 255, 255, 255}
)

func TestChartImageSizeAndBackground(t *testing.T) {
	img := ChartImage("ab\ncd", 4, 2, 10, 20, ink, paper)
	if b := img.Bounds(); b.Dx() != 40 || b.Dy() != 40 {
		t.Fatalf("%v", b)
	}
	if img.RGBAAt(35, 5) != paper {
		t.Fatalf("background %v", img.RGBAAt(35, 5))
	}
	if img.RGBAAt(5, 10) == paper {
		t.Fatal("no glyph drawn for 'a'")
	}
}

func TestChartImageBrailleDots(t *testing.T) {
	// U+2801 is dot 1: top-left. U+2880 is dot 8: bottom-right.
	img := ChartImage("⠁⢀", 2, 1, 10, 20, ink, paper)
	if img.RGBAAt(2, 2) != ink || img.RGBAAt(7, 17) != paper {
		t.Fatalf("dot 1: %v %v", img.RGBAAt(2, 2), img.RGBAAt(7, 17))
	}
	if img.RGBAAt(17, 17) != ink || img.RGBAAt(12, 2) != paper {
		t.Fatalf("dot 8: %v %v", img.RGBAAt(17, 17), img.RGBAAt(12, 2))
	}
}

func TestChartImageBoxAndBlocks(t *testing.T) {
	img := ChartImage("─│█▄", 4, 1, 10, 20, ink, paper)
	if img.RGBAAt(5, 10) != ink || img.RGBAAt(5, 2) != paper { // ─ across the middle
		t.Fatal("horizontal line")
	}
	if img.RGBAAt(15, 2) != ink || img.RGBAAt(12, 10) != paper { // │ down the middle
		t.Fatal("vertical line")
	}
	if img.RGBAAt(25, 2) != ink || img.RGBAAt(25, 18) != ink { // █ fills
		t.Fatal("full block")
	}
	if img.RGBAAt(35, 2) != paper || img.RGBAAt(35, 18) != ink { // ▄ lower half
		t.Fatal("lower half block")
	}
}

func TestChartImageColoursFromSGR(t *testing.T) {
	red := color.RGBA{200, 10, 10, 255}
	img := ChartImage("\x1b[38;2;200;10;10m█\x1b[m█\x1b[48;2;200;10;10m \x1b[0m", 3, 1, 10, 20, ink, paper)
	if img.RGBAAt(5, 10) != red || img.RGBAAt(15, 10) != ink || img.RGBAAt(25, 10) != red {
		t.Fatalf("%v %v %v", img.RGBAAt(5, 10), img.RGBAAt(15, 10), img.RGBAAt(25, 10))
	}
}

func TestChartImageDrawsUnknownRunesAsBoxes(t *testing.T) {
	img := ChartImage("\U0001F600x", 3, 1, 10, 20, ink, paper)
	if img.RGBAAt(1, 10) != ink || img.RGBAAt(10, 10) != paper {
		t.Fatalf("placeholder box: %v %v", img.RGBAAt(1, 10), img.RGBAAt(10, 10))
	}
}

func TestChartImageKeepsCellsInStepWithTheTerminal(t *testing.T) {
	// A wide rune takes two cells: the block after it is in the third.
	img := ChartImage("\U0001F600█", 3, 1, 10, 20, ink, paper)
	if img.RGBAAt(25, 10) != ink {
		t.Fatal("the block after a wide rune is not in the third cell")
	}
	// A combining mark takes none: the block is in the second cell.
	img = ChartImage("e\u0301█", 3, 1, 10, 20, ink, paper)
	if img.RGBAAt(15, 10) != ink || img.RGBAAt(25, 10) != paper {
		t.Fatalf("combining mark advanced the cells: %v %v", img.RGBAAt(15, 10), img.RGBAAt(25, 10))
	}
}

func TestChartGrid(t *testing.T) {
	for _, tc := range []struct{ edge, cols, rows, cellW int }{{1536, 120, 35, 13}, {600, 50, 14, 12}, {64, 40, 11, 12}, {4096, 120, 35, 35}, {512, 42, 12, 13}} {
		cols, rows, cellW, cellH := ChartGrid(tc.edge)
		if cols != tc.cols || rows != tc.rows || cellW != tc.cellW || cellH != 2*cellW || cols*cellW < min(tc.edge, 40*ChartCellWidth) {
			t.Errorf("%d: %d×%d cells of %d×%d", tc.edge, cols, rows, cellW, cellH)
		}
	}
}
