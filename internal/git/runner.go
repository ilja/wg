package git

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner interface {
	Run(ctx context.Context, dir string, args ...string) (Result, error)
}

type ExecRunner struct {
	Binary string
}

func (r ExecRunner) Run(ctx context.Context, dir string, args ...string) (Result, error) {
	binary := r.Binary
	if binary == "" {
		binary = "git"
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}
	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	result.ExitCode = 1
	return result, err
}
