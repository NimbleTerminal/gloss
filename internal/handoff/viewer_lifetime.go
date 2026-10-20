package handoff

import (
	"encoding/json"
	"math"
	"net/http"
	"sync"
	"time"
)

// Browser activity renews only an implicit viewing lease. Polling and terminal
// output (including animations) never renew it. Explicit timeouts stay fixed.
type viewerLifetime struct {
	mu         sync.Mutex
	deadline   time.Time
	idle       time.Duration
	reason     string
	changed    chan struct{}
	settled    chan struct{}
	save       func(time.Time)
	configured bool
}

func newViewerLifetime() *viewerLifetime {
	return &viewerLifetime{changed: make(chan struct{}, 1), settled: make(chan struct{})}
}

func (v *viewerLifetime) configure(timeout, idle time.Duration, save func(time.Time)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.configured {
		return
	}
	v.configured = true
	v.idle, v.save = idle, save
	if timeout > 0 {
		v.idle = 0
	} else {
		timeout = idle
	}
	if timeout > 0 {
		v.deadline = time.Now().Add(timeout)
	}
	if save != nil {
		save(v.deadline)
	}
}

func (v *viewerLifetime) activity(now time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.reason != "" || v.idle == 0 || !now.Before(v.deadline) {
		return
	}
	v.deadline = now.Add(v.idle)
	if v.save != nil {
		v.save(v.deadline)
	}
	select {
	case v.changed <- struct{}{}:
	default:
	}
}

func (v *viewerLifetime) finish(reason string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.reason != "" {
		return
	}
	v.reason = reason
	close(v.settled)
}

type viewerSession struct {
	Reason      string `json:"reason,omitempty"`
	SecondsLeft *int   `json:"seconds_left,omitempty"`
	Idle        bool   `json:"idle"`
}

func (v *viewerLifetime) snapshot(now time.Time) viewerSession {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := viewerSession{Reason: v.reason, Idle: v.idle > 0}
	if !v.deadline.IsZero() {
		n := max(0, int(math.Ceil(v.deadline.Sub(now).Seconds())))
		out.SecondsLeft = &n
	}
	return out
}

func (s *Server) viewerSession(w http.ResponseWriter, r *http.Request, route string) {
	if s.lifetime == nil {
		http.NotFound(w, r)
		return
	}
	if route == "session" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// A pending poll receives the reason before graceful HTTP shutdown.
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		select {
		case <-s.lifetime.settled:
		case <-timer.C:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.lifetime.snapshot(time.Now()))
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !samePickOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if route == "activity" {
		s.lifetime.activity(time.Now())
	} else {
		s.lifetime.finish("closed")
		s.ended.Do(func() { close(s.done) })
	}
	w.WriteHeader(http.StatusNoContent)
}
