package app

import (
	"image/color"
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/theme"
)

func themedModel(t *testing.T, mode string, typeset bool, render ...string) *Model {
	graphics := "glyph"
	if len(render) > 0 {
		graphics = render[0]
	}
	t.Helper()
	m := New(Options{Files: []string{"page.md"}, FilesFS: fstest.MapFS{"page.md": {Data: []byte("# Heading\n\n" + strings.Repeat("Text and **bold**.\n\n", 50))}}, Render: graphics, Theme: mode, Typeset: typeset, Page: 1})
	t.Cleanup(func() { m.Close() })
	m.width, m.height = 80, 24
	drainAnimation(t, m, m.load(false))
	return m
}

func TestTerminalThemeAutoAndOverride(t *testing.T) {
	for _, mode := range []string{"auto", "light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			m := themedModel(t, mode, false)
			if (m.themeRequest() != nil) != (mode == "auto") {
				t.Fatal("wrong query policy")
			}
			before := m.markdown.renderedText
			m.markdown.offset = 5
			_, cmd := m.Update(tea.BackgroundColorMsg{Color: color.White})
			drainAnimation(t, m, cmd)
			want := mode
			if mode == "auto" {
				want = "light"
			}
			if m.theme != want || m.markdown.offset != 5 {
				t.Fatal("theme/scroll", m.theme, m.markdown.offset)
			}
			if mode == "auto" && m.markdown.renderedText == before {
				t.Fatal("terminal text was not restyled")
			}
			_, cmd = m.Update(tea.BackgroundColorMsg{Color: color.Black})
			drainAnimation(t, m, cmd)
			if mode == "auto" && m.theme != "dark" {
				t.Fatal("live theme reply ignored")
			}
			_, cmd = m.Update(tea.BackgroundColorMsg{})
			if cmd != nil {
				drainAnimation(t, m, cmd)
			}
		})
	}
}

func TestTypesetThemeKeepsPosition(t *testing.T) {
	m := themedModel(t, "auto", true)
	cmd, _ := m.documentKey("space")
	drainAnimation(t, m, cmd)
	top := m.typesetTop
	if top == 0 {
		t.Fatal("did not scroll")
	}
	_, cmd = m.Update(tea.BackgroundColorMsg{Color: color.White})
	drainAnimation(t, m, cmd)
	if m.typesetTheme != "light" || m.typesetTop != top {
		t.Fatal("theme change reset the viewport")
	}
	if color.RGBAModel.Convert(m.source.At(0, 0)) != color.RGBAModel.Convert(color.White) {
		t.Fatal("did not repaint typeset page")
	}
}

func TestTerminalThemeFallbackAndPreview(t *testing.T) {
	m := themedModel(t, "auto", false)
	if m.theme != "dark" {
		t.Fatal("missing reply should retain dark fallback")
	}
	_, cmd := m.Update(tea.FocusMsg{})
	if cmd == nil {
		t.Fatal("focus should requery")
	}
	m.isPreview = true
	if m.themeRequest() != nil {
		t.Fatal("preview must not issue its own query")
	}
}

func TestThemeDuringPendingTypeset(t *testing.T) {
	m := themedModel(t, "auto", true)
	m.typesetTop = 100
	queued := m.queueTypesetReflow()
	msg := queued()
	cmd := m.reflowTypeset(msg.(typesetResize))
	_, changed := m.Update(tea.BackgroundColorMsg{Color: color.White})
	drainAnimation(t, m, changed)
	drainAnimation(t, m, cmd)
	if m.theme != "light" || m.typesetTheme != "light" || m.typesetTop != 100 {
		t.Fatal("late dark redraw replaced current theme")
	}
}

func TestThemeRestylesSearch(t *testing.T) {
	m := themedModel(t, "auto", false)
	queryDocument(t, m, "bold")
	if m.search == nil {
		t.Fatal("missing search")
	}
	_, cmd := m.Update(tea.BackgroundColorMsg{Color: color.White})
	drainAnimation(t, m, cmd)
	if m.search == nil || m.search.query != "bold" {
		t.Fatal("theme cleared search")
	}
}

