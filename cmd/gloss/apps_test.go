package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/gloss/internal/openwith"
	"nimbleterminal.dev/ntopenwith"
	"os/exec"
	"strings"
	"testing"
)

// systemOnly offers only the system default, for every file and verb.
type systemOnly struct{}

func (systemOnly) Apps(context.Context, ntopenwith.Target, ntopenwith.Verb) ([]ntopenwith.App, error) {
	return []ntopenwith.App{{ID: "system", Name: "System default", Default: true}}, nil
}
func (systemOnly) Launch(context.Context, ntopenwith.Target, ntopenwith.Verb, ntopenwith.App) (*exec.Cmd, error) {
	return nil, nil
}

func testHost() openwith.Host {
	return openwith.NewHost(systemOnly{}, ntopenwith.Preferences{}, func(ntopenwith.Preferences) error { return nil })
}

func TestAppsParsing(t *testing.T) {
	for _, args := range [][]string{{"--apps"}, {"--apps", "--json"}} {
		o, done, err := parse(args, io.Discard)
		if err != nil || done || !o.Apps {
			t.Fatalf("%v %v", args, err)
		}
	}
	for _, extra := range [][]string{{"file.pdf"}, {"-"}, {"--serve"}, {"--pick"}, {"--pick-web"}, {"--info"}, {"--text"}, {"--output", "a.png"}, {"--glob", "images"}, {"--status", "abc"}, {"--resume", "abc"}, {"--cancel", "abc"}, {"--menu"}, {"--fetch"}} {
		args := append([]string{"--apps"}, extra...)
		if _, _, err := parse(args, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
func TestAppsListing(t *testing.T) {
	var b bytes.Buffer
	if err := writeApps(&b, testHost(), true); err != nil {
		t.Fatal(err)
	}
	var rows []appsListing
	if err := json.Unmarshal(b.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2*len(openwith.Kinds) || rows[0].Verb != "open" || rows[0].Default != "system" || len(rows[0].Apps) != 2 {
		t.Fatalf("%+v", rows)
	}
}

// editorSource offers the system default for opening and a terminal editor for editing.
type editorSource struct{ systemOnly }

func (editorSource) Apps(_ context.Context, _ ntopenwith.Target, v ntopenwith.Verb) ([]ntopenwith.App, error) {
	if v == ntopenwith.Edit {
		return []ntopenwith.App{{ID: "terminal-editor", Name: "Edit in terminal", Terminal: true, Default: true}}, nil
	}
	return []ntopenwith.App{{ID: "system", Name: "System default", Default: true}}, nil
}

func TestAppsJSONHasVerbs(t *testing.T) {
	var b bytes.Buffer
	host := openwith.NewHost(editorSource{}, ntopenwith.Preferences{}, func(ntopenwith.Preferences) error { return nil })
	if err := writeApps(&b, host, true); err != nil {
		t.Fatal(err)
	}
	var rows []appsListing
	if err := json.Unmarshal(b.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2*len(openwith.Kinds) {
		t.Fatalf("%d rows for %d kinds", len(rows), len(openwith.Kinds))
	}
	for i, kind := range openwith.Kinds {
		open, edit := rows[2*i], rows[2*i+1]
		if open.Kind != kind || open.Verb != "open" || edit.Kind != kind || edit.Verb != "edit" {
			t.Fatalf("%s: %s %s, %s %s", kind, open.Kind, open.Verb, edit.Kind, edit.Verb)
		}
		// Only text can be edited; other kinds list no editor.
		wantEdit := ""
		if openwith.Editable(kind) {
			wantEdit = "terminal-editor"
		}
		if open.Default != "system" || edit.Default != wantEdit {
			t.Fatalf("%s defaults %q %q", kind, open.Default, edit.Default)
		}
		last := open.Apps[len(open.Apps)-1]
		if last.ID != "reveal" || last.Terminal {
			t.Fatalf("%s open apps %+v", kind, open.Apps)
		}
		for _, a := range edit.Apps {
			if a.ID == "reveal" {
				t.Fatalf("%s edit offers reveal", kind)
			}
		}
		if openwith.Editable(kind) && (len(edit.Apps) != 1 || !edit.Apps[0].Terminal) {
			t.Fatalf("%s edit apps %+v", kind, edit.Apps)
		}
		if !openwith.Editable(kind) && len(edit.Apps) != 0 {
			t.Fatalf("%s edit apps %+v", kind, edit.Apps)
		}
	}
	if !bytes.Contains(b.Bytes(), []byte(`"terminal":true`)) || !bytes.Contains(b.Bytes(), []byte(`"verb":"edit"`)) {
		t.Fatalf("%s", b.String())
	}
}

func TestAppsText(t *testing.T) {
	var b bytes.Buffer
	if err := writeApps(&b, testHost(), false); err != nil {
		t.Fatal(err)
	}
	k := openwith.Kinds[0]
	for _, want := range []string{k + " open (default: system)\n", k + " edit (default: system)\n"} {
		if !bytes.Contains(b.Bytes(), []byte(want)) {
			t.Fatalf("missing %q in %s", want, b.String())
		}
	}
}

func TestOpenWithHostBoundary(t *testing.T) {
	for _, o := range []options{{Serve: true}, {PickWeb: true}, {Info: true}, {Text: true}, {Options: app.Options{Output: "out.png"}}} {
		calls := 0
		err := configureOpenWith(&o, func() (openwith.Host, error) { calls++; return nil, nil })
		if err != nil || calls != 0 || o.OpenWith != nil {
			t.Fatalf("host acquired native launch: %+v", o)
		}
	}
}

func TestAppsSchema(t *testing.T) {
	var schema, data bytes.Buffer
	if err := writeSchema(&schema); err != nil {
		t.Fatal(err)
	}
	if err := writeApps(&data, testHost(), true); err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	var value any
	if err := json.Unmarshal(schema.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if err := conforms(root, root["$defs"].(map[string]any)["apps"].(map[string]any), value, "apps"); err != nil {
		t.Fatal(err)
	}
}

// hostile names and IDs, as an OS could report them.
type hostileSource struct{ systemOnly }

func (hostileSource) Apps(context.Context, ntopenwith.Target, ntopenwith.Verb) ([]ntopenwith.App, error) {
	return []ntopenwith.App{{ID: "mac:\x1b]0;x\a\u009b", Name: "Evil\x1b]0;x\a\u009b33m App", Default: true}}, nil
}

func TestAppsTextSanitizesOSNames(t *testing.T) {
	var b bytes.Buffer
	host := openwith.NewHost(hostileSource{}, ntopenwith.Preferences{}, func(ntopenwith.Preferences) error { return nil })
	if err := writeApps(&b, host, false); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "Evil") {
		t.Fatalf("names missing: %q", out)
	}
	for _, r := range out {
		if r != '\n' && (r < ' ' || (r >= 0x7f && r <= 0x9f)) {
			t.Fatalf("control %U reached the text output: %q", r, out)
		}
	}
}
