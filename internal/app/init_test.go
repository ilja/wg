package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"wg/internal/git"
)

type initGitCall struct {
	dir  string
	args []string
}

type initGitRunner struct {
	current string
	primary string
	calls   []initGitCall
}

func (r *initGitRunner) Run(_ context.Context, dir string, args ...string) (git.Result, error) {
	r.calls = append(r.calls, initGitCall{dir: dir, args: append([]string(nil), args...)})
	switch strings.Join(args, " ") {
	case "rev-parse --show-toplevel":
		return git.Result{Stdout: r.current + "\n"}, nil
	case "worktree list --porcelain":
		return git.Result{Stdout: "worktree " + r.primary + "\nHEAD abc\nbranch refs/heads/main\n\nworktree " + r.current + "\nHEAD def\nbranch refs/heads/feature\n\n"}, nil
	case "rev-parse --git-path info/exclude":
		return git.Result{Stdout: ".git/info/exclude\n"}, nil
	default:
		return git.Result{ExitCode: 1, Stderr: "unexpected command"}, nil
	}
}

func TestInitTargetsPrimaryAfterRepositoryDiscovery(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "primary")
	current := filepath.Join(root, "linked")
	if err := os.MkdirAll(filepath.Join(primary, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	templateRoot := filepath.Join(root, "xdg")
	if err := os.MkdirAll(filepath.Join(templateRoot, "wg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateRoot, "wg", "setup.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &initGitRunner{current: current, primary: primary}

	result, err := (&App{Cwd: filepath.Join(current, "subdir"), Environ: []string{"XDG_CONFIG_HOME=" + templateRoot}, GitRunner: runner}).Init(context.Background(), InitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantHook := filepath.Join(primary, ".config", "setup.sh")
	if result.HookPath != wantHook {
		t.Fatalf("want hook %q, got %q", wantHook, result.HookPath)
	}
	wantArgs := [][]string{{"rev-parse", "--show-toplevel"}, {"worktree", "list", "--porcelain"}, {"rev-parse", "--git-path", "info/exclude"}}
	gotArgs := make([][]string, 0, len(runner.calls))
	for _, call := range runner.calls {
		gotArgs = append(gotArgs, call.args)
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("git call order mismatch: want %#v, got %#v", wantArgs, gotArgs)
	}
	if runner.calls[2].dir != primary {
		t.Fatalf("exclude lookup ran in %q, want %q", runner.calls[2].dir, primary)
	}
	exclude, err := os.ReadFile(filepath.Join(primary, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if string(exclude) != "/.config/\n" {
		t.Fatalf("unexpected exclude content %q", exclude)
	}
}

func TestInitRefusesExistingHookBeforeExcludeLookup(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "primary")
	templateRoot := filepath.Join(root, "xdg")
	if err := os.MkdirAll(filepath.Join(templateRoot, "wg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateRoot, "wg", "setup.sh"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(primary, ".config", "setup.sh")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, []byte("existing\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &initGitRunner{current: primary, primary: primary}

	_, err := (&App{Cwd: primary, Environ: []string{"XDG_CONFIG_HOME=" + templateRoot}, GitRunner: runner}).Init(context.Background(), InitOptions{})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected force hint, got %v", err)
	}
	if got, want := len(runner.calls), 2; got != want {
		t.Fatalf("Git call count: want %d, got %d", want, got)
	}
	content, readErr := os.ReadFile(hook)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "existing\n" {
		t.Fatalf("existing hook changed: %q", content)
	}
}

func TestInitForceReplacesPrimaryHook(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "primary")
	current := filepath.Join(root, "linked")
	if err := os.MkdirAll(filepath.Join(primary, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	templateRoot := filepath.Join(root, "xdg")
	if err := os.MkdirAll(filepath.Join(templateRoot, "wg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateRoot, "wg", "setup.sh"), []byte("replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(primary, ".config", "setup.sh")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, []byte("existing\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &initGitRunner{current: current, primary: primary}

	result, err := (&App{Cwd: current, Environ: []string{"XDG_CONFIG_HOME=" + templateRoot}, GitRunner: runner}).Init(context.Background(), InitOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.HookPath != hook {
		t.Fatalf("want hook %q, got %q", hook, result.HookPath)
	}
	content, err := os.ReadFile(hook)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "replacement\n" {
		t.Fatalf("unexpected hook content %q", content)
	}
	if runner.calls[2].dir != primary {
		t.Fatalf("exclude lookup ran in %q, want %q", runner.calls[2].dir, primary)
	}
}
