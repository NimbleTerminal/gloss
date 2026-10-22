package clipimage

import (
	"context"
	"os/exec"
)

const supported = false

func command(context.Context, string) (*exec.Cmd, error) { return nil, ErrNoTool }
