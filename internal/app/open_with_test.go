//go:build !js

package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/openwith"
	"nimbleterminal.dev/ntopenwith"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fakeOpenHost struct {
	launched  []openwith.Target
	ids       []string
	verbs     []openwith.Verb
	asked     []openwith.Verb
	saved     string
	launchErr map[openwith.Verb]error // what Launch with no app returns
	launch    *openwith.Launched      // what Launch returns, if not the Editor
}

func (h *fakeOpenHost) Choices(t openwith.Target, v openwith.Verb) ([]openwith.Choice, error) {
	h.asked = append(h.asked, v)
	if t.Directory {
		return []openwith.Choice{{ID: "reveal", Name: "Reveal", Available: true}}, nil
	}
	if v == openwith.Edit {
		return []openwith.Choice{{ID: "terminal-editor", Name: "Terminal editor", Available: true, Default: true, Terminal: true}, {ID: "editor", Name: "Editor", Available: true}}, nil
	}
	return []openwith.Choice{{ID: "system", Name: "System default", Available: true, Default: true}, {ID: "editor", Name: "Editor", Available: true}, {ID: "reveal", Name: "Reveal", Available: true}}, nil
}
func (h *fakeOpenHost) Launch(t openwith.Target, v openwith.Verb, id string) (openwith.Launched, error) {
	h.launched = append(h.launched, t)
	h.ids = append(h.ids, id)
	h.verbs = append(h.verbs, v)
	if err := h.launchErr[v]; err != nil && id == "" {
		return openwith.Launched{}, err
	}
	if h.launch != nil {
		return *h.launch, nil
	}
	return openwith.Launched{Name: "Editor"}, nil
}
func (h *fakeOpenHost) SetDefault(t openwith.Target, v openwith.Verb, id string) error {
	h.saved = t.Kind + ":" + string(v) + ":" + id
	return nil
}

// noteFile is a Markdown file open in m, which asks h.
func noteFile(t *testing.T, m *Model, h openwith.Host) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(file, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	m.opts.Files = []string{file}
	m.opts.OpenWith = h
	m.kind = "markdown"
	return file
}

func TestEditKeyLaunchesEditDefault(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	file := noteFile(t, m, h)
	send(m, press("E"))
	if len(h.launched) != 1 || h.launched[0].Path != file || h.verbs[0] != openwith.Edit || h.ids[0] != "" {
		t.Fatalf("launch %+v %v %v", h.launched, h.verbs, h.ids)
	}
	if m.note != "Editing in Editor" || m.openWithPicker != nil {
		t.Fatalf("note %q", m.note)
	}
}

func TestEditChooseOpensChooser(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{launchErr: map[openwith.Verb]error{openwith.Edit: fmt.Errorf("note.md: %w", ntopenwith.ErrChoose)}}
	noteFile(t, m, h)
	send(m, press("E"))
	if m.layer != layerOpenWith || m.openWithPicker == nil || m.openWithPicker.verb != openwith.Edit {
		t.Fatalf("chooser not opened: layer %v note %q", m.layer, m.note)
	}
	if !strings.Contains(m.View().Content, "Edit with — note.md") {
		t.Fatal(m.View().Content)
	}
	if len(h.asked) != 1 || h.asked[0] != openwith.Edit {
		t.Fatalf("asked %v", h.asked)
	}
}

func TestTabSwitchesVerb(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	noteFile(t, m, h)
	send(m, press("W"))
	if !strings.Contains(m.View().Content, "Open with — note.md") {
		t.Fatal("Open chooser not shown")
	}
	send(m, press("tab"))
	view := m.View().Content
	if !strings.Contains(view, "Edit with — note.md") || !strings.Contains(view, "Terminal editor (default) (terminal)") || strings.Contains(view, "Copy path") {
		t.Fatal(view)
	}
	if m.openWithPicker.verb != openwith.Edit || h.asked[len(h.asked)-1] != openwith.Edit {
		t.Fatalf("asked %v", h.asked)
	}
	if _, hint := m.openWithStatus(200); !strings.Contains(hint, "Enter edit · Tab open with") {
		t.Fatal(hint)
	}
	send(m, press("tab"))
	if m.openWithPicker.verb != openwith.Open || !strings.Contains(m.View().Content, "Open with — note.md") || !strings.Contains(m.View().Content, "Copy path") {
		t.Fatal("did not switch back to Open")
	}
	if _, hint := m.openWithStatus(200); !strings.Contains(hint, "Enter open · Tab edit with") {
		t.Fatal(hint)
	}
	if len(h.launched) != 0 {
		t.Fatal("Tab launched")
	}
}

