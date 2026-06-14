package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
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
	cmd := exec.CommandContext(ctx, r.binary(), args...)
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

func (r ExecRunner) binary() string {
	if r.Binary == "" {
		return "git"
	}
	return r.Binary
}

func (r ExecRunner) RunStreaming(ctx context.Context, dir string, stdout io.Writer, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, r.binary(), args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return commandError(cmd.Run(), args...)
}

func (r ExecRunner) RunWithInput(ctx context.Context, dir string, stdin io.Reader, stdout io.Writer, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, r.binary(), args...)
	cmd.Dir = dir
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return commandError(cmd.Run(), args...)
}

func RunOK(ctx context.Context, runner Runner, dir string, args ...string) error {
	result, err := runner.Run(ctx, dir, args...)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(result.Stderr))
	}
	return nil
}

func Output(ctx context.Context, runner Runner, dir string, args ...string) (string, error) {
	result, err := runner.Run(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(result.Stderr))
	}
	return result.Stdout, nil
}

func RunStreaming(ctx context.Context, runner Runner, dir string, stdout io.Writer, stderr io.Writer, args ...string) error {
	if streaming, ok := runner.(interface {
		RunStreaming(context.Context, string, io.Writer, io.Writer, ...string) error
	}); ok {
		return streaming.RunStreaming(ctx, dir, stdout, stderr, args...)
	}
	result, err := runner.Run(ctx, dir, args...)
	if result.Stdout != "" && stdout != nil {
		_, _ = io.WriteString(stdout, result.Stdout)
	}
	if result.Stderr != "" && stderr != nil {
		_, _ = io.WriteString(stderr, result.Stderr)
	}
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git %s failed", strings.Join(args, " "))
	}
	return nil
}

func RunWithInput(ctx context.Context, runner Runner, dir string, stdin io.Reader, stdout io.Writer, stderr io.Writer, args ...string) error {
	if inputRunner, ok := runner.(interface {
		RunWithInput(context.Context, string, io.Reader, io.Writer, io.Writer, ...string) error
	}); ok {
		return inputRunner.RunWithInput(ctx, dir, stdin, stdout, stderr, args...)
	}
	return fmt.Errorf("git %s requires stdin support unavailable for this runner", strings.Join(args, " "))
}

func commandError(err error, args ...string) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("git %s failed", strings.Join(args, " "))
	}
	return err
}

func (r ExecRunner) TrackedPaths(ctx context.Context, root string, rels []string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	if len(rels) == 0 {
		return out, nil
	}
	args := append([]string{"ls-files", "-z", "--cached", "--"}, rels...)
	result, err := r.Run(ctx, root, args...)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git ls-files failed: %s", strings.TrimSpace(result.Stderr))
	}
	return parseNULPaths(result.Stdout), nil
}

func (r ExecRunner) IgnoredPaths(ctx context.Context, root string, rels []string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	if len(rels) == 0 {
		return out, nil
	}

	cmd := exec.CommandContext(ctx, r.binary(), "check-ignore", "-z", "--stdin")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(strings.Join(rels, "\x00") + "\x00")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return parseNULPaths(stdout.String()), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ExitCode() == 1 {
			return out, nil
		}
		return nil, fmt.Errorf("git check-ignore failed: %s", strings.TrimSpace(stderr.String()))
	}
	return nil, err
}

func (r ExecRunner) NestedWorktreePaths(ctx context.Context, root string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	root = filepath.Clean(root)
	result, err := r.Run(ctx, root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git worktree list --porcelain failed: %s", strings.TrimSpace(result.Stderr))
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		pathValue, ok := strings.CutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		worktreePath := filepath.Clean(pathValue)
		rel, err := filepath.Rel(root, worktreePath)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			continue
		}
		out[worktreePath] = struct{}{}
	}
	return out, nil
}

func parseNULPaths(output string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, value := range strings.Split(output, "\x00") {
		if value == "" {
			continue
		}
		out[filepath.ToSlash(value)] = struct{}{}
	}
	return out
}
