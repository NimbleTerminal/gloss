package openwith

import "testing"

func TestExtOfEveryKind(t *testing.T) {
	for _, kind := range Kinds {
		if ext := ExtOf(kind); len(ext) < 2 || ext[0] != '.' {
			t.Fatalf("ExtOf(%q) = %q", kind, ext)
		}
	}
	if ExtOf("png") != "" || ExtOf("") != "" {
		t.Fatal("unknown kind has an extension")
	}
	if ExtOf("image/*") != ".png" || ExtOf("text") != ".txt" || ExtOf("mermaid") != ".mmd" {
		t.Fatal("extension table changed")
	}
}
