package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/i18n"
)

// langModel is a document viewer in English that records its language saves.
func langModel(t *testing.T) (*Model, *[]string) {
	t.Helper()
	i18n.SetLanguage("en")
	t.Cleanup(func() { i18n.SetLanguage("en") })
	m := themedModel(t, "dark", false)
	var saved []string
	m.opts.SaveLang = func(tag string) error { saved = append(saved, tag); return nil }
	return m, &saved
}

func pressF2(t *testing.T, m *Model) {
	t.Helper()
	cmd, _ := m.key(tea.KeyPressMsg{Code: tea.KeyF2})
	drainAnimation(t, m, cmd)
}

func TestLangPickerPreviewAndCancel(t *testing.T) {
	m, saved := langModel(t)
	pressF2(t, m)
	if m.layer != layerLang || m.langPicker == nil {
		t.Fatal("F2 did not open the language picker")
	}
	p := m.langPicker
	if got := p.choices[p.cursor].Tag; got != "" {
		t.Fatalf("cursor starts on %q, want System", got)
	}
	view := ansi.Strip(m.View().Content)
	for _, want := range []string{"Languages", "System", "English", "Español (Spanish)", "简体中文 (Chinese, Simplified)"} {
		if !strings.Contains(view, want) {
			t.Fatalf("picker lacks %q:\n%s", want, view)
		}
	}
	drainAnimation(t, m, m.langPickerKey("down")) // English
	drainAnimation(t, m, m.langPickerKey("j"))    // Español
	if i18n.Language() != "es" || m.opts.Lang != "es" {
		t.Fatalf("preview: language %q, opts %q", i18n.Language(), m.opts.Lang)
	}
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "Idiomas") {
		t.Fatalf("picker not redrawn in Spanish:\n%s", view)
	}
	if !strings.Contains(helpScreen(), "En todas partes") {
		t.Fatal("help not in Spanish during the preview")
	}
	drainAnimation(t, m, m.langPickerKey("esc"))
	if i18n.Language() != "en" || m.opts.Lang != "" || m.layer != layerNone || m.langPicker != nil {
		t.Fatalf("Esc kept %q / %q, layer %v", i18n.Language(), m.opts.Lang, m.layer)
	}
	if len(*saved) != 0 {
		t.Fatalf("Esc saved %v", *saved)
	}
	for _, closer := range []string{"q", "f2"} {
		pressF2(t, m)
		drainAnimation(t, m, m.langPickerKey("down"))
		drainAnimation(t, m, m.langPickerKey("down"))
		drainAnimation(t, m, m.langPickerKey(closer))
		if i18n.Language() != "en" || m.layer != layerNone {
			t.Fatalf("%s did not restore: %q", closer, i18n.Language())
		}
	}
}

func TestLangPickerKeepSavesAndReloads(t *testing.T) {
	m, saved := langModel(t)
	pressF2(t, m)
	for range 3 {
		drainAnimation(t, m, m.langPickerKey("down"))
	}
	generation := m.generation
	drainAnimation(t, m, m.langPickerKey("enter"))
	if i18n.Language() != "zh-Hans" || m.opts.Lang != "zh-Hans" || m.layer != layerNone {
		t.Fatalf("Enter kept %q / %q", i18n.Language(), m.opts.Lang)
	}
	if fmt.Sprint(*saved) != "[zh-Hans]" {
		t.Fatalf("saved %v", *saved)
	}
	if m.generation == generation {
		t.Fatal("the document was not loaded again")
	}
	if got := m.request(false).Locale.Tag; got != "zh-Hans" {
		t.Fatalf("document locale %q", got)
	}
	// It opens on the kept choice; System saves "".
	pressF2(t, m)
	if p := m.langPicker; p.choices[p.cursor].Tag != "zh-Hans" {
		t.Fatalf("cursor on %q", p.choices[p.cursor].Tag)
	}
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "语言") {
		t.Fatalf("picker not in Chinese:\n%s", view)
	}
	drainAnimation(t, m, m.langPickerKey("home"))
	drainAnimation(t, m, m.langPickerKey("enter"))
	if fmt.Sprint(*saved) != "[zh-Hans ]" || i18n.Language() != "en" || m.opts.Lang != "" {
		t.Fatalf("System: saved %q, language %q / %q", *saved, i18n.Language(), m.opts.Lang)
	}
	// Enter on the choice already kept loads and saves nothing again.
	pressF2(t, m)
	generation = m.generation
	drainAnimation(t, m, m.langPickerKey("enter"))
	if m.generation != generation {
		t.Fatal("an unchanged language reloaded the document")
	}
	if len(*saved) != 2 {
		t.Fatalf("an unchanged choice saved again: %q", *saved)
	}
}

