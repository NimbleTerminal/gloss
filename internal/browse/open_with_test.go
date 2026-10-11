package browse_test

import (
	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/browse"
	"nimbleterminal.dev/gloss/internal/browse/browsetest"
	"testing"
)

func TestOpenWithIntent(t *testing.T) {
	for _, layout := range []browse.Layout{browse.LayoutList, browse.LayoutColumns, browse.LayoutPlaces} {
		d := browsetest.New(t, browse.New(browsetest.NewFS("d/a.txt"), "/d", browse.WithLayout(layout)), 80, 20)
		d.Press("alt+enter")
		found := false
		for _, msg := range d.Messages {
			if v, ok := msg.(browse.OpenWithMsg); ok {
				found = true
				if v.Path != "/d/a.txt" || v.Directory {
					t.Fatalf("%+v", v)
				}
			}
		}
		if !found {
			t.Fatal("no intent", layout)
		}
		if _, ok := d.M.Chosen(); ok {
			t.Fatal("selected file")
		}
		_, _ = d.M.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	}
}
