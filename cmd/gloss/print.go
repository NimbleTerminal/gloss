package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/gloss/internal/document"
)

func printSize(opts options, stdout *os.File) (int, int) {
	w, h := 80, 24
	if cols, rows, err := term.GetSize(stdout.Fd()); err == nil && cols > 0 && rows > 0 {
		w, h = min(cols, 512), min(rows, 256)
	}
	if opts.PrintWidth > 0 {
		w = opts.PrintWidth
	}
	if opts.PrintHeight > 0 {
		h = opts.PrintHeight
	}
	return w, h
}

// printFiles shares the bounded loaders with exports, but writes terminal
// content rather than a PNG. It starts no Tea program and probes no terminal.
func printFiles(opts options, stdout io.Writer, width, height int) error {
	loader := &document.Loader{}
	defer loader.Close()
	for i, path := range opts.Files {
		q := exportRequest(opts.Options, path, i+1)
		r := loader.Load(q) // Animate is false: every picture stays on frame one.
		if r.Err != nil {
			return fmt.Errorf("%s: %w", path, r.Err)
		}
		view, err := app.PrintView(context.Background(), r, opts.Options, width, height)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if i > 0 {
			if _, err := io.WriteString(stdout, "\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(stdout, view+"\n"); err != nil {
			return err
		}
	}
	return nil
}