func TestTabOnFolderAndBusy(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	m.opts.OpenWith = h
	dir := t.TempDir()
	m.openWithPicker = &openWithPicker{verb: openwith.Open, generation: m.generation, target: openwith.Target{Path: dir, Directory: true}, choices: []openwith.Choice{{ID: "reveal", Name: "Reveal", Available: true}}}
	m.layer = layerOpenWith
	send(m, press("tab"))
	if m.note != "Folders cannot be edited" || m.openWithPicker.verb != openwith.Open || len(h.asked) != 0 {
		t.Fatalf("folder: note %q asked %v", m.note, h.asked)
	}
	m.note = ""
	m.openWithPicker = &openWithPicker{verb: openwith.Open, busy: true, generation: m.generation, target: openwith.Target{Path: "/tmp/note.md", Kind: "markdown"}}
	send(m, press("tab"))
	if m.openWithPicker.verb != openwith.Open || len(h.asked) != 0 || m.note != "" {
		t.Fatal("busy picker took Tab")
	}
}

type noEditorHost struct{ fakeOpenHost }

func (h *noEditorHost) Choices(t openwith.Target, v openwith.Verb) ([]openwith.Choice, error) {
	if v == openwith.Edit {
		h.asked = append(h.asked, v)
		return nil, nil
	}
	return h.fakeOpenHost.Choices(t, v)
}

func TestEditBinaryNeverTerminal(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &noEditorHost{fakeOpenHost{launchErr: map[openwith.Verb]error{openwith.Edit: fmt.Errorf("pic.png: %w", ntopenwith.ErrNoEditor)}}}
	file := filepath.Join(t.TempDir(), "pic.png")
	if err := os.WriteFile(file, []byte("\x89PNG"), 0600); err != nil {
		t.Fatal(err)
	}
	m.opts.Files = []string{file}
	m.opts.OpenWith = h
	m.kind = "png"
	_, cmd := m.Update(press("E"))
	msg := cmd()
	if _, ok := msg.(editorReady); ok {
		t.Fatal("binary file handed to a terminal editor")
	}
	send(m, msg)
	if m.note != "No app can edit pic.png" || m.openWithPicker != nil {
		t.Fatalf("note %q", m.note)
	}
	// Tab to Edit lists nothing for it either: the chooser stays on Open.
	send(m, press("W"))
	send(m, press("tab"))
	if m.note != "No app can edit pic.png" || m.openWithPicker == nil || m.openWithPicker.verb != openwith.Open || m.openWithPicker.busy {
		t.Fatalf("note %q", m.note)
	}
}

func TestTerminalAppRunsAttached(t *testing.T) {
	h := &fakeOpenHost{launch: &openwith.Launched{Name: "Vim", Cmd: exec.Command("true")}}
	file := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(file, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Files: []string{file}, Render: "glyph", Theme: "dark", Page: 1, OpenWith: h})
	t.Cleanup(func() { m.Close() })
	m.width, m.height = 80, 24
	drainAnimation(t, m, m.load(false))
	send(m, press("W"))
	send(m, press("tab"))
	_, cmd := m.Update(press("enter"))
	ready, ok := cmd().(editorReady)
	if !ok || h.verbs[0] != openwith.Edit || h.ids[0] != "terminal-editor" {
		t.Fatalf("not handed off: %v %v", h.verbs, h.ids)
	}
	cmd, _ = m.terminalEditorMessage(ready)
	if cmd == nil || !m.openWithPicker.busy {
		t.Fatal("handoff not prepared")
	}
	before := len(h.launched)
	send(m, editorFinished{picker: ready.picker})
	if m.openWithPicker != nil || m.note != "Editor closed" || len(h.launched) != before {
		t.Fatalf("note %q", m.note)
	}
	if m.markdown == nil || !strings.Contains(string(m.markdown.doc.Source), "hello") || ready.picker.target.Path != file {
		t.Fatalf("document not reloaded: %v %+v", m.err, ready.picker.target)
	}
}

// w on a file whose default app is a terminal one (a Linux Terminal=true
// desktop entry) runs it attached without the chooser.
func TestOpenDefaultTerminalRunsAttached(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{launch: &openwith.Launched{Name: "Vim", Cmd: exec.Command("true")}}
	file := noteFile(t, m, h)
	m.opts.IsTemporary = func(p string) bool { return p == file }
	_, cmd := m.Update(press("w"))
	ready, ok := cmd().(editorReady)
	if !ok || m.layer == layerOpenWith || h.verbs[0] != openwith.Open || h.ids[0] != "" {
		t.Fatalf("not handed off: layer %v %v %v", m.layer, h.verbs, h.ids)
	}
	if cmd, _ = m.terminalEditorMessage(ready); cmd == nil || ready.picker.target.Path != file {
		t.Fatal("handoff not prepared")
	}
	send(m, editorFinished{picker: ready.picker})
	if m.openWithPicker != nil || !strings.HasPrefix(m.note, "Editor closed") || !strings.Contains(m.note, "temporary file") {
		t.Fatalf("note %q", m.note)
	}
}

