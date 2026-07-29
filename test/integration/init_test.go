package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitFromLinkedWorktreeInstallsPrimaryHook(t *testing.T) {
	bin := buildWG(t)
	repo := initRepo(t)
	linked := addWorktree(t, repo, "feature-init", "demo.feature-init")
	xdg := filepath.Join(t.TempDir(), "xdg")
	mustMkdir(t, filepath.Join(xdg, "wg"))
	content := "#!/bin/sh\nprintf 'initialized\\n'\n"
	template := filepath.Join(xdg, "wg", "setup.sh")
	mustWriteFile(t, template, content)
	if err := os.Chmod(template, 0o640); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)

	stdout, stderr, code := runWGCommand(t, bin, linked, "init")
	if code != 0 {
		t.Fatalf("wg init exited %d, stderr: %s", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	hook := filepath.Join(repo, ".config", "setup.sh")
	assertFileContent(t, hook, content)
	info, err := os.Stat(hook)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o740); got != want {
		t.Fatalf("hook mode: want %o, got %o", want, got)
	}
	if _, err := os.Stat(filepath.Join(linked, ".config", "setup.sh")); !os.IsNotExist(err) {
		t.Fatalf("expected no linked-worktree hook, stat err: %v", err)
	}
	for _, want := range []string{hook, "Tailor it before running wg new", "wg copy-ignored", "direnv"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
		}
	}
	if ignored := strings.TrimSpace(runGit(t, repo, "check-ignore", "-q", ".config/setup.sh")); ignored != "" {
		t.Fatalf("expected quiet check-ignore output, got %q", ignored)
	}
	assertCleanStatus(t, repo)
}

func TestInitRequiresForceToReplaceExistingHook(t *testing.T) {
	bin := buildWG(t)
	repo := initRepo(t)
	xdg := filepath.Join(t.TempDir(), "xdg")
	mustMkdir(t, filepath.Join(xdg, "wg"))
	template := filepath.Join(xdg, "wg", "setup.sh")
	mustWriteFile(t, template, "initial\n")
	t.Setenv("XDG_CONFIG_HOME", xdg)

	stdout, stderr, code := runWGCommand(t, bin, repo, "init")
	if code != 0 {
		t.Fatalf("initial wg init exited %d, stdout: %s stderr: %s", code, stdout, stderr)
	}
	hook := filepath.Join(repo, ".config", "setup.sh")
	excludePath := filepath.Join(repo, ".git", "info", "exclude")
	excludeBefore, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, hook, "hand edited\n")
	mustWriteFile(t, template, "replacement\n")

	stdout, stderr, code = runWGCommand(t, bin, repo, "init")
	if code == 0 {
		t.Fatal("expected repeated init to fail without --force")
	}
	if stdout != "" {
		t.Fatalf("expected empty refusal stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--force") {
		t.Fatalf("expected force hint, got %q", stderr)
	}
	assertFileContent(t, hook, "hand edited\n")
	excludeAfter, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(excludeAfter) != string(excludeBefore) {
		t.Fatalf("exclude changed on refusal: before %q, after %q", excludeBefore, excludeAfter)
	}

	stdout, stderr, code = runWGCommand(t, bin, repo, "init", "--force")
	if code != 0 {
		t.Fatalf("forced wg init exited %d, stderr: %s", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty forced stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, hook) || !strings.Contains(stdout, "Tailor it") {
		t.Fatalf("expected forced replacement guidance, got %q", stdout)
	}
	assertFileContent(t, hook, "replacement\n")
	assertCleanStatus(t, repo)
}