// Mermaid diagrams are drawn in a light or dark palette at load, so a theme
// change across that line reloads a document that has them, keeping its
// place; other theme changes, and documents without diagrams, only restyle.
func TestThemeRedrawsMermaidAcrossLightAndDark(t *testing.T) {
	text := "# Diagram\n\n" + strings.Repeat("Some prose.\n\n", 6) + "```mermaid\nflowchart LR\n  A --> B\n```\n"
	m := lineModel(t, "diagram.md", text)
	drainAnimation(t, m, m.applyTheme("light"))
	if m.markdown == nil || m.markdown.doc.Diagrams != 1 {
		t.Fatal("diagram not counted")
	}
	m.markdown.offset = 3
	before := m.generation
	drainAnimation(t, m, m.applyTheme("dark"))
	if m.generation == before {
		t.Fatal("light to dark did not reload the diagram")
	}
	if m.markdown == nil || m.markdown.offset != 3 {
		t.Fatal("reload lost the reading position")
	}

	plain := lineModel(t, "plain.md", "# No diagrams\n")
	drainAnimation(t, plain, plain.applyTheme("light"))
	before = plain.generation
	drainAnimation(t, plain, plain.applyTheme("dark"))
	if plain.generation != before {
		t.Fatal("a document without diagrams reloaded")
	}
}

func TestThemeRedrawsMermaidFile(t *testing.T) {
	m := lineModel(t, "flow.mmd", "flowchart LR\n  A --> B\n")
	drainAnimation(t, m, m.applyTheme("light"))
	before := m.generation
	drainAnimation(t, m, m.applyTheme("dark"))
	if m.generation == before || m.kind != "mermaid" {
		t.Fatalf("light to dark did not reload the %q diagram", m.kind)
	}
}

func TestThemeRedrawsMermaidInPreview(t *testing.T) {
	files := fstest.MapFS{"flow.mmd": {Data: []byte("flowchart LR\n  A --> B\n")}, "flow.md": {Data: []byte("# Flow\n\n```mermaid\nflowchart LR\n  A --> B\n```\n")}, "notes.txt": {Data: []byte("notes\n")}}
	m := New(Options{Files: []string{"flow.mmd", "flow.md", "notes.txt"}, FilesFS: files, Menu: true, Preview: true, Render: "glyph", Page: 1, Theme: ""})
	t.Cleanup(func() { _ = m.Close() })
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	drainAnimation(t, m, cmd)
	drainAnimation(t, m, m.applyTheme("light"))
	p := m.preview
	if p == nil || p.kind != "mermaid" {
		t.Fatalf("preview not showing the diagram: %v", p != nil)
	}
	before := p.generation
	drainAnimation(t, m, m.applyTheme("dark"))
	if p.generation == before {
		t.Fatal("light to dark did not reload the previewed diagram")
	}

	// A Markdown document with a diagram, previewed.
	drainAnimation(t, m, m.menuKey("down"))
	if p = m.preview; p == nil || p.kind != "markdown" || p.diagrams() != 1 {
		t.Fatalf("preview not showing the Markdown diagram: %q, %d", p.kind, p.diagrams())
	}
	before = p.generation
	drainAnimation(t, m, m.applyTheme("light"))
	if p.generation == before {
		t.Fatal("dark to light did not reload the previewed Markdown")
	}
}

// Arrowing through the theme picker recolors without drawing diagrams again;
// Enter draws them once in the chosen palette, and Esc leaves them as they were.
func TestThemePickerDefersMermaidRedraw(t *testing.T) {
	text := "# Diagram\n\n```mermaid\nflowchart LR\n  A --> B\n```\n"
	for _, key := range []string{"enter", "esc"} {
		m := lineModel(t, "diagram.md", text)
		m.opts.ThemeChoices = []theme.Choice{{Name: "Light", Ref: "light"}, {Name: "Dark", Ref: "dark"}}
		drainAnimation(t, m, m.applyTheme("light"))
		drainAnimation(t, m, m.openThemePicker())
		before := m.generation
		drainAnimation(t, m, m.themePickerKey("down")) // auto, then light
		drainAnimation(t, m, m.themePickerKey("down")) // dark
		if m.theme != "dark" || m.generation != before {
			t.Fatalf("previewing %q reloaded the document", m.theme)
		}
		drainAnimation(t, m, m.themePickerKey(key))
		switch reloaded := m.generation != before; {
		case key == "enter" && !reloaded:
			t.Fatal("choosing a dark theme did not redraw the diagram")
		case key == "esc" && reloaded:
			t.Fatal("cancelling the picker redrew the diagram")
		}
	}
}
