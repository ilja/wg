package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestReadOnlyCommands(t *testing.T) {
	bin := buildWG(t)
	repo := initRepo(t)

	alphaPath := addWorktree(t, repo, "feature-alpha", "demo.feature-alpha")
	alpinePath := addWorktree(t, repo, "feature-alpine", "demo.feature-alpine")
	detachedPath := addDetachedWorktree(t, repo, "demo.detached")

	nested := filepath.Join(alphaPath, "nested", "dir")
	mustMkdir(t, nested)

	t.Run("list from nested linked worktree is fast and read-only shaped", func(t *testing.T) {
		stdout, stderr, code := runWG(t, bin, nested, "list")
		if code != 0 {
			t.Fatalf("wg list exited %d, stderr: %s", code, stderr)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		if !strings.Contains(stdout, "* ") || !strings.Contains(stdout, "feature-alpha") || !strings.Contains(stdout, alphaPath) {
			t.Fatalf("expected current feature-alpha row with absolute path, got:\n%s", stdout)
		}
		if !strings.Contains(stdout, "feature-alpine") || !strings.Contains(stdout, alpinePath) {
			t.Fatalf("expected feature-alpine row with absolute path, got:\n%s", stdout)
		}
		if !strings.Contains(stdout, detachedPath) || !strings.Contains(strings.ToLower(stdout), "detached") {
			t.Fatalf("expected detached row with absolute path and detached state, got:\n%s", stdout)
		}
		for _, forbidden := range []string{"dirty", "url", "port", "ci", "summary"} {
			if strings.Contains(strings.ToLower(stdout), forbidden) {
				t.Fatalf("wg list output included forbidden %q column/data:\n%s", forbidden, stdout)
			}
		}
	})

	t.Run("path exact name writes only the absolute path", func(t *testing.T) {
		stdout, stderr, code := runWG(t, bin, nested, "path", "feature-alpha")
		if code != 0 {
			t.Fatalf("wg path exited %d, stderr: %s", code, stderr)
		}
		if stdout != alphaPath+"\n" {
			t.Fatalf("expected exact path stdout %q, got %q", alphaPath+"\n", stdout)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
	})

	t.Run("path resolves unique prefixes and refuses ambiguous prefixes", func(t *testing.T) {
		stdout, stderr, code := runWG(t, bin, nested, "path", "feature-alph")
		if code != 0 {
			t.Fatalf("wg path unique prefix exited %d, stderr: %s", code, stderr)
		}
		if stdout != alphaPath+"\n" || stderr != "" {
			t.Fatalf("unique prefix stdout/stderr mismatch: stdout=%q stderr=%q", stdout, stderr)
		}

		stdout, stderr, code = runWG(t, bin, nested, "path", "feature-al")
		if code == 0 {
			t.Fatalf("expected ambiguous prefix to fail")
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout on ambiguity, got %q", stdout)
		}
		if !strings.Contains(stderr, "feature-alpha") || !strings.Contains(stderr, "feature-alpine") {
			t.Fatalf("expected ambiguity stderr to include candidates, got %q", stderr)
		}
	})

	t.Run("env renders stable WG lines with repeatable port", func(t *testing.T) {
		stdout1, stderr, code := runWG(t, bin, nested, "env")
		if code != 0 {
			t.Fatalf("wg env exited %d, stderr: %s", code, stderr)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		stdout2, stderr, code := runWG(t, bin, nested, "env")
		if code != 0 || stderr != "" {
			t.Fatalf("second wg env failed: code=%d stderr=%q", code, stderr)
		}
		if stdout1 != stdout2 {
			t.Fatalf("expected stable env output, first:\n%s\nsecond:\n%s", stdout1, stdout2)
		}
		values := parseEnvLines(t, stdout1)
		if values["WG_BRANCH"] != "feature-alpha" {
			t.Fatalf("expected WG_BRANCH feature-alpha, got %q", values["WG_BRANCH"])
		}
		if values["WG_WORKTREE_PATH"] != alphaPath {
			t.Fatalf("expected WG_WORKTREE_PATH %q, got %q", alphaPath, values["WG_WORKTREE_PATH"])
		}
		port, err := strconv.Atoi(values["WG_PORT"])
		if err != nil || port < 10000 || port > 19999 {
			t.Fatalf("expected WG_PORT in 10000-19999, got %q", values["WG_PORT"])
		}

		stdout, stderr, code := runWG(t, bin, nested, "env", "feature-alpine")
		if code != 0 || stderr != "" {
			t.Fatalf("wg env named failed: code=%d stderr=%q", code, stderr)
		}
		values = parseEnvLines(t, stdout)
		if values["WG_WORKTREE_PATH"] != alpinePath {
			t.Fatalf("expected named env path %q, got %q", alpinePath, values["WG_WORKTREE_PATH"])
		}
	})
}

func buildWG(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "wg")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/wg")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/wg failed: %v\n%s", err, output)
	}
	return bin
}

func initRepo(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	repo := filepath.Join(parent, "demo")
	mustMkdir(t, repo)
	runGit(t, parent, "init", "-b", "main", repo)
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWriteFile(t, filepath.Join(repo, "README.md"), "hello\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	return mustCanonicalPath(t, repo)
}

func addWorktree(t *testing.T, repo, branch, sibling string) string {
	t.Helper()
	runGit(t, repo, "branch", branch)
	path := filepath.Join(filepath.Dir(repo), sibling)
	runGit(t, repo, "worktree", "add", path, branch)
	return mustCanonicalPath(t, path)
}

func addDetachedWorktree(t *testing.T, repo, sibling string) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(repo), sibling)
	runGit(t, repo, "worktree", "add", "--detach", path, "HEAD")
	return mustCanonicalPath(t, path)
}

func runWG(t *testing.T, bin, dir string, args ...string) (string, string, int) {
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

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %v\n%s", args, dir, err, output)
	}
	return string(output)
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustCanonicalPath(t *testing.T, path string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func parseEnvLines(t *testing.T, output string) map[string]string {
	t.Helper()
	values := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid env line %q", line)
		}
		values[key] = value
	}
	return values
}
