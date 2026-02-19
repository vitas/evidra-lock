package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}

type Runner struct {
	timeout time.Duration
}

func NewRunner(timeout time.Duration) *Runner {
	return &Runner{timeout: timeout}
}

func (r *Runner) Execute(ctx context.Context, command string, args []string) (Result, error) {
	runCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, command, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if runCtx.Err() == context.DeadlineExceeded {
		result.ExitCode = -1
		result.TimedOut = true
		return result, nil
	}

	if err == nil {
		result.ExitCode = 0
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return Result{}, fmt.Errorf("execute command: %w", err)
}
