//go:build !js

package openwith

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"nimbleterminal.dev/ntopenwith"
)

// fakeSource offers a fixed list of apps per verb and records launches.
type fakeSource struct {
	mu       sync.Mutex
	apps     map[ntopenwith.Verb][]ntopenwith.App
	fileOnly map[ntopenwith.Verb][]ntopenwith.App // offered only when asked about a real file
	asked    []ntopenwith.Target
	launched []string
}

func (s *fakeSource) Apps(_ context.Context, t ntopenwith.Target, v ntopenwith.Verb) ([]ntopenwith.App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked = append(s.asked, t)
	apps := slices.Clone(s.apps[v])
	if t.Path != "" {
		apps = append(apps, s.fileOnly[v]...)
	}
	return apps, nil
}

func (s *fakeSource) Launch(_ context.Context, t ntopenwith.Target, v ntopenwith.Verb, app ntopenwith.App) (*exec.Cmd, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.launched = append(s.launched, string(v)+":"+app.ID)
	if app.Terminal {
		return exec.Command("true", t.Path), nil
	}
	return nil, nil
}

func newFake() *fakeSource {
	return &fakeSource{apps: map[ntopenwith.Verb][]ntopenwith.App{
		ntopenwith.Open: {{ID: "system", Name: "System default", Default: true}, {ID: "viewer", Name: "Viewer"}},
		ntopenwith.Edit: {{ID: "x", Name: "X Editor"}, {ID: "y", Name: "Y Editor"}, {ID: "vim", Name: "Vim", Terminal: true}},
	}}
}

type recorder struct {
	saved []ntopenwith.Preferences
}

func (r *recorder) save(p ntopenwith.Preferences) error {
	r.saved = append(r.saved, p)
	return nil
}

func testFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func ids(choices []Choice) []string {
	var out []string
	for _, c := range choices {
		out = append(out, c.ID)
	}
	return out
}

func TestChoicesOpenEndsWithReveal(t *testing.T) {
	h := NewHost(newFake(), ntopenwith.Preferences{}, (&recorder{}).save)
	choices, err := h.Choices(Target{Path: testFile(t, "a.pdf"), Kind: "pdf"}, Open)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(choices); !slices.Equal(got, []string{"system", "viewer", "reveal"}) {
		t.Fatalf("choices %v", got)
	}
	for _, c := range choices {
		if !c.Available {
			t.Fatalf("%s unavailable", c.ID)
		}
	}
	if !choices[0].Default || choices[2].Name == "" {
		t.Fatalf("%+v", choices)
	}
	choices, err = h.Choices(Target{Path: t.TempDir(), Directory: true}, Open)
	if err != nil || !slices.Equal(ids(choices), []string{"reveal"}) {
		t.Fatalf("directory %v %v", ids(choices), err)
	}
}

func TestChoicesEditHasNoReveal(t *testing.T) {
	h := NewHost(newFake(), ntopenwith.Preferences{}, (&recorder{}).save)
	choices, err := h.Choices(Target{Path: testFile(t, "a.md"), Kind: "markdown"}, Edit)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(choices); !slices.Equal(got, []string{"x", "y", "vim"}) {
		t.Fatalf("choices %v", got)
	}
	if !choices[2].Terminal {
		t.Fatal("terminal flag lost")
	}
	choices, err = h.Choices(Target{Path: t.TempDir(), Directory: true}, Edit)
	if err != nil || len(choices) != 0 {
		t.Fatalf("directory %v %v", ids(choices), err)
	}
}

func TestLaunchResolvesDefault(t *testing.T) {
	src := newFake()
	h := NewHost(src, ntopenwith.Preferences{}, (&recorder{}).save)
	launched, err := h.Launch(Target{Path: testFile(t, "a.pdf"), Kind: "pdf"}, Open, "")
	if err != nil {
		t.Fatal(err)
	}
	if launched.Name != "System default" || launched.Cmd != nil || !slices.Equal(src.launched, []string{"open:system"}) {
		t.Fatalf("%+v %v", launched, src.launched)
	}
	prefs := ntopenwith.Preferences{Defaults: map[ntopenwith.Verb]map[string]string{ntopenwith.Open: {"pdf": "gone"}}}
	h = NewHost(src, prefs, (&recorder{}).save)
	launched, err = h.Launch(Target{Path: testFile(t, "a.pdf"), Kind: "pdf"}, Open, "")
	if err != nil || launched.Name != "System default (saved app unavailable)" {
		t.Fatalf("fallback %+v %v", launched, err)
	}
	if _, err := h.Launch(Target{Path: testFile(t, "a.pdf"), Kind: "pdf"}, Open, "nope"); err == nil {
		t.Fatal("launched an app that is not offered")
	}
}

