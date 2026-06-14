package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestNewExplicitBaseCreatesSanitizedSiblingAndRunsSetup(t *testing.T) {
	bin := buildWG(t)
	repo := initRepo(t)
	base := commitFile(t, repo, "base.txt", "base\n", "base commit")
	runGit(t, repo, "branch", "setup-base", base)

	capture := filepath.Join(filepath.Dir(repo), "setup.env")
	installSetupScript(t, repo, strings.Join([]string{
		"#!/bin/sh",
		"echo setup stdout from script",
		"{",
		"  printf 'PWD=%s\\n' \"$PWD\"",
		"  env | grep '^WG_' | sort",
		"} > " + shellQuote(capture),
		"",
	}, "\n"))

	branch := "feature/sc-123/add-temp"
	name := "feature-sc-123-add-temp"
	wantPath := filepath.Join(filepath.Dir(repo), "demo."+name)
	stdout, stderr, code := runWGCommand(t, bin, repo, "new", branch, "setup-base")
	if code != 0 {
		t.Fatalf("wg new exited %d, stderr: %s", code, stderr)
	}
	if stdout != wantPath+"\n" {
		t.Fatalf("expected path-only stdout %q, got %q", wantPath+"\n", stdout)
	}
	if !strings.Contains(stderr, "setup stdout from script") {
		t.Fatalf("expected setup stdout routed to stderr, got %q", stderr)
	}
	if got := strings.TrimSpace(runGit(t, repo, "rev-parse", branch)); got != base {
		t.Fatalf("expected branch tip %s to match base %s", got, base)
	}

	values := parseCapturedSetup(t, capture)
	want := map[string]string{
		"PWD":                      wantPath,
		"WG_BRANCH":                branch,
		"WG_WORKTREE_PATH":         wantPath,
		"WG_WORKTREE_NAME":         name,
		"WG_REPO":                  "demo",
		"WG_REPO_PATH":             repo,
		"WG_PRIMARY_WORKTREE_PATH": repo,
		"WG_DEFAULT_BRANCH":        "setup-base",
		"WG_BASE":                  "setup-base",
	}
	for key, wantValue := range want {
		if values[key] != wantValue {
			t.Fatalf("expected %s=%q, got %q in %#v", key, wantValue, values[key], values)
		}
	}
	if port := values["WG_PORT"]; port == "" {
		t.Fatalf("expected WG_PORT to be present in %#v", values)
	}
}

