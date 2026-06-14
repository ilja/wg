package integration

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestZshRemoveCurrentWorktreeFallsBackToPrimary(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is unavailable")
	}

	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branch := "feature/zsh-remove"
	path := addLifecycleWorktree(t, repo, branch)
	commitFile(t, path, "zsh.txt", "zsh\n", "zsh feature")
	runGit(t, repo, "merge", "--ff-only", branch)
	runGit(t, repo, "push", "origin", "main")

	stdout, stderr, code := runZsh(t, bin, fmt.Sprintf(`
eval "$(wg config shell init zsh)"
builtin cd -- %q
wg remove
printf '%%s\n' "$PWD"
`, path))
	if code != 0 {
		t.Fatalf("zsh script exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if stdout != repo+"\n" {
		t.Fatalf("expected final PWD %q, got stdout %q", repo+"\n", stdout)
	}
	if strings.Contains(stderr, repo+"\n") || strings.Contains(stderr, path+"\n") {
		t.Fatalf("expected no machine path diagnostics on stderr, got %q", stderr)
	}
	assertPathMissing(t, path)
	assertBranchMissing(t, repo, branch)
}