func TestLaunchTerminalReturnsCmd(t *testing.T) {
	src := newFake()
	h := NewHost(src, ntopenwith.Preferences{}, (&recorder{}).save)
	launched, err := h.Launch(Target{Path: testFile(t, "a.md"), Kind: "markdown"}, Edit, "vim")
	if err != nil {
		t.Fatal(err)
	}
	if launched.Cmd == nil || launched.Cmd.Process != nil || launched.Name != "Vim" {
		t.Fatalf("%+v", launched)
	}
}

func TestLaunchEditChoose(t *testing.T) {
	src := newFake()
	src.apps[ntopenwith.Edit] = src.apps[ntopenwith.Edit][:2]
	h := NewHost(src, ntopenwith.Preferences{}, (&recorder{}).save)
	_, err := h.Launch(Target{Path: testFile(t, "a.png"), Kind: "image/*"}, Edit, "")
	if !errors.Is(err, ntopenwith.ErrChoose) {
		t.Fatalf("err %v", err)
	}
	src.apps[ntopenwith.Edit] = nil
	_, err = h.Launch(Target{Path: testFile(t, "a.png"), Kind: "image/*"}, Edit, "")
	if !errors.Is(err, ntopenwith.ErrNoEditor) {
		t.Fatalf("err %v", err)
	}
}

func TestSetDefaultSavesAndSystemClears(t *testing.T) {
	rec := &recorder{}
	h := NewHost(newFake(), ntopenwith.Preferences{}, rec.save)
	pdf := Target{Path: testFile(t, "a.pdf"), Kind: "pdf"}
	if err := h.SetDefault(pdf, Edit, "x"); err != nil {
		t.Fatal(err)
	}
	if len(rec.saved) != 1 || rec.saved[0].Default(Edit, "pdf") != "x" {
		t.Fatalf("%+v", rec.saved)
	}
	if err := h.SetDefault(Target{Kind: "pdf"}, Open, "viewer"); err != nil {
		t.Fatal(err)
	}
	if rec.saved[1].Default(Open, "pdf") != "viewer" {
		t.Fatalf("%+v", rec.saved[1])
	}
	if err := h.SetDefault(pdf, Open, "system"); err != nil {
		t.Fatal(err)
	}
	if last := rec.saved[len(rec.saved)-1]; last.Default(Open, "pdf") != "" || last.Default(Edit, "pdf") != "x" {
		t.Fatalf("%+v", last)
	}
}

func TestSetDefaultSaveFailureKeepsPreferences(t *testing.T) {
	src := newFake()
	h := NewHost(src, ntopenwith.Preferences{}, func(ntopenwith.Preferences) error { return errors.New("read-only") })
	if err := h.SetDefault(Target{Kind: "pdf"}, Open, "viewer"); err == nil {
		t.Fatal("save failure lost")
	}
	launched, err := h.Launch(Target{Path: testFile(t, "a.pdf"), Kind: "pdf"}, Open, "")
	if err != nil || launched.Name != "System default" {
		t.Fatalf("unsaved default applied: %+v %v", launched, err)
	}
}

// An app the chooser listed for the real file can be its kind's default,
// though the kind's stand-in extension would not be offered it.
func TestSetDefaultChecksTheListedFile(t *testing.T) {
	src := newFake()
	src.fileOnly = map[ntopenwith.Verb][]ntopenwith.App{ntopenwith.Open: {{ID: "heicview", Name: "HEIC Viewer"}}}
	rec := &recorder{}
	h := NewHost(src, ntopenwith.Preferences{}, rec.save)
	heic := Target{Path: testFile(t, "a.heic"), Kind: "image/*"}
	if err := h.SetDefault(heic, Open, "heicview"); err != nil {
		t.Fatal(err)
	}
	if len(rec.saved) != 1 || rec.saved[0].Default(Open, "image/*") != "heicview" {
		t.Fatalf("%+v", rec.saved)
	}
	if err := h.SetDefault(Target{Kind: "image/*"}, Open, "heicview"); !errors.Is(err, ntopenwith.ErrNotOffered) {
		t.Fatalf("kind alone: %v", err)
	}
}

