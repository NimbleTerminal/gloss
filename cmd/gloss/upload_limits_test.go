package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type uploadGateReader struct {
	first, rest *bytes.Reader
	ready       chan<- struct{}
	gate        <-chan struct{}
	once        sync.Once
}

func (r *uploadGateReader) Read(p []byte) (int, error) {
	if r.first.Len() > 0 {
		return r.first.Read(p)
	}
	r.once.Do(func() { r.ready <- struct{}{}; <-r.gate })
	return r.rest.Read(p)
}
func TestConcurrentUploadQuota(t *testing.T) {
	s := &server{drops: make(chan []string, 2)}
	s.useFolder(t.TempDir())
	s.stored.Store(maxSessionBytes - 10000)
	ready := make(chan struct{}, 2)
	gate := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for range 2 {
		var b bytes.Buffer
		m := multipart.NewWriter(&b)
		f, _ := m.CreateFormFile("file", "sample.txt")
		io.Copy(f, bytes.NewReader(bytes.Repeat([]byte("x"), 8192)))
		m.Close()
		data := b.Bytes()
		r := httptest.NewRequest("POST", "http://localhost/drop", &uploadGateReader{first: bytes.NewReader(data[:1024]), rest: bytes.NewReader(data[1024:]), ready: ready, gate: gate})
		r.Header.Set("Content-Type", m.FormDataContentType())
		wg.Add(1)
		go func() { defer wg.Done(); w := httptest.NewRecorder(); s.receive(w, r); results <- w.Code }()
	}
	for range 2 {
		select {
		case <-ready:
		case <-time.After(5 * time.Second):
			close(gate)
			t.Fatal("barrier timeout")
		}
	}
	close(gate)
	wg.Wait()
	close(results)
	accepted := 0
	for code := range results {
		if code == 200 {
			accepted++
		} else if code != 413 {
			t.Fatalf("status %d", code)
		}
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil || len(entries) != accepted {
		t.Fatalf("partial files left: %v %v", entries, err)
	}
	if accepted > 1 {
		t.Fatal("both requests spent the same quota")
	}
	if s.stored.Load() > maxSessionBytes {
		t.Fatal("quota bypass")
	}
	if want := maxSessionBytes - 10000 + int64(accepted*8192); s.stored.Load() != want {
		t.Fatalf("rollback: got %d want %d", s.stored.Load(), want)
	}
}

func TestUploadRequestBounds(t *testing.T) {
	for _, kind := range []string{"body", "parts", "concurrency"} {
		t.Run(kind, func(t *testing.T) {
			s := &server{drops: make(chan []string, 2)}
			s.useFolder(t.TempDir())
			old := maxSessionBytes
			maxSessionBytes = 1024
			t.Cleanup(func() { maxSessionBytes = old })
			var b bytes.Buffer
			m := multipart.NewWriter(&b)
			switch kind {
			case "body":
				f, _ := m.CreateFormField("ignored")
				io.Copy(f, strings.NewReader(strings.Repeat("x", (1<<20)+2048)))
			case "parts":
				for range maxDrops + 1 {
					m.WriteField("ignored", "")
				}
			case "concurrency":
				s.uploads.Store(4)
				m.WriteField("ignored", "")
			}
			m.Close()
			r := httptest.NewRequest("POST", "http://localhost/drop", &b)
			r.Header.Set("Content-Type", m.FormDataContentType())
			w := httptest.NewRecorder()
			s.receive(w, r)
			want := 413
			if kind == "concurrency" {
				want = 503
			}
			if w.Code != want {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
			if s.stored.Load() != 0 {
				t.Fatal("failed upload kept a reservation")
			}
		})
	}
}

func TestUploadIdleTimeoutRollsBack(t *testing.T) {
	old := uploadIdleTimeout
	uploadIdleTimeout = 100 * time.Millisecond
	t.Cleanup(func() { uploadIdleTimeout = old })
	s := &server{drops: make(chan []string, 2)}
	s.useFolder(t.TempDir())
	front := httptest.NewServer(http.HandlerFunc(s.receive))
	defer front.Close()
	c, err := net.Dial("tcp", strings.TrimPrefix(front.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	fmt.Fprintf(c, "POST /drop HTTP/1.1\r\nHost: localhost\r\nContent-Type: multipart/form-data; boundary=b\r\nContent-Length: 100000\r\n\r\n--b\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.txt\"\r\n\r\npartial")
	response, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 408 {
		t.Fatalf("status %d", response.StatusCode)
	}
	if s.stored.Load() != 0 {
		t.Fatal("timed-out upload kept bytes")
	}
	files, err := os.ReadDir(s.dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("partial file survived: %v %v", files, err)
	}
}
