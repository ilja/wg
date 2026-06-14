package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyIgnoredDryRunPlansCopiesWithoutWriting(t *testing.T) {
	bin := buildWG(t)
	repo, dest := setupCopyIgnoredRepo(t)

	stdout, stderr, code := runWGCommand(t, bin, repo, "copy-ignored", "--to", "feature-copy", "--dry-run")
	if code != 0 {
		t.Fatalf("wg copy-ignored --dry-run exited %d, stderr: %s", code, stderr)
	}
	if stdout != "copy .env\n" {
		t.Fatalf("unexpected dry-run stdout %q", stdout)
	}
	if !strings.Contains(stderr, "dry-run") || !strings.Contains(stderr, "1") {
		t.Fatalf("expected dry-run summary on stderr, got %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(dest, ".env")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write destination .env, stat err=%v", err)
	}
}

func TestCopyIgnoredCopiesIgnoredAllowlistedFilesOnly(t *testing.T) {
	bin := buildWG(t)
	repo, dest := setupCopyIgnoredRepo(t)

	stdout, stderr, code := runWGCommand(t, bin, repo, "copy-ignored", "--to", "feature-copy")
	if code != 0 {
		t.Fatalf("wg copy-ignored exited %d, stderr: %s", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("real copy should keep stdout empty, got %q", stdout)
	}
	if !strings.Contains(stderr, "copied 1") {
		t.Fatalf("expected copy summary on stderr, got %q", stderr)
	}
	assertFileContent(t, filepath.Join(dest, ".env"), "SECRET=primary\n")
	assertFileContent(t, filepath.Join(dest, "tracked-local.txt"), "committed\n")
}

func TestCopyIgnoredSkipAndForceExistingDestination(t *testing.T) {
	bin := buildWG(t)
	repo, dest := setupCopyIgnoredRepo(t)
	destEnv := filepath.Join(dest, ".env")
	mustWriteFile(t, destEnv, "SECRET=destination\n")

	stdout, stderr, code := runWGCommand(t, bin, repo, "copy-ignored", "--to", "feature-copy")
	if code != 0 {
		t.Fatalf("wg copy-ignored exited %d, stderr: %s", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("real copy should keep stdout empty, got %q", stdout)
	}
	if !strings.Contains(stderr, "skipped 1") {
		t.Fatalf("expected skip summary on stderr, got %q", stderr)
	}
	assertFileContent(t, destEnv, "SECRET=destination\n")

	stdout, stderr, code = runWGCommand(t, bin, repo, "copy-ignored", "--to", "feature-copy", "--force")
	if code != 0 {
		t.Fatalf("wg copy-ignored --force exited %d, stderr: %s", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("real copy should keep stdout empty, got %q", stdout)
	}
	assertFileContent(t, destEnv, "SECRET=primary\n")
}

func TestCopyIgnoredUsesWGWorktreePathDefaultDestination(t *testing.T) {
	bin := buildWG(t)
	repo, dest := setupCopyIgnoredRepo(t)

	stdout, stderr, code := runWGCommandWithEnv(t, bin, repo, []string{"WG_WORKTREE_PATH=" + dest}, "copy-ignored", "--from", "main")
	if code != 0 {
		t.Fatalf("wg copy-ignored exited %d, stderr: %s", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("real copy should keep stdout empty, got %q", stdout)
	}
	assertFileContent(t, filepath.Join(dest, ".env"), "SECRET=primary\n")
}

func TestCopyIgnoredMissingOrEmptyIncludeNoops(t *testing.T) {
	bin := buildWG(t)

	t.Run("missing", func(t *testing.T) {
		repo := initRepo(t)
		addWorktree(t, repo, "feature-copy", "demo.feature-copy")
		stdout, stderr, code := runWGCommand(t, bin, repo, "copy-ignored", "--to", "feature-copy")
		if code != 0 {
			t.Fatalf("wg copy-ignored exited %d, stderr: %s", code, stderr)
		}
		if stdout != "" {
			t.Fatalf("expected no-op stdout empty, got %q", stdout)
		}
		if !strings.Contains(strings.ToLower(stderr), "no .worktreeinclude") {
			t.Fatalf("expected clear no-op explanation, got %q", stderr)
		}
	})

	t.Run("empty", func(t *testing.T) {
		repo := initRepo(t)
		mustWriteFile(t, filepath.Join(repo, ".worktreeinclude"), "# comments only\n\n")
		runGit(t, repo, "add", ".worktreeinclude")
		runGit(t, repo, "commit", "-m", "add empty include")
		addWorktree(t, repo, "feature-copy", "demo.feature-copy")
		stdout, stderr, code := runWGCommand(t, bin, repo, "copy-ignored", "--to", "feature-copy")
		if code != 0 {
			t.Fatalf("wg copy-ignored exited %d, stderr: %s", code, stderr)
		}
		if stdout != "" {
			t.Fatalf("expected no-op stdout empty, got %q", stdout)
		}
		if !strings.Contains(strings.ToLower(stderr), "no .worktreeinclude patterns") {
			t.Fatalf("expected clear no-op explanation, got %q", stderr)
		}
	})
}

func TestCopyIgnoredBroadIncludeExcludesUnsafeAndNestedWorktrees(t *testing.T) {
	bin := buildWG(t)
	repo := initRepo(t)
	mustWriteFile(t, filepath.Join(repo, ".gitignore"), "*\n!.gitignore\n!.worktreeinclude\n!README.md\n")
	mustWriteFile(t, filepath.Join(repo, ".worktreeinclude"), "**\n")
	runGit(t, repo, "add", ".gitignore", ".worktreeinclude")
	runGit(t, repo, "commit", "-m", "add broad include")
	dest := addWorktree(t, repo, "feature-copy", "demo.feature-copy")
	mustWriteFile(t, filepath.Join(repo, ".env"), "SECRET=primary\n")
	for _, rel := range []string{
		".git/local-copy",
		".config/local.txt",
		".plans/local.txt",
		".wiki/local.txt",
		".pi/local.txt",
		".context/local.txt",
	} {
		mustMkdir(t, filepath.Dir(filepath.Join(repo, rel)))
		mustWriteFile(t, filepath.Join(repo, rel), "unsafe\n")
	}
	runGit(t, repo, "branch", "nested-copy")
	nested := filepath.Join(repo, "nested-worktree")
	runGit(t, repo, "worktree", "add", nested, "nested-copy")
	mustWriteFile(t, filepath.Join(nested, "nested-local.txt"), "nested\n")

	stdout, stderr, code := runWGCommand(t, bin, repo, "copy-ignored", "--to", "feature-copy")
	if code != 0 {
		t.Fatalf("wg copy-ignored exited %d, stderr: %s", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("real copy should keep stdout empty, got %q", stdout)
	}
	assertFileContent(t, filepath.Join(dest, ".env"), "SECRET=primary\n")
	for _, rel := range []string{
		".git/local-copy",
		".config/local.txt",
		".plans/local.txt",
		".wiki/local.txt",
		".pi/local.txt",
		".context/local.txt",
		"nested-worktree/nested-local.txt",
	} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err == nil {
			t.Fatalf("unsafe or nested path %s should not be copied", rel)
		}
	}
}

func setupCopyIgnoredRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := initRepo(t)
	mustWriteFile(t, filepath.Join(repo, ".gitignore"), ".env\n")
	mustWriteFile(t, filepath.Join(repo, ".worktreeinclude"), ".env\ntracked-local.txt\n")
	mustWriteFile(t, filepath.Join(repo, "tracked-local.txt"), "committed\n")
	runGit(t, repo, "add", ".gitignore", ".worktreeinclude", "tracked-local.txt")
	runGit(t, repo, "commit", "-m", "add copy include")
	mustWriteFile(t, filepath.Join(repo, ".env"), "SECRET=primary\n")
	dest := addWorktree(t, repo, "feature-copy", "demo.feature-copy")
	mustWriteFile(t, filepath.Join(repo, "tracked-local.txt"), "primary local change\n")
	return repo, dest
}

func runWGCommandWithEnv(t *testing.T, bin, dir string, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	}
	t.Fatalf("failed to run wg %v: %v", args, err)
	return "", "", 1
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s content mismatch: want %q got %q", path, want, content)
	}
}
