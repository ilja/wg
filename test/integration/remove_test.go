package integration

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveAllDeletesIntegratedWorktreesAndSkipsUnintegratedWorktrees(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	integratedBranch := "feature/remove-all-integrated"
	integratedPath := addLifecycleWorktree(t, repo, integratedBranch)
	commitFile(t, integratedPath, "integrated.txt", "integrated\n", "integrated feature")
	unintegratedBranch := "feature/remove-all-unintegrated"
	unintegratedPath := addLifecycleWorktree(t, repo, unintegratedBranch)
	commitFile(t, unintegratedPath, "unintegrated.txt", "unintegrated\n", "unintegrated feature")
	runGit(t, repo, "merge", "--ff-only", integratedBranch)
	runGit(t, repo, "push", "origin", "main")

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "--all")
	if code != 0 {
		t.Fatalf("wg remove --all exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "removed "+integratedBranch) {
		t.Fatalf("expected integrated removal report, got %q", stderr)
	}
	if !strings.Contains(stderr, "skipped "+unintegratedBranch) || !strings.Contains(stderr, "not integrated") {
		t.Fatalf("expected unintegrated skip report, got %q", stderr)
	}
	assertPathMissing(t, integratedPath)
	assertBranchMissing(t, repo, integratedBranch)
	assertPathExists(t, unintegratedPath)
	assertBranchExists(t, repo, unintegratedBranch)
}

func TestRemoveAllDryRunReportsWithoutRemovingIntegratedWorktree(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branch := "feature/remove-all-dry-run"
	path := addLifecycleWorktree(t, repo, branch)
	commitFile(t, path, "dry-run.txt", "dry run\n", "dry-run feature")
	runGit(t, repo, "merge", "--ff-only", branch)
	runGit(t, repo, "push", "origin", "main")

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "--all", "--dry-run")
	if code != 0 {
		t.Fatalf("wg remove --all --dry-run exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "would remove "+branch) {
		t.Fatalf("expected dry-run report, got %q", stderr)
	}
	assertPathExists(t, path)
	assertBranchExists(t, repo, branch)
}

func TestRemoveAllSkipsDirtyIntegratedWorktree(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branch := "feature/remove-all-dirty"
	path := addLifecycleWorktree(t, repo, branch)
	commitFile(t, path, "dirty.txt", "committed\n", "dirty feature")
	runGit(t, repo, "merge", "--ff-only", branch)
	runGit(t, repo, "push", "origin", "main")
	mustWriteFile(t, filepath.Join(path, "untracked.txt"), "untracked\n")

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "--all")
	if code != 0 {
		t.Fatalf("wg remove --all exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "skipped "+branch) || !strings.Contains(stderr, "dirty worktree") {
		t.Fatalf("expected dirty-worktree skip report, got %q", stderr)
	}
	assertPathExists(t, path)
	assertBranchExists(t, repo, branch)
}

func TestRemoveAllSkipsLockedIntegratedWorktree(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branch := "feature/remove-all-locked"
	path := addLifecycleWorktree(t, repo, branch)
	commitFile(t, path, "locked.txt", "locked\n", "locked feature")
	runGit(t, repo, "merge", "--ff-only", branch)
	runGit(t, repo, "push", "origin", "main")
	runGit(t, repo, "worktree", "lock", "--reason", "test lock", path)

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "--all")
	if code != 0 {
		t.Fatalf("wg remove --all exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "skipped "+branch) || !strings.Contains(stderr, "locked worktree") {
		t.Fatalf("expected locked-worktree skip report, got %q", stderr)
	}
	assertPathExists(t, path)
	assertBranchExists(t, repo, branch)
}

func TestRemoveAllSkipsDetachedWorktree(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	path := filepath.Join(filepath.Dir(repo), "demo.remove-all-detached")
	runGit(t, repo, "worktree", "add", "--detach", path, "main")
	path = mustCanonicalPath(t, path)

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "--all")
	if code != 0 {
		t.Fatalf("wg remove --all exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "skipped remove-all-detached") || !strings.Contains(stderr, "detached worktree") {
		t.Fatalf("expected detached-worktree skip report, got %q", stderr)
	}
	assertPathExists(t, path)
}

func TestRemoveAllSkipsCurrentIntegratedWorktree(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branch := "feature/remove-all-current"
	path := addLifecycleWorktree(t, repo, branch)
	commitFile(t, path, "current.txt", "current\n", "current feature")
	runGit(t, repo, "merge", "--ff-only", branch)
	runGit(t, repo, "push", "origin", "main")

	stdout, stderr, code := runWGCommand(t, bin, path, "remove", "--all")
	if code != 0 {
		t.Fatalf("wg remove --all exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "skipped "+branch) || !strings.Contains(stderr, "current worktree") {
		t.Fatalf("expected current-worktree skip report, got %q", stderr)
	}
	assertPathExists(t, path)
	assertBranchExists(t, repo, branch)
}

func TestRemoveAllRejectsNamedTarget(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "--all", "feature")
	if code == 0 {
		t.Fatalf("expected wg remove --all with a name to fail")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "cannot be combined") {
		t.Fatalf("expected incompatible-options diagnostic, got %q", stderr)
	}
}

func TestRemoveAllRejectsForce(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "--all", "-D")
	if code == 0 {
		t.Fatalf("expected wg remove --all -D to fail")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "cannot be combined") {
		t.Fatalf("expected incompatible-options diagnostic, got %q", stderr)
	}
}

func TestRemoveDryRunRequiresAll(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "--dry-run")
	if code == 0 {
		t.Fatalf("expected wg remove --dry-run without --all to fail")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "requires --all") {
		t.Fatalf("expected --all requirement diagnostic, got %q", stderr)
	}
}

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

func TestRemoveCherryEquivalentWorktreeDeletesWorktreeAndBranch(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branch := "feature/remove-cherry"
	path := addLifecycleWorktree(t, repo, branch)
	commitFile(t, path, "cherry.txt", "cherry\n", "cherry feature")
	commit := strings.TrimSpace(runGit(t, path, "rev-parse", "HEAD"))
	runGit(t, repo, "cherry-pick", commit)
	commitFile(t, repo, "main-only.txt", "main only\n", "main only")
	runGit(t, repo, "push", "origin", "main")

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "remove-cherry")
	if code != 0 {
		t.Fatalf("wg remove exited %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	assertPathMissing(t, path)
	assertBranchMissing(t, repo, branch)
}

func TestRemoveCumulativePatchEquivalentWorktreeDeletesWorktreeAndBranch(t *testing.T) {
	bin := buildWG(t)
	repo := initRepoWithOrigin(t)
	branch := "feature/remove-patch-id"
	path := addLifecycleWorktree(t, repo, branch)
	commitFile(t, path, "patch.txt", "first\n", "patch first")
	commitFile(t, path, "patch.txt", "first\nsecond\n", "patch second")

	runGit(t, repo, "checkout", "-b", "squash-equivalent", "main")
	mustWriteFile(t, filepath.Join(repo, "patch.txt"), "first\nsecond\n")
	runGit(t, repo, "add", "patch.txt")
	runGit(t, repo, "commit", "-m", "squashed patch feature")
	runGit(t, repo, "checkout", "main")
	runGit(t, repo, "merge", "--no-ff", "squash-equivalent", "-m", "merge squash equivalent")
	commitFile(t, repo, "main-only.txt", "main only\n", "main only")
	runGit(t, repo, "push", "origin", "main")

	stdout, stderr, code := runWGCommand(t, bin, repo, "remove", "remove-patch-id")
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