func TestSaveDefaultPerVerb(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	noteFile(t, m, h)
	send(m, press("W"))
	send(m, press("tab"))
	send(m, press("down"))
	send(m, press("d"))
	if h.saved != "markdown:edit:editor" || len(h.launched) != 0 {
		t.Fatalf("saved %q", h.saved)
	}
	if m.note != "Default saved for markdown (edit)" {
		t.Fatal(m.note)
	}
}

func TestOpenWithPickerAndLaunch(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	file := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(file, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	m.opts.Files = []string{file}
	m.opts.OpenWith = h
	m.kind = "markdown"
	_, cmd := m.Update(press("W"))
	drainAnimation(t, m, cmd)
	if !strings.Contains(m.View().Content, "Open with") {
		t.Fatal("chooser not shown")
	}
	_, cmd = m.Update(press("down"))
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(press("d"))
	drainAnimation(t, m, cmd)
	if h.saved != "markdown:open:editor" || len(h.launched) != 0 {
		t.Fatalf("save %q launches %v", h.saved, h.launched)
	}
	_, cmd = m.Update(press("enter"))
	drainAnimation(t, m, cmd)
	if len(h.launched) != 1 || h.launched[0].Path != file || h.ids[0] != "editor" {
		t.Fatalf("launch %+v %v", h.launched, h.ids)
	}
	_, cmd = m.Update(press("w"))
	drainAnimation(t, m, cmd)
	if len(h.launched) != 2 || h.ids[1] != "" {
		t.Fatal("default launch missing")
	}
}
func TestOpenWithUnavailable(t *testing.T) {
	m := themedModel(t, "dark", false)
	_, cmd := m.Update(press("W"))
	drainAnimation(t, m, cmd)
	if !strings.Contains(m.note, "not available") {
		t.Fatal(m.note)
	}
}

func TestOpenWithSelectionAndCancellation(t *testing.T) {
	for _, screen := range []screen{screenList, screenGrid} {
		m := themedModel(t, "dark", false)
		h := &fakeOpenHost{}
		dir := t.TempDir()
		a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
		for _, p := range []string{a, b} {
			if err := os.WriteFile(p, []byte("text"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		m.opts.Files = []string{a, b}
		m.opts.OpenWith = h
		m.screen = screen
		m.index = 0
		m.selection = 1
		_, cmd := m.Update(press("W"))
		drainAnimation(t, m, cmd)
		if m.openWithPicker == nil || m.openWithPicker.target.Path != b {
			t.Fatal("wrong selected target")
		}
		_, cmd = m.Update(press("esc"))
		drainAnimation(t, m, cmd)
		if m.openWithPicker != nil || len(h.launched) != 0 || m.selection != 1 {
			t.Fatal("cancel changed selection or launched")
		}
	}
}
func TestOpenWithStaleLoadClosesPicker(t *testing.T) {
	m := themedModel(t, "dark", false)
	m.opts.OpenWith = &fakeOpenHost{}
	m.openWithPicker = &openWithPicker{operation: 1, generation: m.generation}
	old := m.generation
	m.generation++
	_, cmd := m.Update(openWithLoaded{operation: 1, generation: old})
	drainAnimation(t, m, cmd)
	if m.openWithPicker != nil {
		t.Fatal("stale picker remained stuck")
	}
}

func TestOpenWithIgnoresMouseAndDoubleSubmit(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	m.opts.OpenWith = h
	m.openWithPicker = &openWithPicker{operation: 1, generation: m.generation, target: openwith.Target{Path: "/tmp/test", Kind: "text"}, choices: []openwith.Choice{{ID: "system", Available: true}}}
	m.layer = layerOpenWith
	_, first := m.Update(press("enter"))
	_, second := m.Update(press("enter"))
	drainAnimation(t, m, second)
	if len(h.launched) != 0 {
		t.Fatal("double Enter launched before first command")
	}
	drainAnimation(t, m, first)
	if len(h.launched) != 1 {
		t.Fatal("expected single launch", h.launched)
	}
}

// The chooser lies over the screen: an animation beneath goes on showing.
func TestOpenWithKeepsTheViewBehind(t *testing.T) {
	m := animated(t, "glyph", 0)
	m.animation.Pause(true)
	h := &fakeOpenHost{}
	m.opts.OpenWith = h
	m.openWithPicker = &openWithPicker{operation: 1, generation: m.generation, busy: true}
	m.layer = layerOpenWith
	if !m.mediaVisible() {
		t.Fatal("the animation is hidden behind the chooser")
	}
	if !strings.Contains(m.View().Content, "Open with") {
		t.Fatal("chooser missing")
	}
	_, cmd := m.Update(press("esc"))
	drainAnimation(t, m, cmd)
	if !m.mediaVisible() || m.openWithPicker != nil {
		t.Fatal("view not restored")
	}
}

func TestOpenWithSmallScreenDoesNotAct(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	m.opts.OpenWith = h
	m.width = 20
	m.height = 5
	m.openWithPicker = &openWithPicker{operation: 1, generation: m.generation, target: openwith.Target{Path: "/tmp/test", Kind: "text"}, choices: []openwith.Choice{{ID: "system", Available: true}}}
	m.layer = layerOpenWith
	for _, key := range []string{"enter", "d"} {
		_, cmd := m.Update(press(key))
		drainAnimation(t, m, cmd)
	}
	if len(h.launched) > 0 || h.saved != "" {
		t.Fatal("acted on hidden choice")
	}
}
func TestOpenWithFooterFits(t *testing.T) {
	m := themedModel(t, "dark", false)
	m.openWithPicker = &openWithPicker{busy: true}
	m.layer = layerOpenWith
	for _, w := range []int{24, 30, 50, 80} {
		m.Update(tea.WindowSizeMsg{Width: w, Height: 12})
		view := m.View().Content
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > w {
				t.Fatalf("row exceeds %d: %q", w, line)
			}
		}
		if len(strings.Split(view, "\n")) > m.height {
			t.Fatalf("too many rows at %d", w)
		}
	}
}

func TestOpenWithBrowserOwnsKeys(t *testing.T) {
	m := browsing(t, folder(t))
	h := &fakeOpenHost{}
	m.opts.OpenWith = h
	pump(m, m.opener.picker.SetFilter("alpha.png"), 0)
	before := m.opener.picker.FilterValue()
	send(m, press("alt+enter"))
	if m.openWithPicker == nil || m.openWithPicker.target.Kind != "image/*" {
		t.Fatal("browser intent not adapted")
	}
	send(m, press("w"))
	if m.opener.picker.FilterValue() != before {
		t.Fatal("chooser key reached filter")
	}
	send(m, press("esc"))
	if !m.browsing() || m.opener.picker.FilterValue() != before {
		t.Fatal("cancel changed browser state")
	}
}
func TestOpenWithCanceledReplyAndSaveFailure(t *testing.T) {
	m := themedModel(t, "dark", false)
	m.opts.OpenWith = &fakeOpenHost{}
	m.openWithPicker = &openWithPicker{operation: 1, generation: m.generation}
	m.layer = layerOpenWith
	send(m, press("esc"))
	send(m, openWithLoaded{operation: 1, generation: m.generation, choices: []openwith.Choice{{ID: "system"}}})
	if m.openWithPicker != nil {
		t.Fatal("late reply reopened chooser")
	}
	m.openWithPicker = &openWithPicker{operation: 2, generation: m.generation, busy: true}
	m.layer = layerOpenWith
	send(m, openWithFinished{operation: 2, generation: m.generation, saved: true, err: fmt.Errorf("read-only config")})
	if m.openWithPicker == nil || m.openWithPicker.busy || !strings.Contains(m.note, "read-only config") {
		t.Fatal("save failure not reported")
	}
}
func TestOpenWithPreservesOriginalKindAndTemporaryProvenance(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	m.opts.OpenWith = h
	file := filepath.Join(t.TempDir(), "gloss-fetch-ordinary.html")
	if err := os.WriteFile(file, []byte("<p>Hello</p>"), 0600); err != nil {
		t.Fatal(err)
	}
	m.opts.Files = []string{file}
	m.kind = "html"
	send(m, press("w"))
	if h.launched[0].Kind != "html" || h.launched[0].Temporary {
		t.Fatal("lost original kind or inferred temp by prefix")
	}
	m.opts.IsTemporary = func(p string) bool { return p == file }
	send(m, press("w"))
	if !h.launched[1].Temporary || !strings.Contains(m.note, "temporary file") {
		t.Fatal("lost temporary provenance")
	}
}

type managingOpenHost struct {
	fakeOpenHost
	hidden bool
	fail   bool
}

func (h *managingOpenHost) Apps() []openwith.AppPreference {
	return []openwith.AppPreference{{ID: "editor", Name: "Editor", Enabled: !h.hidden}}
}
func (h *managingOpenHost) SetEnabled(id string, enabled bool) error {
	if h.fail {
		return fmt.Errorf("save failed")
	}
	h.hidden = !enabled
	return nil
}
func (h *managingOpenHost) Choices(target openwith.Target, v openwith.Verb) ([]openwith.Choice, error) {
	choices, err := h.fakeOpenHost.Choices(target, v)
	if h.hidden {
		choices = append(choices[:1], choices[2:]...)
	}
	return choices, err
}
func TestManageAppsPicker(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &managingOpenHost{}
	m.opts.OpenWith = h
	m.layer = layerOpenWith
	m.openWithPicker = &openWithPicker{target: openwith.Target{Path: "/tmp/test.md", Kind: "markdown"}, generation: m.generation}
	key := func(k string) { t.Helper(); _, cmd := m.Update(press(k)); drainAnimation(t, m, cmd) }
	key("m")
	if !strings.Contains(m.View().Content, "[x] Editor") {
		t.Fatal("missing checklist")
	}
	key("space")
	if !h.hidden || !strings.Contains(m.View().Content, "[ ] Editor") {
		t.Fatal("toggle failed")
	}
	key("esc")
	if m.openWithPicker == nil || m.openWithPicker.managing {
		t.Fatal("did not return to chooser")
	}
	for _, c := range m.openWithPicker.choices {
		if c.ID == "editor" {
			t.Fatal("hidden app still offered")
		}
	}
	key("m")
	h.fail = true
	key("space")
	if !h.hidden || !strings.Contains(m.note, "save failed") {
		t.Fatal("save failure not preserved")
	}
	h.fail = false
	key("space")
	key("enter")
	if h.hidden || m.openWithPicker.managing || len(h.launched) != 0 {
		t.Fatal("restore launched or failed")
	}
	// Closing during an asynchronous load must ignore the late result.
	_, cmd := m.Update(press("m"))
	key("esc")
	drainAnimation(t, m, cmd)
	if m.openWithPicker != nil {
		t.Fatal("late load reopened chooser")
	}
}

func TestManageAppsEmptyAndNarrow(t *testing.T) {
	m := themedModel(t, "dark", false)
	m.opts.OpenWith = &managingOpenHost{}
	m.layer = layerOpenWith
	m.openWithPicker = &openWithPicker{managing: true, generation: m.generation}
	for _, width := range []int{24, 40, 80} {
		for _, line := range strings.Split(m.openWithView(width, 6), "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("line exceeds width %d: %q", width, line)
			}
		}
	}
	for _, k := range []string{"down", "space", "d"} {
		_, cmd := m.Update(press(k))
		drainAnimation(t, m, cmd)
	}
	if m.openWithPicker == nil || !m.openWithPicker.managing {
		t.Fatal("empty checklist exited")
	}
}

// The chooser is a box over the picture it was opened on, not a blank
// screen: the picture shows around it, and the box fits the screen.
func TestOpenWithIsABoxOverThePicture(t *testing.T) {
	m := loaded(t, "photos/landscape.png")
	m.opts.OpenWith = &fakeOpenHost{}
	m.layer = layerOpenWith
	m.openWithPicker = &openWithPicker{generation: m.generation, target: openwith.Target{Path: "photos/landscape.png", Kind: "image/*"},
		choices: []openwith.Choice{{ID: "system", Name: "System default", Available: true, Default: true}, {ID: "preview", Name: "Preview", Available: true}}}
	got := rows(m)
	box, picture := -1, 0
	for i, row := range got[:m.bodyHeight()] {
		if strings.Contains(row, "Open with — landscape.png") {
			box = i
		}
		if strings.ContainsAny(row, "▀▄█") {
			picture++
		}
	}
	if box < 1 || picture == 0 {
		t.Fatalf("box at row %d, %d picture rows:\n%s", box, picture, strings.Join(got, "\n"))
	}
	if !strings.Contains(strings.Join(got, "\n"), "› System default (default)") {
		t.Fatal("choices missing")
	}
	for _, row := range got {
		if ansi.StringWidth(row) > m.width {
			t.Fatalf("row wider than the screen: %q", row)
		}
	}
	if len(got) != m.height {
		t.Fatalf("%d rows on a screen of %d", len(got), m.height)
	}
}
