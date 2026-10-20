//go:build !js

package document

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// countWorkers records the workers started until the test ends.
func countWorkers(t *testing.T) *[]*exec.Cmd {
	t.Helper()
	old := parserCommand
	t.Cleanup(func() { parserCommand = old })
	var started []*exec.Cmd
	parserCommand = func(exe string) *exec.Cmd {
		cmd := old(exe)
		started = append(started, cmd)
		return cmd
	}
	return &started
}

func hangingWorker(t *testing.T) **exec.Cmd {
	t.Helper()
	oldCommand, oldDeadline := parserCommand, parseDeadline
	t.Cleanup(func() { parserCommand = oldCommand; parseDeadline = oldDeadline })
	var child *exec.Cmd
	parserCommand = func(exe string) *exec.Cmd {
		child = exec.Command(exe, "-test.run=^TestParserHangHelper$")
		child.Env = append(os.Environ(), "GLOSS_TEST_HANG=1")
		return child
	}
	return &child
}

func reaped(t *testing.T, child *exec.Cmd) {
	t.Helper()
	if child == nil || child.Process == nil || child.ProcessState == nil {
		t.Fatal("worker was not started and reaped")
	}
	if err := child.Process.Kill(); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("worker still alive: %v", err)
	}
	if len(parserSlots) != 0 {
		t.Fatal("worker retained its slot")
	}
}

func TestParserTimeoutKillsAndReapsWorker(t *testing.T) {
	child := hangingWorker(t)
	parseDeadline = 200 * time.Millisecond
	_, err := runParser(parserRequest{Kind: "svg"})
	if err == nil || !strings.Contains(err.Error(), "worker stopped") {
		t.Fatalf("timeout: %v", err)
	}
	reaped(t, *child)
}

func TestParserRecoversAfterATimeout(t *testing.T) {
	real := parserCommand
	hangingWorker(t)
	parseDeadline = 200 * time.Millisecond
	if _, err := runParser(parserRequest{Kind: "svg"}); err == nil {
		t.Fatal("hanging worker answered")
	}
	parserCommand, parseDeadline = real, 15*time.Second
	out, err := runParser(parserRequest{Kind: "svg", Name: "test.svg", Edge: 16, Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10"/></svg>`)})
	if err != nil || len(out.PNG) == 0 {
		t.Fatalf("recovery: %v", err)
	}
}

// Leaving the document, as a cancelled search does, stops its worker at
// once, without waiting out the deadline.
func TestParserCancelStopsTheWorker(t *testing.T) {
	child := hangingWorker(t)
	worker, err := dialParser()
	if err != nil {
		t.Fatal(err)
	}
	defer worker.close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	begun := time.Now()
	if _, err := worker.ask(ctx, parserRequest{Kind: "svg"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancelled: %v", err)
	}
	if time.Since(begun) > 5*time.Second {
		t.Fatal("cancellation waited for the worker")
	}
	reaped(t, *child)
}

// One worker opens a PDF and answers for every page; closing the document
// stops it.
func TestPDFWorkerStaysOpenWithTheDocument(t *testing.T) {
	started := countWorkers(t)
	path := filepath.Join(t.TempDir(), "three.pdf")
	if err := os.WriteFile(path, pagesPDF("BT /F1 12 Tf 10 10 Td (one) Tj ET", "BT /F1 12 Tf 10 10 Td (two) Tj ET", "BT /F1 12 Tf 10 10 Td (three) Tj ET"), 0600); err != nil {
		t.Fatal(err)
	}
	loader := &Loader{}
	generation := uint64(0)
	load := func(q Request) Result {
		generation++
		q.Path, q.Generation = path, generation
		return loader.Load(q)
	}
	for page, want := range []string{"one", "two", "three"} {
		if r := load(Request{Page: page + 1, TextOnly: true}); r.Err != nil || strings.TrimSpace(r.Text) != want {
			t.Fatalf("page %d: %q, %v", page+1, r.Text, r.Err)
		}
	}
	r := load(Request{Page: 2, DPI: 72})
	if r.Err != nil || r.Image == nil || !hasField(r.Info, "Pages", "3") {
		t.Fatalf("drawn page: %v, %v", r.Err, r.Info)
	}
	if len(*started) != 1 {
		t.Fatalf("%d workers for one document", len(*started))
	}
	worker := (*started)[0]
	if err := loader.Close(); err != nil {
		t.Fatal(err)
	}
	reaped(t, worker)
}

// --info and the i box want the fields alone: the page is not drawn.
func TestPDFDescribeDoesNotDraw(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one.pdf")
	if err := os.WriteFile(path, pagesPDF("BT /F1 12 Tf 10 10 Td (one) Tj ET"), 0600); err != nil {
		t.Fatal(err)
	}
	loader := &Loader{}
	defer loader.Close()
	r := loader.Load(Request{Path: path, Page: 1, Describe: true, Generation: 1})
	if r.Err != nil || r.Image != nil || loader.pdf != nil || !hasField(r.Info, "Page size", "200 × 200 pt (2.78 × 2.78 in)") {
		t.Fatalf("described: %v, image %v, PDFium %v, %v", r.Err, r.Image != nil, loader.pdf != nil, r.Info)
	}
}

func TestParserHangHelper(t *testing.T) {
	if os.Getenv("GLOSS_TEST_HANG") != "1" {
		return
	}
	for {
		time.Sleep(time.Hour)
	}
}
