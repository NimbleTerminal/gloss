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
	if len(rows) != len(openwith.Kinds) || rows[0].Default != "system" || len(rows[0].Apps) != 2 {
		t.Fatalf("%+v", rows)
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
