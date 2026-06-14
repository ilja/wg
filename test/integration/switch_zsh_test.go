package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwitchZshChangesParentShellDirectory(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is unavailable")
	}

	bin := buildWG(t)
	repo := initRepo(t)
	alphaPath := addWorktree(t, repo, "feature-alpha", "demo.feature-alpha")

	stdout, stderr, code := runZsh(t, bin, fmt.Sprintf(`
eval "$(wg config shell init zsh)"
builtin cd -- %q
wg switch feature-alpha
printf '%%s\n' "$PWD"
`, repo))
	if code != 0 {
		t.Fatalf("zsh script exited %d, stderr: %s", code, stderr)
	}
	if stdout != alphaPath+"\n" {
		t.Fatalf("expected final PWD %q, got stdout %q", alphaPath+"\n", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSwitchZshLeavesDirectoryUnchangedOnAmbiguity(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is unavailable")
	}

	bin := buildWG(t)
	repo := initRepo(t)
	alphaPath := addWorktree(t, repo, "feature-alpha", "demo.feature-alpha")
	alpinePath := addWorktree(t, repo, "feature-alpine", "demo.feature-alpine")

	stdout, stderr, code := runZsh(t, bin, fmt.Sprintf(`
eval "$(wg config shell init zsh)"
builtin cd -- %q
wg switch feature-al
wg_status=$?
printf 'status=%%d\npwd=%%s\n' "$wg_status" "$PWD"
`, repo))
	if code != 0 {
		t.Fatalf("zsh script exited %d, stderr: %s", code, stderr)
	}
	if stdout != fmt.Sprintf("status=1\npwd=%s\n", repo) {
		t.Fatalf("expected unchanged PWD stdout, got %q", stdout)
	}
	if strings.Contains(stdout, alphaPath) || strings.Contains(stdout, alpinePath) {
		t.Fatalf("expected stdout to contain no selected target path, got %q", stdout)
	}
	if !strings.Contains(stderr, "ambiguous") || !strings.Contains(stderr, "feature-alpha") || !strings.Contains(stderr, "feature-alpine") {
		t.Fatalf("expected ambiguity diagnostic with candidates on stderr, got %q", stderr)
	}
}

func buildWG(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
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

func runZsh(t *testing.T, bin string, script string) (string, string, int) {
	t.Helper()
	cmd := exec.Command("zsh", "-f", "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
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
	t.Fatalf("failed to run zsh script: %v", err)
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