func TestSetDefaultRefusesActionsAndUnknownKinds(t *testing.T) {
	rec := &recorder{}
	h := NewHost(newFake(), ntopenwith.Preferences{}, rec.save)
	path := testFile(t, "a.pdf")
	for _, c := range []struct {
		t  Target
		id string
	}{
		{Target{Path: path, Kind: "pdf"}, "reveal"},
		{Target{Path: path, Kind: "pdf"}, "copy"},
		{Target{Path: path}, "viewer"},
		{Target{Path: path, Kind: "png"}, "viewer"},
		{Target{Path: t.TempDir(), Directory: true, Kind: "pdf"}, "viewer"},
	} {
		if err := h.SetDefault(c.t, Open, c.id); err == nil {
			t.Errorf("saved %s for %+v", c.id, c.t)
		}
	}
	if len(rec.saved) != 0 {
		t.Fatalf("%+v", rec.saved)
	}
}

func TestKindOnlyUsesExt(t *testing.T) {
	src := newFake()
	h := NewHost(src, ntopenwith.Preferences{}, (&recorder{}).save)
	if _, err := h.Choices(Target{Kind: "pdf"}, Open); err != nil {
		t.Fatal(err)
	}
	got := src.asked[len(src.asked)-1]
	if got.Ext != ".pdf" || got.Kind != "pdf" || got.Text {
		t.Fatalf("%+v", got)
	}
	if _, err := h.Choices(Target{Kind: "markdown"}, Edit); err != nil {
		t.Fatal(err)
	}
	if got := src.asked[len(src.asked)-1]; got.Ext != ".md" || !got.Text {
		t.Fatalf("%+v", got)
	}
	path := testFile(t, "a.pdf")
	if _, err := h.Choices(Target{Path: path, Kind: "pdf"}, Open); err != nil {
		t.Fatal(err)
	}
	if got := src.asked[len(src.asked)-1]; got.Ext != "" || got.Path != path {
		t.Fatalf("%+v", got)
	}
}

func TestAppsAndSetEnabled(t *testing.T) {
	rec := &recorder{}
	h := NewHost(newFake(), ntopenwith.Preferences{}, rec.save)
	m, ok := h.(Manager)
	if !ok {
		t.Fatal("host is not a Manager")
	}
	apps := m.Apps()
	var names []string
	for _, a := range apps {
		names = append(names, a.Name)
		if !a.Enabled {
			t.Fatalf("%s hidden", a.ID)
		}
	}
	// Terminal editors cannot be hidden, so they are not listed.
	if !slices.Equal(names, []string{"Viewer", "X Editor", "Y Editor"}) {
		t.Fatalf("apps %v", names)
	}
	if err := m.SetEnabled("viewer", false); err != nil {
		t.Fatal(err)
	}
	if len(rec.saved) != 1 || !rec.saved[0].IsHidden("viewer") {
		t.Fatalf("%+v", rec.saved)
	}
	choices, _ := h.Choices(Target{Path: testFile(t, "a.pdf"), Kind: "pdf"}, Open)
	if slices.Contains(ids(choices), "viewer") {
		t.Fatal("hidden app still offered")
	}
	for _, a := range m.Apps() {
		if a.ID == "viewer" && a.Enabled {
			t.Fatal("hidden app enabled")
		}
	}
	for _, id := range []string{"system", "reveal"} {
		if err := m.SetEnabled(id, false); err == nil {
			t.Fatalf("hid %s", id)
		}
	}
}

func TestRevealLaunch(t *testing.T) {
	src := newFake()
	h := NewHost(src, ntopenwith.Preferences{}, (&recorder{}).save)
	if _, err := h.Launch(Target{Path: t.TempDir(), Directory: true}, Open, ""); err == nil {
		t.Fatal("opened a directory with an app")
	}
	if _, err := h.Launch(Target{Path: t.TempDir(), Directory: true}, Edit, "x"); err == nil {
		t.Fatal("edited a directory")
	}
	if len(src.launched) != 0 {
		t.Fatalf("%v", src.launched)
	}
}

func TestSourceAliases(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "freebsd", "openbsd", "netbsd"} {
		if got := aliases(goos); len(got) == 0 {
			t.Fatalf("no aliases on %s", goos)
		}
	}
	if len(aliases("darwin")) != 6 || len(aliases("linux")) != 4 || len(aliases("windows")) != 0 {
		t.Fatal("alias table changed")
	}
	if Source() == nil {
		t.Fatal("no source")
	}
}
