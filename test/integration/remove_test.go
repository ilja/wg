package integration

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveMergedWorktreeDeletesWorktreeAndBranch(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branch := "feature/remove-merged"
	path := addLifecycleWorktree(t, repo, branch)
	commitFile(t, path, "merged.txt", "merged\n", "merged feature")
	runGit(t, repo, "merge", "--ff-only", branch)
	runGit(t, repo, "push", "origin", "main")

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "remove-merged")
	if code != 0 {
		t.Fatalf("wg remove exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	assertPathMissing(t, path)
	assertBranchMissing(t, repo, branch)
}

func TestRemoveSquashEquivalentWorktreeDeletesWorktreeAndBranch(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branch := "feature/remove-squash"
	path := addLifecycleWorktree(t, repo, branch)
	commitFile(t, path, "squash.txt", "squash\n", "squash feature")
	runGit(t, repo, "merge", "--squash", branch)
	runGit(t, repo, "commit", "-m", "squash feature")
	runGit(t, repo, "push", "origin", "main")

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "remove-squash")
	if code != 0 {
		t.Fatalf("wg remove exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	assertPathMissing(t, path)
	assertBranchMissing(t, repo, branch)
}

func TestRemoveUnmergedRefusesAndPreservesWorktreeAndBranch(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branch := "feature/remove-unmerged"
	path := addLifecycleWorktree(t, repo, branch)
	commitFile(t, path, "unmerged.txt", "unmerged\n", "unmerged feature")

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "remove-unmerged")
	if code == 0 {
		t.Fatalf("expected unmerged remove refusal")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "-D") {
		t.Fatalf("expected stderr to mention -D, got %q", stderr)
	}
	assertPathExists(t, path)
	assertBranchExists(t, repo, branch)
}

func TestRemoveForceNamedDeletesOneUnmergedWorktree(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branchOne := "feature/remove-force-one"
	branchTwo := "feature/remove-force-two"
	pathOne := addLifecycleWorktree(t, repo, branchOne)
	pathTwo := addLifecycleWorktree(t, repo, branchTwo)
	commitFile(t, pathOne, "one.txt", "one\n", "one")
	commitFile(t, pathTwo, "two.txt", "two\n", "two")

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "-D", "remove-force-one")
	if code != 0 {
		t.Fatalf("wg remove -D exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	assertPathMissing(t, pathOne)
	assertBranchMissing(t, repo, branchOne)
	assertPathExists(t, pathTwo)
	assertBranchExists(t, repo, branchTwo)
}

func TestRemoveRefusesPrimaryWorktree(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove")
	if code == 0 {
		t.Fatalf("expected primary remove refusal")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "primary") {
		t.Fatalf("expected primary-worktree diagnostic, got %q", stderr)
	}
	assertPathExists(t, repo)
	assertBranchExists(t, repo, "main")
}

func TestRemoveForceRequiresNamedTarget(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branch := "feature/remove-force-unnamed"
	path := addLifecycleWorktree(t, repo, branch)
	commitFile(t, path, "force.txt", "force\n", "force")

	stdout, stderr, code := runWGCommand(t, bin, path, "remove", "-D")
	if code == 0 {
		t.Fatalf("expected unnamed -D refusal")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "name") && !strings.Contains(strings.ToLower(stderr), "target") {
		t.Fatalf("expected named-target diagnostic, got %q", stderr)
	}
	assertPathExists(t, path)
	assertBranchExists(t, repo, branch)
}

func TestRemoveDetachedWorktreeRequiresForceAndForceSkipsBranchDeletion(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	path := filepath.Join(filepath.Dir(repo), "demo.detached-remove")
	runGit(t, repo, "worktree", "add", "--detach", path, "main")
	path = mustCanonicalPath(t, path)

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "detached-remove")
	if code == 0 {
		t.Fatalf("expected detached refusal")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "detached") || !strings.Contains(stderr, "-D") {
		t.Fatalf("expected detached -D diagnostic, got %q", stderr)
	}
	assertPathExists(t, path)

	stdout, stderr, code = runWGCommand(t, bin, repo, "remove", "-D", "detached-remove")
	if code != 0 {
		t.Fatalf("wg remove -D detached exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "no branch") && !strings.Contains(strings.ToLower(stderr), "detached") {
		t.Fatalf("expected clear detached branch-skip note, got %q", stderr)
	}
	assertPathMissing(t, path)
}

func TestRemoveRefusesBareWorktreeEvenWithForce(t *testing.T) {
	bin := buildWG(t)
	repo := initBarePrimaryWithLinkedWorktree(t)
	linked := filepath.Join(filepath.Dir(repo), "linked")

	stdout, stderr, code := runWGCommand(t, bin, linked, "remove", "-D", filepath.Base(repo))
	if code == 0 {
		t.Fatalf("expected bare worktree refusal")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "bare") || !strings.Contains(lower, "git") {
		t.Fatalf("expected bare/native Git guidance, got %q", stderr)
	}
	assertPathExists(t, repo)
}
