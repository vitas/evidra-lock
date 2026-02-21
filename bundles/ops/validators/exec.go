package validators

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

var errToolMissing = errors.New("validator tool missing")

var execRunner = execCommand
var execRunnerWithEnv = execCommandWithEnv

func execCommand(ctx context.Context, workdir, name string, args []string, stdin []byte) (stdout, stderr []byte, exitCode int, durationMS int64, err error) {
	return execCommandWithEnv(ctx, workdir, name, args, stdin, nil)
}

func execCommandWithEnv(ctx context.Context, workdir, name string, args []string, stdin []byte, env map[string]string) (stdout, stderr []byte, exitCode int, durationMS int64, err error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), envMapToSlice(env)...)
	}
	cmd.Stdin = bytes.NewReader(stdin)

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	durationMS = time.Since(start).Milliseconds()
	stdout = outBuf.Bytes()
	stderr = errBuf.Bytes()

	if runErr == nil {
		return stdout, stderr, 0, durationMS, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return stdout, stderr, exitErr.ExitCode(), durationMS, nil
	}

	if errors.Is(runErr, exec.ErrNotFound) {
		return stdout, stderr, -1, durationMS, fmt.Errorf("%w: %s", errToolMissing, name)
	}
	var execErr *exec.Error
	if errors.As(runErr, &execErr) && execErr.Err == exec.ErrNotFound {
		return stdout, stderr, -1, durationMS, fmt.Errorf("%w: %s", errToolMissing, name)
	}

	return stdout, stderr, -1, durationMS, runErr
}

func envMapToSlice(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}

func toolMissingReport(tool, message string) Report {
	return Report{
		Tool:     tool,
		ExitCode: -1,
		Findings: []Finding{
			{
				Tool:     tool,
				Severity: SeverityLow,
				Title:    "tool-missing",
				Message:  message,
				RuleID:   "tool-missing",
			},
		},
	}
}
