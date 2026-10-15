// Run from experiments/typeset. This harness never changes the shipped viewer.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"bytes"
	"github.com/ledongthuc/pdf"
	"nimbleterminal.dev/ntpdf"
)

type measurement struct {
	Backend           string  `json:"backend"`
	Fixture           string  `json:"fixture"`
	FirstMS           float64 `json:"first_viewport_ms"`
	PrepareMS         float64 `json:"prepare_ms,omitempty"`
	FirstLayoutMS     float64 `json:"first_layout_ms,omitempty"`
	FirstRasterMS     float64 `json:"first_raster_ms,omitempty"`
	RemainingLayoutMS float64 `json:"remaining_layout_ms,omitempty"`
	ResizeMS          float64 `json:"resize_first_viewport_ms"`
	RepaintMS         float64 `json:"repaint_viewport_ms"`
	FirstAlloc        uint64  `json:"first_allocated_bytes"`
	FirstHeap         uint64  `json:"first_live_heap_bytes"`
	ResizeAlloc       uint64  `json:"resize_allocated_bytes"`
	PeakRSS           uint64  `json:"process_peak_rss_bytes"`
	BlocksAtFirst     int     `json:"blocks_at_first,omitempty"`
	BlocksTotal       int     `json:"blocks_total,omitempty"`
	PDFPages          int     `json:"pdf_pages,omitempty"`
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
func save(path string, img image.Image) error {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	err := png.Encode(f, img)
	ce := f.Close()
	if err != nil {
		return err
	}
	return ce
}
func crop(img image.Image, width, height int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(out, out.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Over)
	return out
}
func main() {
	worker := flag.String("worker", "", "internal: direct or pdf")
	fixture := flag.String("fixture", "../../examples/typeset.md", "local Markdown fixture")
	fonts := flag.String("fonts", "../../internal/document/fonts", "pinned Go TTF directory")
	output := flag.String("out", "", "new results directory (default: temporary directory)")
	runs := flag.Int("runs", 5, "fresh-process samples per backend and fixture (1..20)")
	flag.Parse()
	var err error
	if *worker != "" {
		var m measurement
		m, err = measure(*worker, *fixture, *fonts, *output)
		if err == nil {
			err = json.NewEncoder(os.Stdout).Encode(m)
		}
	} else {
		err = compare(*fixture, *fonts, *output, *runs)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func measure(backend, path, fonts, out string) (m measurement, err error) {
	m.Backend, m.Fixture = backend, filepath.Base(path)
	runtime.GC()
	var before, first, resizeStart, resizeEnd runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	var initial, narrow, full image.Image
	switch backend {
	case "direct":
		e, er := newEngine(fonts)
		if er != nil {
			return m, er
		}
		p, er := prepare(path)
		if er != nil {
			return m, er
		}
		m.PrepareMS = ms(time.Since(start))
		l := e.layout(p, 800)
		t := time.Now()
		l.ensure(900)
		m.FirstLayoutMS = ms(time.Since(t))
		t = time.Now()
		initial = l.viewport(0, 900)
		m.FirstRasterMS = ms(time.Since(t))
		m.FirstMS = ms(time.Since(start))
		runtime.ReadMemStats(&first)
		m.BlocksAtFirst, m.BlocksTotal = l.next, len(p.blocks)
		t = time.Now()
		l.ensure(int(^uint(0) >> 1))
		m.RemainingLayoutMS = ms(time.Since(t))
		runtime.ReadMemStats(&resizeStart)
		t = time.Now()
		small := e.layout(p, 600)
		narrow = small.viewport(0, 900)
		m.ResizeMS = ms(time.Since(t))
		runtime.ReadMemStats(&resizeEnd)
		t = time.Now()
		_ = small.viewport(0, 900)
		m.RepaintMS = ms(time.Since(t))
		// Memory statistics exclude artifact rendering/encoding. A long overview is capped.
		m.PeakRSS = peakRSS()
		if out != "" {
			full = l.viewport(0, min(l.y+50, 8192))
		}
	case "pdf":
		data, er := legacyPDF(path, fonts, 384)
		if er != nil {
			return m, er
		}
		reader, er := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
		if er != nil {
			return m, er
		}
		m.PDFPages = reader.NumPage()
		renderer, er := ntpdf.DefaultRendererFactoryWithLimits(ntpdf.Limits{MaxRenderPixels: 32 << 20})(path, data)
		if er != nil {
			return m, er
		}
		page, er := renderer.RenderPage(1, 150)
		if er != nil {
			_ = renderer.Close()
			return m, er
		}
		initial = crop(page, 800, 900)
		full = page
		m.FirstMS = ms(time.Since(start))
		runtime.ReadMemStats(&first)
		runtime.ReadMemStats(&resizeStart)
		t := time.Now()
		_ = renderer.Close()
		data, er = legacyPDF(path, fonts, 288)
		if er != nil {
			return m, er
		}
		renderer, er = ntpdf.DefaultRendererFactoryWithLimits(ntpdf.Limits{MaxRenderPixels: 32 << 20})(path, data)
		if er != nil {
			return m, er
		}
		defer renderer.Close()
		page, er = renderer.RenderPage(1, 150)
		if er != nil {
			return m, er
		}
		narrow = crop(page, 600, 900)
		m.ResizeMS = ms(time.Since(t))
		runtime.ReadMemStats(&resizeEnd)
		t = time.Now()
		_ = crop(page, 600, 900)
		m.RepaintMS = ms(time.Since(t))
		m.PeakRSS = peakRSS()

	default:
		return m, fmt.Errorf("unknown backend %q", backend)
	}
	m.FirstAlloc = first.TotalAlloc - before.TotalAlloc
	m.FirstHeap = first.HeapAlloc
	m.ResizeAlloc = resizeEnd.TotalAlloc - resizeStart.TotalAlloc
	if out != "" {
		for suffix, img := range map[string]image.Image{"800": initial, "600": narrow, "overview": full} {
			if err = save(filepath.Join(out, backend+"-"+suffix+".png"), img); err != nil {
				return m, err
			}
		}
	}
	return m, nil
}
func compare(fixture, fonts, out string, runs int) error {
	if runs < 1 || runs > 20 {
		return fmt.Errorf("runs must be 1..20")
	}
	if out == "" {
		var e error
		out, e = os.MkdirTemp("", "gloss-typeset-comparison-")
		if e != nil {
			return e
		}
	} else {
		if err := os.Mkdir(out, 0700); err != nil {
			return err
		}
	}
	// Identical original fixture for both paths, plus a longer original text-only fixture.
	data := "# A longer reading session\n\n"
	for i := 0; i < 100; i++ {
		data += fmt.Sprintf("## Section %d\n\n", i+1) + strings.Repeat("A reader should reach the first screen before the final paragraph is laid out. **Mixed emphasis**, *café and résumé*, and `local code` keep the line-breaking workload realistic. ", 3) + "\n\n"
	}
	longPath := filepath.Join(out, "long.md")
	if err := os.WriteFile(longPath, []byte(data), 0600); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	var all []measurement
	for _, path := range []string{fixture, longPath} {
		dir := filepath.Join(out, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		if err = os.Mkdir(dir, 0700); err != nil {
			return err
		}
		for i := 0; i < runs; i++ {
			backends := []string{"direct", "pdf"}
			if i%2 == 1 {
				backends = []string{"pdf", "direct"}
			}
			for _, backend := range backends {
				args := []string{"-worker", backend, "-fixture", path, "-fonts", fonts}
				if i == 0 {
					args = append(args, "-out", dir)
				}
				cmd := exec.Command(executable, args...)
				cmd.Stderr = os.Stderr
				b, e := cmd.Output()
				if e != nil {
					return e
				}
				var m measurement
				if e = json.Unmarshal(b, &m); e != nil {
					return e
				}
				all = append(all, m)
				fmt.Fprintf(os.Stderr, "%s %s %d/%d: first %.1f ms, resize %.1f ms\n", m.Fixture, backend, i+1, runs, m.FirstMS, m.ResizeMS)
			}
		}
	}
	b, _ := json.MarshalIndent(all, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "measurements.json"), b, 0600); err != nil {
		return err
	}
	var report strings.Builder
	fmt.Fprintf(&report, "# Typesetting comparison\n\n%s; %s/%s; %d fresh-process samples per row. Medians.\n\n", runtime.Version(), runtime.GOOS, runtime.GOARCH, runs)
	report.WriteString("| Fixture / backend | First viewport ms | Resize ms | Repaint ms | First allocated MiB | Live Go heap MiB | Peak process RSS MiB |\n|---|---:|---:|---:|---:|---:|---:|\n")
	for _, name := range []string{filepath.Base(fixture), "long.md"} {
		for _, backend := range []string{"direct", "pdf"} {
			var first, resize, repaint, alloc, heap, rss []float64
			for _, m := range all {
				if m.Fixture == name && m.Backend == backend {
					first = append(first, m.FirstMS)
					resize = append(resize, m.ResizeMS)
					repaint = append(repaint, m.RepaintMS)
					alloc = append(alloc, float64(m.FirstAlloc)/(1<<20))
					heap = append(heap, float64(m.FirstHeap)/(1<<20))
					rss = append(rss, float64(m.PeakRSS)/(1<<20))
				}
			}
			fmt.Fprintf(&report, "| %s / %s | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f |\n", name, backend, median(first), median(resize), median(repaint), median(alloc), median(heap), median(rss))
		}
	}
	report.WriteString("\nFirst viewport includes font/input preparation, layout and rasterization, but not PNG encoding or process launch. PDF must compile the whole document and initialize PDFium. Direct preflights/parses the whole input but lays out blocks only through the viewport. Resize is 800 → 600 px with warm font/image/shaping data, after completing direct layout. Repaint draws an unchanged viewport: direct rasterizes again; PDF copies its existing raster. Neither includes Kitty encoding/transmission. Peak RSS covers first render, remaining layout, resize and repaint (before PNG artifacts), not only startup; it includes native/WASM memory. Live Go heap is a point sample before an explicit GC, not retained size. Both use the same eight pinned Go TTFs, 25 px body text and 50 px margins. Both paths use Chroma with the GitHub code theme; document styles and pagination still differ.\n")
	if err = os.WriteFile(filepath.Join(out, "RESULTS.md"), []byte(report.String()), 0600); err != nil {
		return err
	}
	fmt.Print(report.String())
	fmt.Println("\nArtifacts:", out)
	return nil
}
func median(a []float64) float64 {
	sort.Float64s(a)
	n := len(a)
	if n%2 == 1 {
		return a[n/2]
	}
	return (a[n/2-1] + a[n/2]) / 2
}
