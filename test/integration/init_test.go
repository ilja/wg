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
