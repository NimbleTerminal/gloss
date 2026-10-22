//go:build !darwin && !windows && !js

package clipimage

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCopyImageMissingTool(t *testing.T) {
	fakeTools(t)
	err := Copy(context.Background(), []byte("png"))
	if !errors.Is(err, ErrNoTool) {
		t.Fatalf("err = %v, want ErrNoTool", err)
	}
	if !strings.Contains(err.Error(), "wl-copy or xclip") {
		t.Fatalf("err = %q does not say what to install", err)
	}
}
