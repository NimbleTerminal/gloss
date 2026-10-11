//go:build !js

package document

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestParserTimeoutKillsAndReapsWorker(t *testing.T) {
	oldCommand, oldDeadline := parserCommand, parseDeadline
	t.Cleanup(func() { parserCommand = oldCommand; parseDeadline = oldDeadline })
	parseDeadline = 200 * time.Millisecond
	var child *exec.Cmd
	parserCommand = func(ctx context.Context, exe string) *exec.Cmd {
		child = exec.CommandContext(ctx, exe, "-test.run=^TestParserHangHelper$")
		child.Env = append(os.Environ(), "GLOSS_TEST_HANG=1")
		return child
	}
	_, err := runParser(parserRequest{Kind: "svg"})
	if err == nil || !strings.Contains(err.Error(), "worker stopped") {
		t.Fatalf("timeout: %v", err)
	}
	if child == nil || child.Process == nil || child.ProcessState == nil {
		t.Fatal("worker was not started and reaped")
	}
	if err := child.Process.Kill(); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("worker still alive: %v", err)
	}
	if len(parserSlots) != 0 {
		t.Fatal("worker retained its slot")
	}
	// A later request can succeed; timeouts do not poison the worker pool.
	parserCommand = oldCommand
	parseDeadline = oldDeadline
	out, err := runParser(parserRequest{Kind: "svg", Name: "test.svg", Edge: 16, Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10"/></svg>`)})
	if err != nil || len(out.PNG) == 0 {
		t.Fatalf("recovery: %v", err)
	}
}
func TestParserHangHelper(t *testing.T) {
	if os.Getenv("GLOSS_TEST_HANG") != "1" {
		return
	}
	for {
		time.Sleep(time.Hour)
	}
}
