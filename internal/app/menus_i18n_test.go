package app

import (
	"fmt"
	"image"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"nimbleterminal.dev/gloss/internal/i18n"
	"nimbleterminal.dev/gloss/internal/openwith"
)

// entryIDs are the internal names of the entries, in order.
func entryIDs(entries []menuEntry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.id)
	}
	return out
}

// entryByID is the entry named id; the test fails without one.
func entryByID(t *testing.T, entries []menuEntry, id string) menuEntry {
	t.Helper()
	for _, e := range entries {
		if e.id == id {
			return e
		}
	}
	t.Fatalf("no entry %q in %v", id, entryIDs(entries))
	return menuEntry{}
}

// The action menus speak the language in use; their entries are known by
// ID, which stays the same, and do the same thing.
func TestActionMenusSpeakTheLanguage(t *testing.T) {
	for _, c := range []struct {
		tag              string
		picture, table   []string
		pathNote, copied string
	}{
		{"es", []string{"Abrir con…", "Editar con…", "Copiar imagen", "Copiar ruta"},
			[]string{"Abrir tabla con…", "Copiar como TSV", "Copiar como Markdown"}, "Ruta copiada", "Tabla copiada"},
		{"zh-Hans", []string{"打开方式…", "编辑方式…", "复制图片", "复制路径"},
			[]string{"用应用打开表格…", "复制为 TSV", "复制为 Markdown"}, "路径已复制", "表格已复制"},
	} {
		t.Run(c.tag, func(t *testing.T) {
			speaking(t, c.tag)
			m := loaded(t, "photos/landscape.png")
			pictureHosts(m, t.TempDir())
			send(m, press("a"))
			if m.layer != layerActions {
				t.Fatalf("a: layer=%v", m.layer)
			}
			if got := labels(m.actions.entries); !reflect.DeepEqual(got, c.picture) {
				t.Errorf("picture entries = %q, want %q", got, c.picture)
			}
			if got, want := entryIDs(m.actions.entries), []string{"open-with", "edit-with", "copy-image", "copy-path"}; !reflect.DeepEqual(got, want) {
				t.Errorf("picture IDs = %q, want %q", got, want)
			}
			box := ansi.Strip(m.actionsView(80, 22))
			for _, label := range c.picture {
				if !strings.Contains(box, label) {
					t.Errorf("the menu lacks %q:\n%s", label, box)
				}
			}
			fits(t, box, actionsMaxW)
			// Choosing by place still runs the entry of that ID.
			i := slices.Index(entryIDs(m.actions.entries), "copy-path")
			for range i {
				send(m, press("j"))
			}
			if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd == nil || m.note != c.pathNote {
				t.Errorf("Copy path: cmd=%v note=%q, want %q", cmd != nil, m.note, c.pathNote)
			}

			cells := [][]string{{"a", "b"}, {"1", "2"}}
			entries := m.tableEntries(cells)
			if got := labels(entries); !reflect.DeepEqual(got, c.table) {
				t.Errorf("table entries = %q, want %q", got, c.table)
			}
			m.note = ""
			cmd := entryByID(t, entries, "copy-tsv").do(m)
			if cmd == nil || m.note != c.copied || !strings.Contains(fmt.Sprint(cmd()), "a\tb") {
				t.Errorf("Copy as TSV: note=%q", m.note)
			}
			m.openActions(i18n.T("app.actions.table", "Table"), entries, image.Pt(1, 1))
			fits(t, m.actionsView(80, 22), actionsMaxW)

			// A link and a sheet's cell.
			if got := labels(linkEntries(m, "https://e.org/", i18n.T("app.actions.open", "Open"))); slices.Contains(got, "Copy address") || len(got) == 0 {
				t.Errorf("link entries = %q", got)
			}
			if got := labels(addressSheet(t).cellEntries()); slices.Contains(got, "Copy value") || slices.Contains(got, "Show QR") {
				t.Errorf("cell entries = %q", got)
			}
		})
	}
}

