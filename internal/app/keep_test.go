package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func glyphs(view string) bool { return strings.ContainsAny(view, "▄▀") }

func TestMainScreenIsAnOption(t *testing.T) {
	m := loaded(t, "photo.png")
	if !m.View().AltScreen {
		t.Fatal("gloss takes the alternate screen unless told otherwise")
	}
	kept := viewing(t, Options{Files: []string{"photo.png"}, KeepScreen: true}, "")
	if kept.View().AltScreen {
		t.Fatal("the main screen was asked for")
	}
}

func TestQuittingLeavesTheLastViewOnTheMainScreen(t *testing.T) {
	m := loaded(t, "photo.png")
	m.opts.KeepScreen = true
	if view := m.View().Content; !glyphs(view) || !strings.Contains(view, "q quit") {
		t.Fatalf("before quitting:\n%s", view)
	}
	m.Update(press("?"))
	_, cmd := m.Update(press("q"))
	if cmd == nil {
		t.Fatal("q did not quit")
	}
	view := m.View()
	if view.AltScreen || !glyphs(view.Content) || !strings.Contains(view.Content, "photo.png") {
		t.Fatalf("the picture did not stay:\n%s", view.Content)
	}
	// What is left behind is a picture, not a program: no keys are offered,
	// and the mouse is given back.
	if strings.Contains(view.Content, "q quit") || strings.Contains(view.Content, "a visual pager") || view.MouseMode != tea.MouseModeNone {
		t.Fatalf("the last view still offers keys:\n%s", view.Content)
	}
	// Bubble Tea erases the last row of the frame on its way out, and draws
	// a frame of another height below the old one rather than over it. So
	// the last view is as tall as those before it, and ends on an empty
	// row, which the prompt takes.
	lines := strings.Split(view.Content, "\n")
	if len(lines) != 30 || lines[29] != "" || !strings.Contains(lines[28], "photo.png") {
		t.Fatalf("%d rows of 30, ending %q, %q", len(lines), lines[len(lines)-2], lines[len(lines)-1])
	}
}

func TestQuittingClearsTheAlternateScreenAsBefore(t *testing.T) {
	m := loaded(t, "photo.png")
	m.Update(press("q"))
	if view := m.View(); !view.AltScreen || glyphs(view.Content) {
		t.Fatalf("graphics must be cleared before the screen is given back:\n%s", view.Content)
	}
}

func TestQuittingFromTheBrowserLeavesTheDocument(t *testing.T) {
	m := browsing(t, folder(t))
	m.opts.KeepScreen = true
	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if view := m.View().Content; m.opener != nil || strings.Contains(view, "Filter:") || !strings.Contains(view, "alpha.png") {
		t.Fatalf("last view:\n%s", view)
	}
}

func TestPagerAndPreviewHaveDistinctMessageOwners(t *testing.T) {
	seen := map[int]bool{}
	for range 8 {
		m := New(Options{Files: []string{"a.png", "b.png"}, Menu: true, Preview: true, Render: "glyph", Page: 1})
		m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		if m.preview == nil {
			t.Fatal("no preview")
		}
		for _, id := range []int{m.ownerID, m.preview.ownerID} {
			if id <= 0 || seen[id] {
				t.Fatalf("reused message owner %d", id)
			}
			seen[id] = true
		}
		m.Close()
	}
}

func TestShiftQKeepsTheViewWhateverWasLaunched(t *testing.T) {
	// From the alternate screen, Q leaves the last view on the main screen,
	// as -X would have; q clears it as ever.
	m := loaded(t, "photo.png")
	if !m.View().AltScreen {
		t.Fatal("not on the alternate screen")
	}
	_, cmd := m.Update(press("Q"))
	if cmd == nil {
		t.Fatal("Q did not quit")
	}
	view := m.View()
	if view.AltScreen || !glyphs(view.Content) || !strings.Contains(view.Content, "photo.png") || strings.Contains(view.Content, "q quit") {
		t.Fatalf("the picture did not stay:\n%s", view.Content)
	}
	// With -X, Q and q are the same: the view stays.
	kept := loaded(t, "photo.png")
	kept.opts.KeepScreen = true
	kept.Update(press("Q"))
	if view := kept.View(); view.AltScreen || !glyphs(view.Content) {
		t.Fatalf("with -X:\n%s", view.Content)
	}
}