// A saved choice that matches the environment is still a choice: Enter on
// it, untouched, must not forget it.
func TestLangPickerKeepsASavedChoice(t *testing.T) {
	m, saved := langModel(t)
	m.opts.Lang, m.opts.SystemLang, m.opts.SavedLang = "es", "es-ES", "es"
	i18n.SetLanguage("es")
	pressF2(t, m)
	if p := m.langPicker; p.choices[p.cursor].Tag != "es" {
		t.Fatalf("cursor on %q, want es", p.choices[p.cursor].Tag)
	}
	drainAnimation(t, m, m.langPickerKey("enter"))
	for _, tag := range *saved {
		if tag == "" {
			t.Fatalf("untouched Enter forgot the saved choice: %q", *saved)
		}
	}
	if m.opts.SavedLang != "es" || i18n.Language() != "es" {
		t.Fatalf("saved %q, language %q", m.opts.SavedLang, i18n.Language())
	}
	// Choosing System, then, forgets it, and reopening starts there.
	pressF2(t, m)
	drainAnimation(t, m, m.langPickerKey("home"))
	drainAnimation(t, m, m.langPickerKey("enter"))
	if len(*saved) == 0 || (*saved)[len(*saved)-1] != "" || m.opts.SavedLang != "" {
		t.Fatalf("System: saved %q, SavedLang %q", *saved, m.opts.SavedLang)
	}
	pressF2(t, m)
	if p := m.langPicker; p.choices[p.cursor].Tag != "" {
		t.Fatalf("cursor on %q, want System", p.choices[p.cursor].Tag)
	}
	// English chosen over System is a choice of its own.
	drainAnimation(t, m, m.langPickerKey("down"))
	drainAnimation(t, m, m.langPickerKey("enter"))
	if (*saved)[len(*saved)-1] != "en" || m.opts.SavedLang != "en" {
		t.Fatalf("English: saved %q", *saved)
	}
}

func TestLangPickerOwnsTheMouse(t *testing.T) {
	m, _ := langModel(t)
	pressF2(t, m)
	offset := m.markdown.offset
	for _, msg := range []tea.Msg{
		tea.MouseClickMsg{Button: tea.MouseRight, X: 10, Y: 5},
		tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: 5},
		tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 10, Y: 5},
	} {
		drainAnimation(t, m, m.handle(msg))
		if m.layer != layerLang || m.langPicker == nil {
			t.Fatalf("%T took the picker down: layer %v", msg, m.layer)
		}
	}
	if m.markdown.offset != offset {
		t.Fatalf("the wheel scrolled the document under the picker: %d to %d", offset, m.markdown.offset)
	}
}

func TestLangPickerHintAndInputs(t *testing.T) {
	m, _ := langModel(t)
	pressF2(t, m)
	if !strings.Contains(m.hints(), "preview language") {
		t.Fatalf("hint %q", m.hints())
	}
	drainAnimation(t, m, m.langPickerKey("esc"))
	// Typed into a prompt, F2 and T are not pickers.
	for _, open := range []func(){
		func() { m.lineInput = &lineInput{} },
		func() { m.placeInput = &placeInput{} },
	} {
		open()
		pressF2(t, m)
		cmd, _ := m.key(tea.KeyPressMsg{Code: 'T', Text: "T"})
		drainAnimation(t, m, cmd)
		if m.langPicker != nil || m.themePicker != nil {
			t.Fatal("a picker opened over a prompt")
		}
		m.lineInput, m.placeInput = nil, nil
	}
}

