//go:build !js

package document

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

var parserSlots = make(chan struct{}, 2)
var parserCommand = func(exe string) *exec.Cmd { return exec.Command(exe, parserWorkerArg) }

func init() {
	if len(os.Args) == 2 && os.Args[1] == parserWorkerArg {
		if err := serveParser(os.Stdin, os.Stdout); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
}

// A native worker is this executable in a child process, talking over its
// standard input and output.
type processWorker struct {
	cmd  *exec.Cmd
	in   io.WriteCloser
	out  *bufio.Reader
	once sync.Once
}

func dialParser() (parserConn, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := parserCommand(exe)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &processWorker{cmd: cmd, in: in, out: bufio.NewReader(out)}, nil
}

// ask takes one of the two parser slots for the question only: an open
// document's idle worker does not hold one.
func (w *processWorker) ask(ctx context.Context, q parserRequest) (parserReply, error) {
	ctx, cancel := context.WithTimeout(ctx, parseDeadline)
	defer cancel()
	select {
	case parserSlots <- struct{}{}:
		defer func() { <-parserSlots }()
	case <-ctx.Done():
		return parserReply{}, fmt.Errorf("parser busy: %w", ctx.Err())
	}
	type answer struct {
		reply parserReply
		err   error
	}
	done := make(chan answer, 1)
	go func() {
		var a answer
		if a.err = writeMessage(w.in, q); a.err == nil {
			a.err = readMessage(w.out, &a.reply)
		}
		done <- a
	}()
	var a answer
	select {
	case a = <-done:
	case <-ctx.Done():
		// Killed, its pipes end, and the reader returns before Wait closes them.
		_ = w.cmd.Process.Kill()
		<-done
		w.close()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return parserReply{}, fmt.Errorf("%s parsing timed out; worker stopped: %w", q.Kind, ctx.Err())
		}
		return parserReply{}, fmt.Errorf("%s parsing stopped: %w", q.Kind, ctx.Err())
	}
	if a.err != nil {
		w.close()
		return parserReply{}, fmt.Errorf("%s parser failed: %w", q.Kind, a.err)
	}
	if a.reply.Error != "" {
		return a.reply, errors.New(a.reply.Error)
	}
	return a.reply, nil
}

// close ends the worker: its input closes, which ends a well-behaved one,
// and it is killed and reaped all the same.
func (w *processWorker) close() {
	w.once.Do(func() {
		_ = w.in.Close()
		_ = w.cmd.Process.Kill()
		_ = w.cmd.Wait()
	})
}
