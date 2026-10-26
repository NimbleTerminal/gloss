package app

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"nimbleterminal.dev/gloss/internal/i18n"
)

// The file browser's kinds, prompt and help speak the language; a kind
// chosen is kept by its English name, so it stays chosen in another.
func TestBrowserSpeaksTheLanguage(t *testing.T) {
	for _, c := range []struct{ tag, pictures, prompt, help string }{
		{"es", "Imágenes", "Filtro:", "Explorador de archivos"},
		{"zh-Hans", "图片", "筛选：", "文件浏览器"},
	} {
		t.Run(c.tag, func(t *testing.T) {
			speaking(t, c.tag)
			m := browsing(t, folder(t))
			if view := ansi.Strip(m.View().Content); !strings.Contains(view, c.prompt) {
				t.Errorf("the browser lacks %q:\n%s", c.prompt, view)
			}
			send(m, tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
			if view := ansi.Strip(m.View().Content); !strings.Contains(view, c.pictures) {
				t.Errorf("the kinds lack %q:\n%s", c.pictures, view)
			}
			send(m, press("down"), press("space"))
			if !reflect.DeepEqual(m.browseTypes, []string{"Pictures"}) {
				t.Fatalf("kept kinds %q", m.browseTypes)
			}
			if got := m.opener.picker.ActiveFilters(); !reflect.DeepEqual(got, []string{c.pictures}) {
				t.Errorf("the chooser's kinds %q", got)
			}
			// Another language, the same kind.
			i18n.SetLanguage("en")
			m.browseFrom(m.opener.dir)
			if got := m.opener.picker.ActiveFilters(); !reflect.DeepEqual(got, []string{"Pictures"}) {
				t.Errorf("in English the chooser's kinds are %q", got)
			}
			i18n.SetLanguage(c.tag)
			m.browseFrom(m.opener.dir)
			send(m, press("?"))
			if view := ansi.Strip(m.View().Content); !strings.Contains(view, c.help) {
				t.Errorf("the browser's help lacks %q:\n%s", c.help, view)
			}
		})
	}
}
