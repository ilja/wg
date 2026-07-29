package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initRepoWithOrigin(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	origin := filepath.Join(parent, "origin.git")
	runGit(t, parent, "init", "--bare", "-b", "main", origin)
	repo := filepath.Join(parent, "demo")
	runGit(t, parent, "clone", origin, repo)
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWriteFile(t, filepath.Join(repo, "README.md"), "hello\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	runGit(t, repo, "push", "-u", "origin", "main")
	runGit(t, repo, "remote", "set-head", "origin", "-a")
	return mustCanonicalPath(t, repo)
}

func addLifecycleWorktree(t *testing.T, repo, branch string) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(repo), "demo."+lastBranchComponent(branch))
	runGit(t, repo, "worktree", "add", "-b", branch, path, "main")
	return mustCanonicalPath(t, path)
}

func advanceOriginDefaultBranch(t *testing.T, repo, name, content, message string) string {
	t.Helper()
	remoteURL := strings.TrimSpace(runGit(t, repo, "remote", "get-url", "origin"))
	clone := filepath.Join(t.TempDir(), "remote")
	runGit(t, filepath.Dir(clone), "clone", remoteURL, clone)
	runGit(t, clone, "config", "user.email", "test@example.com")
	runGit(t, clone, "config", "user.name", "Test User")
	commit := commitFile(t, clone, name, content, message)
	runGit(t, clone, "push", "origin", "main")
	return commit
}

func assertAncestor(t *testing.T, repo, ancestor, descendant string) {
	t.Helper()
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected %s to be ancestor of %s in %s: %v\n%s", ancestor, descendant, repo, err, output)
	}
}

func assertCleanStatus(t *testing.T, repo string) {
	t.Helper()
	if status := gitStatusShort(t, repo); status != "" {
		t.Fatalf("expected clean status in %s, got %q", repo, status)
	}
}

func gitStatusShort(t *testing.T, repo string) string {
	t.Helper()
	return runGit(t, repo, "status", "--short")
}

func replaceEnvironment(environ []string, replacements ...string) []string {
	values := make(map[string]string, len(replacements))
	for _, replacement := range replacements {
		key, _, ok := strings.Cut(replacement, "=")
		if ok {
			values[key] = replacement
		}
	}

	result := make([]string, 0, len(environ)+len(values))
	for _, entry := range environ {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replaced := values[key]; replaced {
				continue
			}
		}
		result = append(result, entry)
	}
	for _, replacement := range replacements {
		key, _, ok := strings.Cut(replacement, "=")
		if !ok || values[key] != replacement {
			continue
		}
		result = append(result, replacement)
		delete(values, key)
	}
	return result
}

func assertBranchExists(t *testing.T, repo, branch string) {
	t.Helper()
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected branch %s to exist in %s: %v", branch, repo, err)
	}
}

func assertBranchMissing(t *testing.T, repo, branch string) {
	t.Helper()
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repo
	if err := cmd.Run(); err == nil {
		t.Fatalf("expected branch %s to be missing in %s", branch, repo)
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path %s to exist: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected path %s to be missing, stat err: %v", path, err)
	}
}

func initBarePrimaryWithLinkedWorktree(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	bare := filepath.Join(parent, "primary.git")
	runGit(t, parent, "init", "--bare", "-b", "main", bare)
	seed := filepath.Join(parent, "seed")
	runGit(t, parent, "clone", bare, seed)
	runGit(t, seed, "config", "user.email", "test@example.com")
	runGit(t, seed, "config", "user.name", "Test User")
	mustWriteFile(t, filepath.Join(seed, "README.md"), "hello\n")
	runGit(t, seed, "add", "README.md")
	runGit(t, seed, "commit", "-m", "initial")
	runGit(t, seed, "push", "origin", "main")
	linked := filepath.Join(parent, "linked")
	runGit(t, parent, "--git-dir", bare, "worktree", "add", linked, "main")
	return mustCanonicalPath(t, bare)
}

func lastBranchComponent(branch string) string {
	parts := strings.Split(branch, "/")
	return parts[len(parts)-1]
}