func TestNewResolvesDefaultBaseSuccessCases(t *testing.T) {
	bin := buildWG(t)

	t.Run("primary main", func(t *testing.T) {
		repo := initRepo(t)
		mainTip := strings.TrimSpace(runGit(t, repo, "rev-parse", "main"))
		stdout, stderr, code := runWGCommand(t, bin, repo, "new", "feature/default-main")
		if code != 0 {
			t.Fatalf("wg new exited %d, stderr: %s", code, stderr)
		}
		path := strings.TrimSpace(stdout)
		if got := strings.TrimSpace(runGit(t, repo, "rev-parse", "feature/default-main")); got != mainTip {
			t.Fatalf("expected new branch tip %s to match main %s", got, mainTip)
		}
		if path != filepath.Join(filepath.Dir(repo), "demo.feature-default-main") {
			t.Fatalf("unexpected stdout path %q", stdout)
		}
	})

	t.Run("origin head", func(t *testing.T) {
		repo := initRepoOnBranch(t, "develop")
		originTip := commitFile(t, repo, "origin.txt", "origin\n", "origin commit")
		runGit(t, repo, "branch", "main", originTip)
		runGit(t, repo, "update-ref", "refs/remotes/origin/main", originTip)
		runGit(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

		_, stderr, code := runWGCommand(t, bin, repo, "new", "feature/origin-default")
		if code != 0 {
			t.Fatalf("wg new exited %d, stderr: %s", code, stderr)
		}
		if got := strings.TrimSpace(runGit(t, repo, "rev-parse", "feature/origin-default")); got != originTip {
			t.Fatalf("expected new branch tip %s to match origin HEAD %s", got, originTip)
		}
	})

	t.Run("unambiguous local master", func(t *testing.T) {
		repo := initRepoOnBranch(t, "develop")
		masterTip := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
		runGit(t, repo, "branch", "master", masterTip)

		_, stderr, code := runWGCommand(t, bin, repo, "new", "feature/local-master")
		if code != 0 {
			t.Fatalf("wg new exited %d, stderr: %s", code, stderr)
		}
		if got := strings.TrimSpace(runGit(t, repo, "rev-parse", "feature/local-master")); got != masterTip {
			t.Fatalf("expected new branch tip %s to match master %s", got, masterTip)
		}
	})
}

func TestNewRefusesAmbiguousDefaultBeforeMutation(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoOnBranch(t, "develop")
	runGit(t, repo, "branch", "main")
	runGit(t, repo, "branch", "master")
	refsBefore := snapshotRefs(t, repo)
	worktreesBefore := snapshotWorktrees(t, repo)

	stdout, stderr, code := runWGCommand(t, bin, repo, "new", "feature/ambiguous-default")
	if code == 0 {
		t.Fatalf("expected ambiguous default to fail")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "ambiguous") || !strings.Contains(stderr, "main") || !strings.Contains(stderr, "master") {
		t.Fatalf("expected ambiguous default diagnostic, got %q", stderr)
	}
	assertNoMutation(t, repo, refsBefore, worktreesBefore)
}

func TestNewRefusesCollisionsBeforeMutation(t *testing.T) {
	bin := buildWG(t)

	t.Run("existing local branch", func(t *testing.T) {
		repo := initRepo(t)
		runGit(t, repo, "branch", "feature/collision")
		assertNewRefusalBeforeMutation(t, bin, repo, []string{"new", "feature/collision", "main"}, "branch")
	})

	t.Run("existing sanitized worktree name", func(t *testing.T) {
		repo := initRepo(t)
		addWorktree(t, repo, "other-branch", "demo.feature-sc-123-add-temp")
		assertNewRefusalBeforeMutation(t, bin, repo, []string{"new", "feature/sc-123/add-temp", "main"}, "worktree")
	})

	t.Run("existing display name sanitizes to target name", func(t *testing.T) {
		repo := initRepo(t)
		addWorktreeAtPath(t, repo, "feature/foo", "custom-existing")
		assertNewRefusalBeforeMutation(t, bin, repo, []string{"new", "feature-foo", "main"}, "worktree")
	})

	t.Run("existing empty derived path", func(t *testing.T) {
		repo := initRepo(t)
		mustMkdir(t, filepath.Join(filepath.Dir(repo), "demo.feature-empty-path"))
		assertNewRefusalBeforeMutation(t, bin, repo, []string{"new", "feature/empty-path", "main"}, "path")
	})

	t.Run("existing non-empty target location", func(t *testing.T) {
		repo := initRepo(t)
		path := filepath.Join(filepath.Dir(repo), "demo.feature-non-empty")
		mustMkdir(t, path)
		mustWriteFile(t, filepath.Join(path, "local.txt"), "do not clobber\n")
		assertNewRefusalBeforeMutation(t, bin, repo, []string{"new", "feature/non-empty", "main"}, "non-empty")
	})
}

func TestNewSetupFailureReportsAndLeavesWorktreeAvailable(t *testing.T) {
	bin := buildWG(t)
	repo := initRepo(t)
	installSetupScript(t, repo, strings.Join([]string{
		"#!/bin/sh",
		"echo setup failed >&2",
		"exit 7",
		"",
	}, "\n"))

	branch := "feature/setup-fails"
	wantPath := filepath.Join(filepath.Dir(repo), "demo.feature-setup-fails")
	stdout, stderr, code := runWGCommand(t, bin, repo, "new", branch, "main")
	if code == 0 {
		t.Fatalf("expected setup failure")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on setup failure, got %q", stdout)
	}
	for _, want := range []string{".config/setup.sh", "exit code 7", branch, wantPath, "setup failed"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
	if got := strings.TrimSpace(runGit(t, repo, "rev-parse", "--verify", branch)); got == "" {
		t.Fatalf("expected branch %s to remain available", branch)
	}
	if !strings.Contains(snapshotWorktrees(t, repo), wantPath) {
		t.Fatalf("expected worktree %s to remain available", wantPath)
	}
}

func runWGCommand(t *testing.T, bin, dir string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
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

func initRepoOnBranch(t *testing.T, branch string) string {
	t.Helper()
	parent := t.TempDir()
	repo := filepath.Join(parent, "demo")
	mustMkdir(t, repo)
	runGit(t, parent, "init", "-b", branch, repo)
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWriteFile(t, filepath.Join(repo, "README.md"), "hello\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	return mustCanonicalPath(t, repo)
}

func commitFile(t *testing.T, repo, name, content, message string) string {
	t.Helper()
	mustWriteFile(t, filepath.Join(repo, name), content)
	runGit(t, repo, "add", name)
	runGit(t, repo, "commit", "-m", message)
	return strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
}

func installSetupScript(t *testing.T, repo, content string) {
	t.Helper()
	path := filepath.Join(repo, ".config", "setup.sh")
	mustMkdir(t, filepath.Dir(path))
	mustWriteFile(t, path, content)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func addWorktreeAtPath(t *testing.T, repo, branch, sibling string) string {
	t.Helper()
	runGit(t, repo, "branch", branch)
	path := filepath.Join(filepath.Dir(repo), sibling)
	runGit(t, repo, "worktree", "add", path, branch)
	return mustCanonicalPath(t, path)
}

func parseCapturedSetup(t *testing.T, path string) map[string]string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid capture line %q in %s", line, content)
		}
		values[key] = value
	}
	return values
}

func snapshotRefs(t *testing.T, repo string) []string {
	t.Helper()
	output := strings.TrimSpace(runGit(t, repo, "show-ref", "--heads"))
	if output == "" {
		return nil
	}
	refs := strings.Split(output, "\n")
	sort.Strings(refs)
	return refs
}

func snapshotWorktrees(t *testing.T, repo string) string {
	t.Helper()
	return runGit(t, repo, "worktree", "list", "--porcelain")
}

func assertNoMutation(t *testing.T, repo string, refsBefore []string, worktreesBefore string) {
	t.Helper()
	if refsAfter := snapshotRefs(t, repo); !reflect.DeepEqual(refsAfter, refsBefore) {
		t.Fatalf("refs mutated\nbefore: %#v\nafter:  %#v", refsBefore, refsAfter)
	}
	if worktreesAfter := snapshotWorktrees(t, repo); worktreesAfter != worktreesBefore {
		t.Fatalf("worktrees mutated\nbefore:\n%s\nafter:\n%s", worktreesBefore, worktreesAfter)
	}
}

func assertNewRefusalBeforeMutation(t *testing.T, bin, repo string, args []string, diagnostic string) {
	t.Helper()
	refsBefore := snapshotRefs(t, repo)
	worktreesBefore := snapshotWorktrees(t, repo)
	stdout, stderr, code := runWGCommand(t, bin, repo, args...)
	if code == 0 {
		t.Fatalf("expected wg %v to fail", args)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), strings.ToLower(diagnostic)) {
		t.Fatalf("expected stderr to contain %q, got %q", diagnostic, stderr)
	}
	assertNoMutation(t, repo, refsBefore, worktreesBefore)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
