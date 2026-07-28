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

func TestZshRemoveCompletionOffersMatchingBranchPrefixes(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is unavailable")
	}

	bin := buildWG(t)
	repo := initRepo(t)
	addWorktree(t, repo, "feature-alpha", "demo.feature-alpha")
	addWorktree(t, repo, "feature-alpine", "demo.feature-alpine")

	stdout, stderr, code := runZsh(t, bin, fmt.Sprintf(`
autoload -Uz compinit
compinit -D
eval "$(wg config shell init zsh)"
builtin cd -- %q
compadd() {
  if [[ "$1" == "-a" ]]; then
    typeset -p "$2"
  fi
}

words=(wg remove feature-alph)
CURRENT=3
_wg

words=(wg remove feature-a)
CURRENT=3
_wg

words=(wg remove -D feature-alph)
CURRENT=4
_wg

words=(wg remove feature-alpha extra)
CURRENT=4
_wg
`, repo))
	if code != 0 {
		t.Fatalf("zsh script exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	want := strings.Join([]string{
		"typeset -g -a wg_remove_candidates=( feature-alpha )",
		"typeset -g -a wg_remove_candidates=( feature-alpha feature-alpine )",
		"typeset -g -a wg_remove_candidates=( feature-alpha )",
		"",
	}, "\n")
	if stdout != want {
		t.Fatalf("expected matching branch completions, got stdout %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
