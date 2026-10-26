package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"nimbleterminal.dev/gloss/internal/i18n"
)

func helpRows(t *testing.T, w, h int, keys ...string) []string {
	t.Helper()
	m := New(Options{Files: []string{"a.png"}, Render: "glyph", Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.Update(press("?"))
	for _, k := range keys {
		m.Update(press(k))
	}
	var rows []string
	for _, row := range strings.Split(m.View().Content, "\n") {
		rows = append(rows, strings.TrimRight(ansi.Strip(row), " "))
	}
	return rows[:len(rows)-2] // Without the status and hint lines.
}

// At 80 columns the help is two columns under section headings, every
// feature on a row of its own, with the docs on the last row.
func TestHelpTwoColumns(t *testing.T) {
	rows := helpRows(t, 80, 24)
	if len(rows) != 22 {
		t.Fatalf("%d rows: %q", len(rows), rows)
	}
	page := strings.Join(rows, "\n")
	for _, heading := range []string{"Everywhere", "Text", "Pictures and pages", "Meshes", "GIFs"} {
		if !strings.Contains(page, heading) {
			t.Errorf("no %q heading", heading)
		}
	}
	// The two columns share rows: the first key row has quit on the left
	// and zoom on the right.
	var shared bool
	for _, row := range rows {
		if strings.Contains(row, "quit") && strings.Contains(row, "zoom") {
			shared = true
		}
		if ansi.StringWidth(row) > 80 {
			t.Errorf("row too wide: %q", row)
		}
	}
	if !shared {
		t.Fatalf("columns do not share rows: %q", rows)
	}
	if !strings.Contains(rows[0], "gloss") || !strings.Contains(rows[0], "Esc") {
		t.Errorf("title row %q", rows[0])
	}
	if last := rows[len(rows)-1]; !strings.Contains(last, DocsURL) || !strings.Contains(last, "--help") {
		t.Errorf("footer %q", last)
	}
}

// Narrower, the sections stack in one column that scrolls.
func TestHelpStacksAndScrolls(t *testing.T) {
	rows := helpRows(t, 50, 24)
	page := strings.Join(rows, "\n")
	if !strings.Contains(page, "Everywhere") || strings.Contains(page, "GIFs") {
		t.Fatalf("narrow help did not stack: %q", rows)
	}
	for _, row := range rows {
		if ansi.StringWidth(row) > 50 {
			t.Errorf("row too wide: %q", row)
		}
	}
	scrolled := helpRows(t, 50, 24, "pgdown")
	if scrolled[0] == rows[0] || !strings.Contains(strings.Join(scrolled, "\n"), "GIFs") {
		t.Fatalf("down did not scroll the help: %q", scrolled)
	}
	m := New(Options{Files: []string{"a.png"}, Render: "glyph", Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	m.Update(press("?"))
	for range 30 {
		m.Update(press("down"))
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.help || m.helpOffset != 0 {
		t.Fatal("Esc did not close the help and forget the scroll")
	}
}

// In every language the help is 22 rows within 80 cells and cuts no word
// off: each heading and note within its column and each action beside its
// keys. The translations are used.
func TestHelpFitsInEveryLanguage(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage("en") })
	screens := map[string]string{}
	for _, tag := range []string{"en", "es", "zh-Hans"} {
		i18n.SetLanguage(tag)
		screens[tag] = helpScreen()
		rows := helpRows(t, 80, 24)
		if len(rows) != 22 {
			t.Errorf("%s: %d rows", tag, len(rows))
		}
		for _, row := range rows {
			if ansi.StringWidth(row) > 80 {
				t.Errorf("%s: row too wide: %q", tag, row)
			}
		}
		for _, row := range rows {
			if strings.Contains(row, "…") {
				t.Errorf("%s: cut off: %q", tag, row)
			}
		}
		col := (80 - 4) / 2
		for _, s := range append(helpLeft(), helpRight()...) {
			if w := ansi.StringWidth(s.title); w > col {
				t.Errorf("%s: heading %q is %d cells, over %d", tag, s.title, w, col)
			}
			for _, r := range s.rows {
				limit := col - helpKeyWidth - 1
				if r.keys == "" {
					limit = col
				}
				if w := ansi.StringWidth(r.action); w > limit {
					t.Errorf("%s: %q is %d cells, over %d", tag, r.action, w, limit)
				}
			}
		}
	}
	for _, tag := range []string{"es", "zh-Hans"} {
		if screens[tag] == screens["en"] {
			t.Errorf("%s help is the English one", tag)
		}
	}
}
