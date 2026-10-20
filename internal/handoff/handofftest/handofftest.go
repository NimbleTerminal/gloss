// Package handofftest drives a handoff page from outside, as a browser
// would: it fetches, drops files, and talks to the viewer's terminal over
// the page's WebSocket. It needs only the page's address, so the handoff
// package's tests and the command's end-to-end tests share it.
package handofftest

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// Get fetches target, after edit has changed the request if it is given.
func Get(t *testing.T, target string, edit func(*http.Request)) (int, string) {
	t.Helper()
	r, err := http.NewRequest("GET", target, nil)
	if err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(r)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// Upload is a file to drop: its name and contents.
type Upload struct {
	Name string
	Data []byte
}

// Drop posts files to target as a multipart form, and returns the status and
// the paths the server says it kept them at.
func Drop(t *testing.T, target string, edit func(*http.Request), files ...Upload) (int, []string) {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for _, f := range files {
		part, err := form.CreateFormFile("file", f.Name)
		if err != nil {
			t.Fatal(err)
		}
		part.Write(f.Data)
	}
	form.Close()
	r, err := http.NewRequest("POST", target, &body)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", form.FormDataContentType())
	if edit != nil {
		edit(r)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var reply struct{ Paths []string }
	_ = json.NewDecoder(resp.Body).Decode(&reply)
	return resp.StatusCode, reply.Paths
}

// Picture is a small PNG, a file the viewer can show.
func Picture(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// Terminal is the page's side of the conversation, without the page.
type Terminal struct {
	t      *testing.T
	conn   *websocket.Conn
	screen strings.Builder
}

// Connect opens the viewer's terminal of the page at address, at 120×30.
func Connect(t *testing.T, address string) *Terminal {
	t.Helper()
	conn, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(address, "http")+"ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetReadLimit(64 << 20)
	t.Cleanup(func() { conn.CloseNow() })
	term := &Terminal{t: t, conn: conn}
	term.Send('2', `{"cols":120,"rows":30}`)
	return term
}

// Send writes one message of the page's protocol: its kind, and payload.
func (term *Terminal) Send(kind byte, payload string) {
	term.t.Helper()
	if err := term.conn.Write(context.Background(), websocket.MessageBinary, append([]byte{kind}, payload...)); err != nil {
		term.t.Fatal(err)
	}
}

// Close drops the connection, as closing the page's tab does.
func (term *Terminal) Close() { term.conn.CloseNow() }

// Until reads what the viewer draws until it has drawn text.
func (term *Terminal) Until(text string) {
	term.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for !strings.Contains(term.screen.String(), text) {
		_, data, err := term.conn.Read(ctx)
		if err != nil {
			term.t.Fatalf("waiting for %q: %v\n%s", text, err, term.screen.String())
		}
		if len(data) > 0 && data[0] == '1' {
			term.screen.Write(data[1:])
		}
	}
	term.screen.Reset()
}

// Lines collects what a program writes, for a reader in another goroutine.
type Lines struct {
	mu   sync.Mutex
	text strings.Builder
}

func (l *Lines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.Write(p)
}

func (l *Lines) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.String()
}
