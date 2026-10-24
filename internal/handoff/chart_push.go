package handoff

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/gloss/internal/chart"
)

// chartWait is how long a POST waits for the viewer to draw its chart.
var chartWait = 20 * time.Second

// pushChart takes a chart document for the viewer and answers what it drew;
// asked for, it gives the chart the viewer shows as a page of its own.
func (s *Server) pushChart(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		s.chartPage(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST a chart document, or GET the chart's page", http.StatusMethodNotAllowed)
		return
	}
	if !samePickOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	answer := func(code int, reply app.ChartReply) {
		if reply.Warnings == nil {
			reply.Warnings = []chart.Warning{}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(reply)
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, chart.MaxDocumentBytes+1))
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig) || len(data) > chart.MaxDocumentBytes:
		answer(http.StatusRequestEntityTooLarge, app.ChartReply{Error: "chart document exceeds 16 MiB"})
		return
	case err != nil:
		answer(http.StatusBadRequest, app.ChartReply{Error: "could not read the chart document"})
		return
	}
	doc, err := chart.Sniff(data)
	if err != nil {
		answer(http.StatusBadRequest, app.ChartReply{Error: err.Error()})
		return
	}
	reply, claim := make(chan app.ChartReply, 1), &app.PushClaim{}
	select {
	case s.charts <- app.ChartPush{Doc: doc, Reply: reply, Claim: claim}:
	default:
		answer(http.StatusServiceUnavailable, app.ChartReply{Error: "the viewer is busy"})
		return
	}
	s.lifetime.activity(time.Now())
	relay := func(got app.ChartReply) {
		code := http.StatusOK
		if got.Error != "" {
			code = http.StatusUnprocessableEntity
		}
		answer(code, got)
	}
	select {
	case got := <-reply:
		relay(got)
	case <-time.After(chartWait):
		if claim.Abandon() {
			answer(http.StatusServiceUnavailable, app.ChartReply{Error: "the viewer did not answer in time"})
			return
		}
		// The viewer kept the chart as the wait ran out: its reply is coming.
		relay(<-reply)
	case <-r.Context().Done():
		// No one is left to tell, so the chart must not land either.
		claim.Abandon()
	}
}

// chartPage draws the chart the viewer shows as a self-contained page: 404
// while no chart is shown, 422 for a chart the browser cannot draw.
func (s *Server) chartPage(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	doc := s.shown
	s.mu.Unlock()
	if doc == nil {
		http.Error(w, "no chart is shown: press C on a table in the viewer, or POST a chart document here", http.StatusNotFound)
		return
	}
	option, err := chart.Default.Option(r.Context(), *doc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	page, csp := chart.Page("gloss chart", option)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("Content-Length", strconv.Itoa(len(page)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(page)
}

// charted keeps the document the viewer's chart pane drew last, nil once no
// chart is shown, for GET /chart.
func (s *Server) charted(doc *chart.Document) {
	s.mu.Lock()
	s.shown = doc
	s.mu.Unlock()
}
