// Package releasecheck holds a test that the macOS release signing keeps PDFs
// working. The notarized binary runs under the hardened runtime, which kills a
// process that executes code it wrote into memory itself, as the WebAssembly
// compiler behind PDFium does. The signing step must therefore grant the
// entitlement that allows unsigned executable memory, and the file it names
// must exist, or every released macOS build dies on its first PDF.
package releasecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// entitlement is the hardened-runtime exception wazero's compiled code needs:
// it maps memory, writes machine code, and makes it executable without MAP_JIT.
const entitlement = "com.apple.security.cs.allow-unsigned-executable-memory"

// entitlementsPath is the file the notarize sign block names, relative to the
// repository root.
func entitlementsPath(t *testing.T, config string) string {
	t.Helper()
	inNotarize, inSign := false, false
	for _, l := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(l)
		switch {
		case l == "notarize:":
			inNotarize = true
		case inNotarize && !strings.HasPrefix(l, " ") && trimmed != "" && !strings.HasPrefix(trimmed, "#"):
			inNotarize = false // The next top-level key.
		case inNotarize && trimmed == "sign:":
			inSign = true
		case inNotarize && inSign && trimmed == "notarize:":
			inSign = false
		case inNotarize && inSign && strings.HasPrefix(trimmed, "entitlements:"):
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "entitlements:")), `'"`)
		}
	}
	t.Fatal("notarize.macos sign block names no entitlements file")
	return ""
}

func TestReleaseSigningAllowsUnsignedExecutableMemory(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	config, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := entitlementsPath(t, string(config))
	plist, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatalf("entitlements file: %v", err)
	}
	text := string(plist)
	key := "<key>" + entitlement + "</key>"
	i := strings.Index(text, key)
	if i < 0 {
		t.Fatalf("%s lacks %s", path, entitlement)
	}
	if rest := strings.TrimSpace(text[i+len(key):]); !strings.HasPrefix(rest, "<true/>") {
		t.Fatalf("%s does not set %s to true", path, entitlement)
	}
}

// TestReleaseBuildsMacOSWithCgo keeps the macOS build on cgo, with a pinned
// minimum, and its binaries signed. Without cgo the link fails: purego
// (ntopenwith) and goffi (nt3d's GPU renderer) each bring a fakecgo, and both
// define _cgo_init. A cgo build also needs a Mac, so the release runs on one.
func TestReleaseBuildsMacOSWithCgo(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	config, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(config)
	start := strings.Index(text, "  - id: gloss-darwin\n")
	if start < 0 {
		t.Fatal("no gloss-darwin build")
	}
	build := text[start:]
	if end := strings.Index(build[1:], "\n  - id: "); end >= 0 {
		build = build[:end+1]
	}
	if end := strings.Index(build, "\n\n"); end >= 0 {
		build = build[:end]
	}
	for _, want := range []string{"CGO_ENABLED=1", "-mmacosx-version-min=12.0", "goos: [darwin]"} {
		if !strings.Contains(build, want) {
			t.Errorf("the gloss-darwin build lacks %q:\n%s", want, build)
		}
	}
	if !strings.Contains(text, "      ids:\n        - gloss-darwin\n      sign:") {
		t.Error("notarize does not sign the gloss-darwin build")
	}
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(workflow), "runs-on: macos-"); n != 2 {
		t.Errorf("release.yml: %d jobs on macOS, want the snapshot and the release", n)
	}
}