// Where --lang or GLOSS_LANG decides the language at start, keeping a choice
// says that it counts only without them.
func TestLangPickerNotesAnOverride(t *testing.T) {
	m, saved := langModel(t)
	m.opts.LangFixed = true
	pressF2(t, m)
	drainAnimation(t, m, m.langPickerKey("down"))
	drainAnimation(t, m, m.langPickerKey("down"))
	drainAnimation(t, m, m.langPickerKey("enter"))
	if fmt.Sprint(*saved) != "[es]" {
		t.Fatalf("saved %q", *saved)
	}
	if !strings.Contains(m.note, "--lang") || !strings.Contains(m.note, "GLOSS_LANG") {
		t.Fatalf("note %q", m.note)
	}
	// No note where nothing is saved.
	m.opts.SaveLang = nil
	pressF2(t, m)
	drainAnimation(t, m, m.langPickerKey("down"))
	drainAnimation(t, m, m.langPickerKey("enter"))
	if m.note != "" {
		t.Fatalf("note %q without saving", m.note)
	}
}

func TestLangPickerSystemFollowsTheEnvironment(t *testing.T) {
	m, saved := langModel(t)
	// Started with a saved Spanish over a Chinese environment.
	m.opts.Lang, m.opts.SystemLang, m.opts.SavedLang = "es", "zh-CN", "es"
	i18n.SetLanguage("es")
	pressF2(t, m)
	p := m.langPicker
	if p.choices[p.cursor].Tag != "es" {
		t.Fatalf("cursor on %q, want es", p.choices[p.cursor].Tag)
	}
	drainAnimation(t, m, m.langPickerKey("home"))
	if i18n.Language() != "zh-Hans" || m.opts.Lang != "zh-CN" {
		t.Fatalf("System previews %q / %q", i18n.Language(), m.opts.Lang)
	}
	drainAnimation(t, m, m.langPickerKey("enter"))
	if len(*saved) != 1 || (*saved)[0] != "" {
		t.Fatalf("saved %q", *saved)
	}
}

func TestLangPickerSaveErrorAndHosts(t *testing.T) {
	m, _ := langModel(t)
	m.opts.SaveLang = func(string) error { return fmt.Errorf("read-only preferences") }
	pressF2(t, m)
	drainAnimation(t, m, m.langPickerKey("down"))
	drainAnimation(t, m, m.langPickerKey("down"))
	drainAnimation(t, m, m.langPickerKey("enter"))
	if !strings.Contains(m.note, "save language: read-only preferences") {
		t.Fatalf("note %q", m.note)
	}
	// Hosts without SaveLang still switch the language. The picker opens
	// on Español, kept above.
	m.opts.SaveLang = nil
	pressF2(t, m)
	drainAnimation(t, m, m.langPickerKey("down"))
	drainAnimation(t, m, m.langPickerKey("enter"))
	if i18n.Language() != "zh-Hans" {
		t.Fatalf("language %q", i18n.Language())
	}
	// Not over help or another layer.
	m.help = true
	pressF2(t, m)
	if m.langPicker != nil {
		t.Fatal("opened over help")
	}
	m.help = false
	m.openThemePicker()
	pressF2(t, m)
	if m.langPicker != nil {
		t.Fatal("opened over the theme picker")
	}
}

func TestLangPickerNarrow(t *testing.T) {
	m, _ := langModel(t)
	m.openLangPicker()
	for _, tag := range []string{"en", "es", "zh-Hans"} {
		i18n.SetLanguage(tag)
		for _, size := range [][2]int{{10, 4}, {24, 8}, {40, 15}, {80, 24}} {
			s := m.langPickerView(size[0], size[1])
			rows := strings.Split(s, "\n")
			if len(rows) > size[1] {
				t.Fatalf("%s %v: %d rows", tag, size, len(rows))
			}
			for _, r := range rows {
				if ansi.StringWidth(r) > size[0] {
					t.Fatalf("%s %v: row %q too wide", tag, size, ansi.Strip(r))
				}
			}
		}
	}
}
