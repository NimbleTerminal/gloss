//go:build !js

package app

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"errors"
	"fmt"
	"io"
	"nimbleterminal.dev/gloss/internal/document"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nimbleterminal.dev/gloss/internal/openwith"
)

// editorTestHost launches every app as a terminal editor: an unstarted command.
type editorTestHost struct {
	fakeOpenHost
	err error
}

func (h *editorTestHost) Launch(t openwith.Target, v openwith.Verb, id string) (openwith.Launched, error) {
	h.launched = append(h.launched, t)
	h.ids = append(h.ids, id)
	if h.err != nil {
		return openwith.Launched{}, h.err
	}
	return openwith.Launched{Name: "Test editor", Cmd: exec.Command("test-editor", t.Path)}, nil
}
func TestTerminalEditorReloadAndPosition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.md")
	before := strings.Repeat("Paragraph before. "+strings.Repeat("text ", 40)+"\n\n", 100)
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Files: []string{path}, Render: "glyph", Theme: "dark", Page: 1, OpenWith: &editorTestHost{}})
	t.Cleanup(func() { m.Close() })
	m.width, m.height = 80, 24
	drainAnimation(t, m, m.load(false))
	m.chopLines = true
	m.markdown.raw, m.markdown.offset, m.markdown.left = true, 8, 7
	p := &openWithPicker{target: openwith.Target{Path: path, Kind: "markdown"}, generation: m.generation}
	m.openWithPicker, m.layer = p, layerOpenWith
	ready := m.openWithLaunch("vim", false)().(editorReady)
	if ready.command.Args[1] != path {
		t.Fatalf("ready: %+v", ready)
	}
	// Preparing the handoff clears graphics and saves the logical text position.
	cmd, handled := m.terminalEditorMessage(ready)
	if !handled || cmd == nil || m.markdown != nil || m.savedMarkdown == nil || !p.busy {
		t.Fatal("handoff not prepared")
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(before, "before", "after")), 0600); err != nil {
		t.Fatal(err)
	}
	cmd, handled = m.terminalEditorMessage(editorFinished{picker: p})
	if !handled {
		t.Fatal("completion not handled")
	}
	drainAnimation(t, m, cmd)
	if m.openWithPicker != nil || m.markdown == nil || !m.markdown.raw || m.markdown.offset != 8 || m.markdown.left != 7 {
		t.Fatal("position/source state not restored")
	}
	if !strings.Contains(string(m.markdown.doc.Source), "after") {
		t.Fatal("file was not reloaded")
	}
}

func TestTerminalEditorErrorAndStaleReady(t *testing.T) {
	m := themedModel(t, "dark", false)
	m.opts.OpenWith = &editorTestHost{err: errors.New("set EDITOR")}
	p := &openWithPicker{generation: m.generation}
	m.openWithPicker, m.layer = p, layerOpenWith
	send(m, m.openWithLaunch("vim", false)())
	if m.openWithPicker != p || p.busy || !strings.Contains(m.note, "set EDITOR") {
		t.Fatal("preparation error lost")
	}
	m.closeOpenWith()
	if cmd, _ := m.terminalEditorMessage(editorReady{picker: p, command: exec.Command("ignored")}); cmd != nil {
		t.Fatal("canceled editor started")
	}
}

type editorProcessHost struct {
	fakeOpenHost
	command *exec.Cmd
}

func (h *editorProcessHost) Launch(openwith.Target, openwith.Verb, string) (openwith.Launched, error) {
	return openwith.Launched{Name: "Test editor", Cmd: h.command}, nil
}

type editorProgramTest struct {
	model    *Model
	finished bool
}

func (h *editorProgramTest) Init() tea.Cmd  { return h.model.openWithLaunch("vim", false) }
func (h *editorProgramTest) View() tea.View { return tea.NewView("") }
func (h *editorProgramTest) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(editorFinished); ok {
		h.finished = true
	}
	_, cmd := h.model.Update(msg)
	if _, ok := msg.(document.Result); ok && h.finished {
		return h, tea.Sequence(cmd, tea.Quit)
	}
	return h, cmd
}

