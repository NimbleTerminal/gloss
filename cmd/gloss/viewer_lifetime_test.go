package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/gloss/internal/app"
)

func TestViewerLifetimeRenewalAndHardLimit(t *testing.T) {
	for _, idle := range []time.Duration{0, time.Minute} {
		v := newViewerLifetime()
		v.configure(time.Minute, idle, nil)
		before := v.deadline
		v.activity(time.Now().Add(10 * time.Second))
		if v.deadline != before {
			t.Fatal("activity extended explicit deadline")
		}
	}
	v := newViewerLifetime()
	v.configure(0, time.Minute, nil)
	before := v.deadline
	v.activity(time.Now().Add(10 * time.Second))
	if !v.deadline.After(before) {
		t.Fatal("idle deadline not renewed")
	}
	before = v.deadline
	v.activity(before.Add(time.Second))
	if v.deadline != before {
		t.Fatal("expired lease revived")
	}
	v.finish("timeout")
	v.finish("closed")
	if v.snapshot(time.Now()).Reason != "timeout" {
		t.Fatal("end reason overwritten")
	}
}

func TestViewerSessionRoutesAndPageModes(t *testing.T) {
	for _, pick := range []bool{false, true} {
		s := &server{token: "token", pick: pick, lifetime: newViewerLifetime(), done: make(chan struct{})}
		s.lifetime.configure(0, time.Minute, nil)
		req := func(method, route, origin string) *httptest.ResponseRecorder {
			r := httptest.NewRequest(method, "http://127.0.0.1:1234/token/"+route, nil)
			if origin != "" {
				r.Header.Set("Origin", origin)
			}
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			return w
		}
		page := req("GET", "", "").Body.String()
		if strings.Contains(page, "Done viewing") == pick || strings.Contains(page, ">Choose files</button>") != pick {
			t.Fatal("viewer and picker chrome not distinct")
		}
		old := s.lifetime.deadline
		if req("POST", "activity", "https://evil.example").Code != http.StatusForbidden || s.lifetime.deadline != old {
			t.Fatal("foreign origin renewed lease")
		}
		if req("GET", "activity", "").Code != http.StatusMethodNotAllowed {
			t.Fatal("GET renewed lease")
		}
		if req("POST", "activity", "http://127.0.0.1:1234").Code != http.StatusNoContent || !s.lifetime.deadline.After(old) {
			t.Fatal("input did not renew")
		}
		req("POST", "finish", "")
		if !strings.Contains(req("GET", "session", "").Body.String(), `"reason":"closed"`) {
			t.Fatal("finish not reported")
		}
	}
}

func TestViewerIdleStatusReadsLeaseWithoutChangingIt(t *testing.T) {
	dir := settle(t, testToken, state{Status: statusWaiting, Deadline: time.Now().Add(-time.Hour), IdleTimeoutSeconds: 600})
	file := filepath.Join(dir, "viewer-lease.json")
	if err := writeState(file, state{Deadline: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := status(options{Status: testToken}, &out); err != nil {
		t.Fatal(err)
	}
	var got sessionStatus
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.State != "waiting" || got.IdleTimeoutSeconds != 600 || got.SecondsLeft == nil || *got.SecondsLeft < 55 {
		t.Fatalf("wrong idle status: %+v", got)
	}
}

func TestViewerTimeoutNotifiesPendingSessionPoll(t *testing.T) {
	s := serving(t, app.Options{})
	s.lifetime.configure(30*time.Millisecond, 0, nil)
	result := make(chan string, 1)
	go func() {
		r := httptest.NewRequest("GET", s.URL+"session", nil)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		result <- w.Body.String()
	}()
	if _, err := s.Wait(time.Second); err != errTimeout {
		t.Fatal(err)
	}
	select {
	case body := <-result:
		if !strings.Contains(body, `"reason":"timeout"`) {
			t.Fatal(body)
		}
	case <-time.After(time.Second):
		t.Fatal("poll not notified")
	}
	// Closing a server whose parent context ended remains safe and idempotent.
	s.Close()
}
