//go:build !js

package openwith

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestBuildCommand(t *testing.T) {
	file := "/tmp/-invoice $(touch nope); 'quoted', 中文.pdf"
	for _, args := range [][]string{{"/usr/bin/open", "-a", "Preview", "{}"}, {"/usr/bin/open", "-a", "Preview"}} {
		name, got, err := BuildCommand(Entry{Exec: args}, file)
		if err != nil || name != args[0] || !reflect.DeepEqual(got, []string{"-a", "Preview", file}) {
			t.Fatalf("%q %q %v", name, got, err)
		}
	}
	for _, file := range []string{"relative.pdf", "/tmp/nul\x00.pdf"} {
		if _, _, err := BuildCommand(Entry{Exec: []string{"open"}}, file); err == nil {
			t.Fatalf("accepted %q", file)
		}
	}
	for _, args := range [][]string{nil, {"{}"}, {"open", "prefix{}"}} {
		if _, _, err := BuildCommand(Entry{Exec: args}, file); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
}

type fakeChild struct {
	err             error
	waited, release chan struct{}
}

func (c *fakeChild) Start() error { return c.err }
func (c *fakeChild) Wait() error  { close(c.waited); <-c.release; return nil }
func TestStartReapsWithoutWaiting(t *testing.T) {
	c := &fakeChild{waited: make(chan struct{}), release: make(chan struct{})}
	defer close(c.release)
	err := startWith(func(string, ...string) child { return c }, Entry{Exec: []string{"open"}}, "/tmp/a")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-c.waited:
	case <-time.After(time.Second):
		t.Fatal("child not reaped")
	}
}
func TestStartFailureDoesNotWait(t *testing.T) {
	c := &fakeChild{err: errors.New("failed"), waited: make(chan struct{}), release: make(chan struct{})}
	if err := startWith(func(string, ...string) child { return c }, Entry{Exec: []string{"open"}}, "/tmp/a"); err == nil {
		t.Fatal("missing error")
	}
	select {
	case <-c.waited:
		t.Fatal("wait after failed start")
	default:
	}
}
