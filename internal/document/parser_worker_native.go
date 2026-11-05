//go:build !js

package document

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"nimbleterminal.dev/ntmediabox"
)

var parserSlots = make(chan struct{}, 2)
var parserCommand = defaultParserCommand
var inParserWorker bool

func defaultParserCommand(exe string, required bool) (*exec.Cmd, error) {
	cmd, _, err := ntmediabox.Command(ntmediabox.HelperConfig{
		WorkerPath: exe, WorkerArgs: []string{parserWorkerArg},
		Policy: ntmediabox.Policy{ReadPaths: ntmediabox.DefaultLibPaths(), ExecPaths: []string{exe}, AllowUnsandboxed: !required},
	})
	return cmd, err // A policy or stage error never triggers a direct-exec retry.
}

func init() {
	if len(os.Args) == 2 && os.Args[1] == parserWorkerArg {
		inParserWorker = true
		if err := ntmediabox.ServeRender(struct {
			io.Reader
			io.Writer
		}{os.Stdin, os.Stdout}, &parserRenderServer{}); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
}

type processWorker struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    io.ReadCloser
	client *ntmediabox.RenderClient
	once   sync.Once
}
type parserPipe struct {
	io.Reader
	io.Writer
	closeFn func()
}

func (p parserPipe) Close() error { p.closeFn(); return nil }

func dialParser() (parserConn, error) { return dialParserPolicy(false) }
func dialParserPolicy(required bool) (parserConn, error) {
	if inParserWorker {
		return &localParser{}, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd, err := parserCommand(exe, required)
	if err != nil {
		return nil, err
	}
	// Command may attach a Linux stage-spec pipe. The child receives its own
	// descriptors at Start; the parent owns closing every attached original,
	// including on pipe setup or launch failure. No caller files are passed here.
	defer func() {
		for _, f := range cmd.ExtraFiles {
			_ = f.Close()
		}
	}()
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, err
	}
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		in.Close()
		out.Close()
		return nil, err
	}
	w := &processWorker{cmd: cmd, in: in, out: out}
	w.client = ntmediabox.NewRenderClientWithKill(parserPipe{out, in, func() { in.Close(); out.Close() }}, w.kill)
	return w, nil
}

// Bound the queue separately from the protocol client's serialized requests.
func (w *processWorker) ask(ctx context.Context, q parserRequest) (parserReply, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, parseDeadline)
	defer cancel()
	select {
	case parserSlots <- struct{}{}:
		defer func() { <-parserSlots }()
	case <-ctx.Done():
		return parserReply{}, fmt.Errorf("parser busy: %w", ctx.Err())
	}
	data := q.Data
	q.Data = nil
	meta, err := json.Marshal(q)
	if err != nil {
		return parserReply{}, err
	}
	resp, err := w.client.Do(ctx, ntmediabox.RenderRequest{Kind: "gloss-parser", Data: data, Meta: meta, Width: ntmediabox.MaxRenderSide, Height: ntmediabox.MaxRenderSide, MaxPixels: MaxPixels})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return parserReply{}, fmt.Errorf("%s parsing timed out; worker stopped: %w", q.Kind, err)
		}
		return parserReply{}, fmt.Errorf("%s parsing stopped: %w", q.Kind, err)
	}
	reply, err := decodeParserResponse(resp)
	if err != nil {
		w.close()
		return parserReply{}, err
	}
	if reply.Error != "" {
		return reply, errors.New(reply.Error)
	}
	return reply, nil
}
func (w *processWorker) kill() {
	w.once.Do(func() {
		w.in.Close()
		w.out.Close()
		ntmediabox.KillHelper(w.cmd.Process.Pid)
		_ = w.cmd.Process.Kill()
		_ = w.cmd.Wait()
	})
}
func (w *processWorker) close() { _ = w.client.Close() }
