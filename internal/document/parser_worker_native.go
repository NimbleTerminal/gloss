//go:build !js

package document

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

var parserSlots = make(chan struct{}, 2)
var parserCommand = func(ctx context.Context, exe string) *exec.Cmd { return exec.CommandContext(ctx, exe, parserWorkerArg) }

func init() {
	if len(os.Args) == 2 && os.Args[1] == parserWorkerArg {
		if err := serveParser(os.Stdin, os.Stdout); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
}

type parserOutput struct{ bytes.Buffer }

func (b *parserOutput) Write(p []byte) (int, error) {
	if len(p) > maxParserMessage-b.Len() {
		return 0, errors.New("parser output exceeds limit")
	}
	return b.Buffer.Write(p)
}

func runParser(q parserRequest) (parserReply, error) {
	ctx, cancel := context.WithTimeout(context.Background(), parseDeadline)
	defer cancel()
	select {
	case parserSlots <- struct{}{}:
		defer func() { <-parserSlots }()
	case <-ctx.Done():
		return parserReply{}, fmt.Errorf("parser busy: %w", ctx.Err())
	}
	exe, err := os.Executable()
	if err != nil {
		return parserReply{}, err
	}
	input, err := json.Marshal(q)
	if err != nil {
		return parserReply{}, err
	}
	cmd := parserCommand(ctx, exe)
	cmd.Stdin = bytes.NewReader(input)
	var output parserOutput
	cmd.Stdout = &output
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return parserReply{}, fmt.Errorf("%s parsing timed out; worker stopped: %w", q.Kind, ctx.Err())
		}
		return parserReply{}, fmt.Errorf("%s parser failed: %w", q.Kind, err)
	}
	var out parserReply
	if err := json.Unmarshal(output.Bytes(), &out); err != nil {
		return out, fmt.Errorf("invalid parser reply: %w", err)
	}
	if out.Error != "" {
		return out, errors.New(out.Error)
	}
	return out, nil
}
