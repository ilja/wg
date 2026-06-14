package integration

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRebaseExplicitBaseFetchesAndReplays(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	worktreePath := addLifecycleWorktree(t, repo, "feature/rebase-explicit")
	commitFile(t, worktreePath, "feature.txt", "feature\n", "feature commit")
	remoteTip := advanceOriginDefaultBranch(t, repo, "remote.txt", "remote\n", "remote commit")

	stdout, stderr, code := runWGCommand(t, bin, worktreePath, "rebase", "main")
	if code != 0 {
		t.Fatalf("wg rebase exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	assertAncestor(t, worktreePath, remoteTip, "HEAD")
	assertCleanStatus(t, worktreePath)
}

func TestRebaseDefaultBaseFetchesAndReplays(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	worktreePath := addLifecycleWorktree(t, repo, "feature/rebase-default")
	commitFile(t, worktreePath, "feature.txt", "feature\n", "feature commit")
	remoteTip := advanceOriginDefaultBranch(t, repo, "remote-default.txt", "remote default\n", "remote default commit")

	stdout, stderr, code := runWGCommand(t, bin, worktreePath, "rebase")
	if code != 0 {
		t.Fatalf("wg rebase exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	assertAncestor(t, worktreePath, remoteTip, "HEAD")
	assertCleanStatus(t, worktreePath)
}

func TestRebaseRefusesDirtyWorktreeAndLeavesChanges(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	worktreePath := addLifecycleWorktree(t, repo, "feature/rebase-dirty")
	mustWriteFile(t, filepath.Join(worktreePath, "README.md"), "dirty local edit\n")

	stdout, stderr, code := runWGCommand(t, bin, worktreePath, "rebase", "main")
	if code == 0 {
		t.Fatalf("expected dirty rebase refusal")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout for dirty refusal, got %q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "unstaged changes") && !strings.Contains(strings.ToLower(stderr), "local changes") {
		t.Fatalf("expected Git dirty-worktree diagnostic, got %q", stderr)
	}
	if got := gitStatusShort(t, worktreePath); !strings.Contains(got, " M README.md") {
		t.Fatalf("expected unstaged README.md change to remain, status: %q", got)
	}
}

func TestRebaseSurfacesConflictsForUserResolution(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	worktreePath := addLifecycleWorktree(t, repo, "feature/rebase-conflict")
	mustWriteFile(t, filepath.Join(worktreePath, "README.md"), "feature line\n")
	runGit(t, worktreePath, "add", "README.md")
	runGit(t, worktreePath, "commit", "-m", "feature conflict")
	advanceOriginDefaultBranch(t, repo, "README.md", "main line\n", "main conflict")

	stdout, stderr, code := runWGCommand(t, bin, worktreePath, "rebase", "main")
	if code == 0 {
		t.Fatalf("expected conflicting rebase to fail")
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "CONFLICT") && !strings.Contains(strings.ToLower(combined), "could not apply") {
		t.Fatalf("expected Git conflict output, got stdout %q stderr %q", stdout, stderr)
	}
	if got := gitStatusShort(t, worktreePath); !strings.Contains(got, "UU README.md") {
		t.Fatalf("expected unresolved conflict status, got %q", got)
	}
}
