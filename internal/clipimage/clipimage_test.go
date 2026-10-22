//go:build !js

package clipimage

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// fakeTools finds only the named tools, and records the command run.
func fakeTools(t *testing.T, found ...string) *[]*exec.Cmd {
	t.Helper()
	oldLook, oldRun := lookPath, run
	t.Cleanup(func() { lookPath, run = oldLook, oldRun })
	lookPath = func(name string) (string, error) {
		if slices.Contains(found, name) {
			return name, nil
		}
		return "", exec.ErrNotFound
	}
	var ran []*exec.Cmd
	run = func(cmd *exec.Cmd) error {
		ran = append(ran, cmd)
		return nil
	}
	return &ran
}

func TestCopyImageCommand(t *testing.T) {
	png := []byte("\x89PNG fake")
	switch runtime.GOOS {
	case "darwin":
		ran := fakeTools(t, "osascript")
		if err := Copy(context.Background(), png); err != nil {
			t.Fatal(err)
		}
		if len(*ran) != 1 {
			t.Fatalf("ran %d commands, want 1", len(*ran))
		}
		args := (*ran)[0].Args
		if args[0] != "osascript" || !strings.Contains(strings.Join(args, " "), "as «class PNGf»") {
			t.Fatalf("args = %q", args)
		}
	case "windows":
		ran := fakeTools(t, "powershell")
		if err := Copy(context.Background(), png); err != nil {
			t.Fatal(err)
		}
		args := (*ran)[0].Args
		joined := strings.Join(args, " ")
		for _, want := range []string{"powershell", "-NoProfile", "-STA", "-Command", "[Windows.Forms.Clipboard]::SetImage("} {
			if !strings.Contains(joined, want) {
				t.Fatalf("args %q lack %q", args, want)
			}
		}
	default:
		t.Run("wayland", func(t *testing.T) {
			t.Setenv("WAYLAND_DISPLAY", "wayland-0")
			ran := fakeTools(t, "wl-copy", "xclip")
			if err := Copy(context.Background(), png); err != nil {
				t.Fatal(err)
			}
			cmd := (*ran)[0]
			if want := []string{"wl-copy", "--type", "image/png"}; !slices.Equal(cmd.Args, want) {
				t.Fatalf("args = %q, want %q", cmd.Args, want)
			}
			if cmd.Stdin == nil {
				t.Fatal("wl-copy has no picture on its input")
			}
		})
		t.Run("x11", func(t *testing.T) {
			t.Setenv("WAYLAND_DISPLAY", "")
			ran := fakeTools(t, "wl-copy", "xclip")
			if err := Copy(context.Background(), png); err != nil {
				t.Fatal(err)
			}
			args := (*ran)[0].Args
			if len(args) != 7 || !slices.Equal(args[:6], []string{"xclip", "-selection", "clipboard", "-t", "image/png", "-i"}) {
				t.Fatalf("args = %q", args)
			}
		})
	}
}

func TestCopyImageRemovesTemporaryFile(t *testing.T) {
	var file string
	switch runtime.GOOS {
	case "darwin":
		fakeTools(t, "osascript")
	case "windows":
		fakeTools(t, "powershell")
	default:
		t.Setenv("WAYLAND_DISPLAY", "")
		fakeTools(t, "xclip")
	}
	run = func(cmd *exec.Cmd) error {
		file = cmd.Args[len(cmd.Args)-1]
		if runtime.GOOS == "windows" {
			for _, e := range cmd.Env {
				if v, ok := strings.CutPrefix(e, "GLOSS_CLIP_FILE="); ok {
					file = v
				}
			}
		}
		got, err := os.ReadFile(file)
		if err != nil || string(got) != "bytes" {
			t.Fatalf("file during copy = %q, %v", got, err)
		}
		return nil
	}
	if err := Copy(context.Background(), []byte("bytes")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file left behind: %v", err)
	}
}
