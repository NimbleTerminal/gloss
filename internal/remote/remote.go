// Package remote is the socket through which a running gloss viewer takes
// requests from another process on the same machine: another gloss, run with
// --remote, an editor, or a script. A viewer started with --socket NAME
// listens on a Unix domain socket; a client connects, writes one request as
// a line of JSON, and reads one line of JSON back. The shapes here are
// gloss's contract with those programs, not a public library.
package remote

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Request is what a client asks of the viewer, one line of JSON.
type Request struct {
	Action string   `json:"action" enum:"open,reload,quit,status" doc:"What the viewer is asked: open shows the paths, reload loads the file on screen again, quit ends the viewer, status says what is shown."`
	Paths  []string `json:"paths,omitempty" doc:"With open: absolute paths of the files to show, or one folder to browse, or one http(s) address when the viewer runs with --fetch."`
	Page   int      `json:"page,omitempty" doc:"With open: the page or sheet to start on, from 1."`
}

// Response is the viewer's answer, one line of JSON.
type Response struct {
	OK      bool     `json:"ok" doc:"Whether the request was carried out."`
	Error   string   `json:"error,omitempty" doc:"Why not, when ok is false."`
	Added   int      `json:"added,omitempty" doc:"With open: how many files were new to the viewer's list."`
	Skipped []string `json:"skipped,omitempty" doc:"With open: the paths that could not be shown, each with why."`
	File    string   `json:"file,omitempty" doc:"The file on screen after the request, or being loaded; empty when the viewer shows none."`
	Kind    string   `json:"kind,omitempty" doc:"The kind of that file, as --info names it, once it is loaded."`
	Page    int      `json:"page,omitempty" doc:"Its page or sheet, from 1."`
	Pages   int      `json:"pages,omitempty" doc:"How many pages or sheets it has."`
	Files   int      `json:"files,omitempty" doc:"How many files the viewer lists."`
}

// ErrNoViewer says no viewer answers on the socket: none was started with
// that --socket, or it has quit.
var ErrNoViewer = errors.New("no gloss viewer is listening")

// Handler answers one request. It runs on its own goroutine, one per
// connection, and may take as long as the viewer takes.
type Handler func(Request) Response

const (
	// ioTimeout is how long a client has to write its request once
	// connected, and the server to write its answer once it has one.
	ioTimeout = 5 * time.Second
	// dialTimeout is how long a client waits for the connection.
	dialTimeout = time.Second
	// maxLine bounds a request or answer: 200 paths of 4 KiB fit.
	maxLine = 1 << 20
	// maxPath is the longest socket path every system accepts (macOS
	// stops at 104 bytes, including the terminator).
	maxPath = 103
)

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// SocketPath is where the socket called name goes: a name of letters,
// digits, dots, dashes and underscores is gloss-NAME.sock in the system's
// temporary folder, and anything with a path separator in it is used as the
// path itself.
func SocketPath(name string) (string, error) {
	if name == "" {
		return "", errors.New("the socket needs a name")
	}
	path := name
	if !strings.ContainsAny(name, `/\`) {
		if !validName.MatchString(name) {
			return "", fmt.Errorf("socket name %q: use letters, digits, dots, dashes and underscores, up to 64, or give a path", name)
		}
		path = filepath.Join(os.TempDir(), "gloss-"+name+".sock")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if len(path) > maxPath {
		return "", fmt.Errorf("socket path %q is longer than %d bytes", path, maxPath)
	}
	return path, nil
}

// Server is a viewer's end of the socket.
type Server struct {
	path     string
	listener net.Listener
	handle   Handler
}

// Listen opens the socket at path and answers requests with handle until
// Close. A socket file another viewer left behind is replaced; one a
// viewer still answers on is an error, as is any other file at the path.
func Listen(path string, handle Handler) (*Server, error) {
	if handle == nil {
		return nil, errors.New("remote: no handler")
	}
	if err := clearStale(path); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", path, err)
	}
	// Only this user may ask the viewer anything; the folder may be shared.
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, fmt.Errorf("protect %s: %w", path, err)
	}
	s := &Server{path: path, listener: listener, handle: handle}
	go s.accept()
	return s, nil
}

// clearStale removes a socket file nothing listens on, and refuses the path
// when something does, or when it is not a socket.
func clearStale(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%s exists and is not a socket", path)
	}
	if conn, err := net.DialTimeout("unix", path, dialTimeout); err == nil {
		conn.Close()
		return fmt.Errorf("a gloss viewer is already listening on %s", path)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("replace stale socket %s: %w", path, err)
	}
	return nil
}

// Path is where the server listens.
func (s *Server) Path() string { return s.path }

func (s *Server) accept() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // Closed.
		}
		go s.serve(conn)
	}
}

func (s *Server) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(ioTimeout))
	req, err := readLine[Request](conn)
	var resp Response
	switch {
	case err != nil:
		resp = Response{Error: "bad request: " + err.Error()}
	case req.Action == "":
		resp = Response{Error: "bad request: no action"}
	default:
		resp = s.handle(req)
	}
	_ = conn.SetWriteDeadline(time.Now().Add(ioTimeout))
	_ = writeLine(conn, resp)
}

// readLine reads one line of JSON as a T.
func readLine[T any](r io.Reader) (T, error) {
	var v T
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 4096), maxLine)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return v, err
		}
		return v, io.ErrUnexpectedEOF
	}
	err := json.Unmarshal(scanner.Bytes(), &v)
	return v, err
}

func writeLine(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

// Close stops answering and removes the socket file.
func (s *Server) Close() error {
	err := s.listener.Close()
	if rmErr := os.Remove(s.path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) && err == nil {
		err = rmErr
	}
	return err
}

// Send asks the viewer on the socket at path, and waits up to wait for its
// answer; a zero wait is a minute. An answer with ok false is not an error
// here: the caller reports it.
func Send(path string, req Request, wait time.Duration) (Response, error) {
	if wait <= 0 {
		wait = time.Minute
	}
	conn, err := net.DialTimeout("unix", path, dialTimeout)
	if err != nil {
		return Response{}, fmt.Errorf("%w on %s", ErrNoViewer, path)
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(ioTimeout))
	if err := writeLine(conn, req); err != nil {
		return Response{}, fmt.Errorf("send to %s: %w", path, err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(wait))
	resp, err := readLine[Response](conn)
	if err != nil {
		return Response{}, fmt.Errorf("answer from %s: %w", path, err)
	}
	return resp, nil
}
