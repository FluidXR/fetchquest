//go:build !windows

package rclone

import (
	"context"
	"os/exec"
)

func newCmd(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func newCmdContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