func TestTerminalEditorProcessRoundTrip(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file with spaces.md")
			if err := os.WriteFile(path, []byte("before\n"), 0600); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			child := exec.Command(executable, "-test.run=^TestTerminalEditorChild$", "--", path)
			child.Env = append(os.Environ(), "GLOSS_TEST_EDITOR_CHILD=1", fmt.Sprintf("GLOSS_TEST_EDITOR_FAIL=%t", fail))
			m := New(Options{Files: []string{path}, Render: "glyph", Theme: "dark", Page: 1, OpenWith: &editorProcessHost{command: child}})
			defer m.Close()
			m.width, m.height = 80, 24
			drainAnimation(t, m, m.load(false))
			m.openWithPicker = &openWithPicker{target: openwith.Target{Path: path, Kind: "markdown"}, generation: m.generation}
			m.layer = layerOpenWith
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			harness := &editorProgramTest{model: m}
			_, err = tea.NewProgram(harness, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler(), tea.WithContext(ctx)).Run()
			if err != nil {
				t.Fatal(err)
			}
			if !harness.finished || m.markdown == nil || string(m.markdown.doc.Source) != "after\n" {
				t.Fatal("attached editor did not reload content")
			}
			if fail && !strings.Contains(m.note, "exit status") {
				t.Fatalf("exit error lost: %s", m.note)
			}
		})
	}
}
func TestTerminalEditorChild(t *testing.T) {
	if os.Getenv("GLOSS_TEST_EDITOR_CHILD") != "1" {
		return
	}
	if err := os.WriteFile(os.Args[len(os.Args)-1], []byte("after\n"), 0600); err != nil {
		os.Exit(2)
	}
	if os.Getenv("GLOSS_TEST_EDITOR_FAIL") == "true" {
		os.Exit(7)
	}
	os.Exit(0)
}

func TestTerminalEditorTypesetViewport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.md")
	content := strings.Repeat("## Heading\n\nSome paragraph text.\n\n", 100)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Files: []string{path}, Render: "glyph", Theme: "dark", Page: 1, Typeset: true, OpenWith: &editorTestHost{}})
	defer m.Close()
	m.width, m.height = 80, 24
	drainAnimation(t, m, m.load(false))
	m.typesetTop = 120
	p := &openWithPicker{target: openwith.Target{Path: path, Kind: "markdown"}, generation: m.generation}
	m.openWithPicker, m.layer = p, layerOpenWith
	ready := m.openWithLaunch("vim", false)().(editorReady)
	_, _ = m.terminalEditorMessage(ready)
	// Editors commonly save by replacing the original file.
	replacement := filepath.Join(filepath.Dir(path), "replacement.md")
	if err := os.WriteFile(replacement, []byte(strings.ReplaceAll(content, "Some", "Edited")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	cmd, _ := m.terminalEditorMessage(editorFinished{picker: p, top: 120})
	drainAnimation(t, m, cmd)
	if m.err != nil || m.typeset == nil || m.typesetTop != 120 {
		t.Fatalf("viewport not restored: top=%d err=%v", m.typesetTop, m.err)
	}
	if !strings.Contains(string(m.typeset.Source), "Edited") {
		t.Fatal("typeset source stale")
	}
}

func TestTerminalChoiceHandsOffUnstarted(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &editorTestHost{}
	m.opts.OpenWith = h
	p := &openWithPicker{generation: m.generation, target: openwith.Target{Path: "/tmp/test.md", Kind: "markdown"}, choices: []openwith.Choice{{ID: "vim", Name: "Vim", Available: true, Terminal: true}}}
	m.openWithPicker, m.layer = p, layerOpenWith
	_, cmd := m.Update(press("enter"))
	if cmd == nil || !p.busy {
		t.Fatal("launch not submitted")
	}
	ready, ok := cmd().(editorReady)
	if !ok || ready.picker != p || ready.command.Process != nil {
		t.Fatal("terminal app not handed to the editor handoff unstarted")
	}
	if len(h.launched) != 1 || h.ids[0] != "vim" || h.saved != "" {
		t.Fatalf("launches %v saved %q", h.ids, h.saved)
	}
}
