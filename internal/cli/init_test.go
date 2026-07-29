package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wg/internal/app"
	"wg/internal/git"
)

type initCLIRunner struct {
	primary string
}

func (r initCLIRunner) Run(_ context.Context, _ string, args ...string) (git.Result, error) {
	switch strings.Join(args, " ") {
	case "rev-parse --show-toplevel":
		return git.Result{Stdout: r.primary + "\n"}, nil
	case "worktree list --porcelain":
		return git.Result{Stdout: "worktree " + r.primary + "\nHEAD abc\nbranch refs/heads/main\n\n"}, nil
	case "rev-parse --git-path info/exclude":
		return git.Result{Stdout: filepath.Join(r.primary, ".git", "info", "exclude") + "\n"}, nil
	default:
		return git.Result{ExitCode: 1, Stderr: "unexpected command"}, nil
	}
}

func TestInitCommandPrintsGuidance(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(primary, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	xdg := filepath.Join(root, "xdg")
	if err := os.MkdirAll(filepath.Join(xdg, "wg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "wg", "setup.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"init"}, Options{
		Cwd: primary, Stdout: &stdout, Stderr: &stderr,
		Environ: []string{"XDG_CONFIG_HOME=" + xdg}, GitRunner: initCLIRunner{primary: primary},
	})
	if code != 0 {
		t.Fatalf("expected success, code=%d stderr=%q", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
	for _, want := range []string{filepath.Join(primary, ".config", "setup.sh"), "Tailor it before running wg new", ".worktreeinclude", "wg copy-ignored", "local environment files", "project dependencies", "Symlink resources", "direnv"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, stdout.String())
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRenderInitResultPropagatesWriteError(t *testing.T) {
	err := renderInitResult(failingWriter{}, app.InitResult{HookPath: "/repo/.config/setup.sh"})
	if err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected write error, got %v", err)
	}
}
