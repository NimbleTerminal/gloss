package main

import (
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"nimbleterminal.dev/ntqrcode/qrcode"
)

// qrEdge is about how wide a --qr picture is drawn: large enough to stay
// sharp when a viewer scales it up, and within an export's default edge.
const qrEdge = 1024

// qrFile writes text as a QR code, a PNG named qr.png in a folder of its own,
// for gloss to open like any other picture. The text is only encoded, never
// fetched. remove deletes the folder.
func qrFile(text string) (path string, remove func(), err error) {
	code, err := qrcode.Encode(text, qrcode.Options{})
	if err != nil {
		return "", nil, fmt.Errorf("--qr: %w", err)
	}
	img, err := code.Image(min(qrcode.MaxScale, max(1, qrEdge/len(code.Matrix()))))
	if err != nil {
		return "", nil, fmt.Errorf("--qr: %w", err)
	}
	dir, err := os.MkdirTemp("", "gloss-qr-*")
	if err != nil {
		return "", nil, err
	}
	remove = func() { _ = os.RemoveAll(dir) }
	path = filepath.Join(dir, "qr.png")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		remove()
		return "", nil, err
	}
	encodeErr := png.Encode(f, img)
	if err := errors.Join(encodeErr, f.Close()); err != nil {
		remove()
		return "", nil, err
	}
	return path, remove, nil
}