// The chooser's titles, the rows gloss and ntopenwith name, and its keys
// speak the language; the system's apps keep their names.
func TestOpenWithChooserSpeaksTheLanguage(t *testing.T) {
	for _, c := range []struct {
		tag                                    string
		open, edit, system, reveal, copy, keys string
		terminal, manage, noApps, close        string
	}{
		{"es", "Abrir con — note.md", "Editar con — note.md", "Predeterminada del sistema (predeterminada)", "Mostrar en Finder",
			"Copiar ruta", "Tab editar con", "Editar en la terminal", "Gestionar apps — todos los formatos", "No hay apps instaladas que gestionar", "Esc cerrar"},
		{"zh-Hans", "打开方式 — note.md", "编辑方式 — note.md", "系统默认（默认）", "在 Finder 中显示",
			"复制路径", "Enter 打开", "在终端中编辑", "管理应用 — 所有格式", "没有可管理的已安装应用", "Esc 关闭"},
	} {
		t.Run(c.tag, func(t *testing.T) {
			speaking(t, c.tag)
			m := themedModel(t, "dark", false)
			m.opts.OpenWith = &managingOpenHost{} // m manage is in the keys too.
			send(m, tea.WindowSizeMsg{Width: 80, Height: 24})
			m.openWithPicker = &openWithPicker{verb: openwith.Open, target: openwith.Target{Path: "/tmp/note.md", Kind: "markdown"}, choices: []openwith.Choice{
				{ID: "system", Name: "System default", Available: true, Default: true},
				{ID: "mac:com.example.editor", Name: "Editor", Available: true},
				{ID: "reveal", Name: "Reveal in Finder", Available: true},
				{ID: "copy", Name: "Copy path", Available: true},
			}}
			m.layer = layerOpenWith
			view := ansi.Strip(m.View().Content)
			for _, want := range []string{c.open, c.system, c.reveal, c.copy, c.keys, "Editor"} {
				if !strings.Contains(view, want) {
					t.Errorf("the chooser lacks %q:\n%s", want, view)
				}
			}
			fits(t, view, 80)
			// The keys fit their row whole, to the last.
			for _, verb := range []openwith.Verb{openwith.Open, openwith.Edit} {
				m.openWithPicker.verb = verb
				_, hint := m.openWithStatus(80)
				if !strings.HasSuffix(hint, c.close) {
					t.Errorf("%s keys cut short at 80: %q", verb, hint)
				}
				if other := map[string]string{"es": "Tab abrir con", "zh-Hans": "Tab 打开方式"}[c.tag]; verb == openwith.Edit && !strings.Contains(hint, other) {
					t.Errorf("edit keys lack %q: %q", other, hint)
				}
			}
			m.openWithPicker.verb = openwith.Edit
			m.openWithPicker.choices = []openwith.Choice{{ID: "terminal-editor", Name: "Edit in terminal", Available: true, Terminal: true}}
			view = ansi.Strip(m.View().Content)
			for _, want := range []string{c.edit, c.terminal} {
				if !strings.Contains(view, want) {
					t.Errorf("the edit chooser lacks %q:\n%s", want, view)
				}
			}
			m.openWithPicker.managing = true
			view = ansi.Strip(m.View().Content)
			for _, want := range []string{c.manage, c.noApps} {
				if !strings.Contains(view, want) {
					t.Errorf("Manage apps lacks %q:\n%s", want, view)
				}
			}
			fits(t, view, 80)
		})
	}
}

// The color pickers speak the language, and a click still finds the tab
// it lands on when the tabs' names are wider.
func TestColorPickerSpeaksTheLanguage(t *testing.T) {
	for _, c := range []struct {
		tag                                  string
		title, background, palette, filament string
		reset, backdrop                      string
	}{
		{"es", "Color", "Fondo", "Paleta", "Filamento", "Restablecer", "Fondos"},
		{"zh-Hans", "颜色", "背景", "调色板", "耗材", "重置", "背景色"},
	} {
		t.Run(c.tag, func(t *testing.T) {
			speaking(t, c.tag)
			m := themedModel(t, "dark", false)
			m.colorPicker = &colorPicker{}
			view := ansi.Strip(m.colorView(80, 30))
			for _, want := range []string{c.title, c.palette, c.filament, c.reset} {
				if !strings.Contains(view, want) {
					t.Errorf("the picker lacks %q:\n%s", want, view)
				}
			}
			p := m.colorPicker
			modes := pickerModes()
			x := ansi.StringWidth(modes[0]) + 3 + ansi.StringWidth(modes[1]) + 3 + 1
			m.colorClick(p, x, p.rows.tabs)
			if p.mode != pickHex {
				t.Errorf("a click on the third tab chose mode %d", p.mode)
			}
			m.colorPicker = &colorPicker{bg: true}
			view = ansi.Strip(m.colorView(80, 30))
			for _, want := range []string{c.background, c.backdrop} {
				if !strings.Contains(view, want) {
					t.Errorf("the background picker lacks %q:\n%s", want, view)
				}
			}
		})
	}
}

// A launch the system resolved, with no app chosen, names gloss's and
// ntopenwith's own apps in the language in use, its fallback note too.
func TestDirectLaunchNamesTheAppInTheLanguage(t *testing.T) {
	for _, c := range []struct{ tag, plain, fallback, editor string }{
		{"es", "Abierto en Predeterminada del sistema", "Abierto en Predeterminada del sistema (la app guardada no está disponible)", "Editando en Editar en la terminal"},
		{"zh-Hans", "已在 系统默认 中打开", "已在 系统默认（已保存的应用不可用） 中打开", "正在 在终端中编辑 中编辑"},
	} {
		t.Run(c.tag, func(t *testing.T) {
			speaking(t, c.tag)
			for name, want := range map[string]string{"System default": c.plain, "System default (saved app unavailable)": c.fallback} {
				m := themedModel(t, "dark", false)
				h := &fakeOpenHost{launch: &openwith.Launched{Name: name}}
				noteFile(t, m, h)
				send(m, press("w"))
				if len(h.ids) != 1 || h.ids[0] != "" {
					t.Fatalf("w did not launch the default: %v", h.ids)
				}
				if m.note != want {
					t.Errorf("w with %q: note = %q, want %q", name, m.note, want)
				}
			}
			m := themedModel(t, "dark", false)
			noteFile(t, m, &fakeOpenHost{launch: &openwith.Launched{Name: "Edit in terminal"}})
			send(m, press("E"))
			if m.note != c.editor {
				t.Errorf("E: note = %q, want %q", m.note, c.editor)
			}
		})
	}
	// In English the names are the host's own.
	if got := appName("terminal-editor", "Terminal editor"); got != "Terminal editor" {
		t.Errorf("English renamed the terminal editor %q", got)
	}
}

// A picture with no file and no alt text is titled in the language in use.
func TestUntitledPictureTitle(t *testing.T) {
	speaking(t, "es")
	p := pictureItem{name: "picture", untitled: true}
	if got := itemTitle(pageItem{kind: itemPicture, picture: p}); got != "imagen" {
		t.Errorf("title = %q", got)
	}
	if p.name != "picture" {
		t.Error("the scratch name changed")
	}
}
