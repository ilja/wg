package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupExistingGitWorktree(t *testing.T) {
	bin := buildWG(t)
	repo := initRepo(t)
	target := filepath.Join(filepath.Dir(repo), "custom-worktree")
	runGit(t, repo, "worktree", "add", "-b", "feature/setup", target)
	capture := filepath.Join(filepath.Dir(repo), "setup-existing.env")
	installSetupScript(t, repo, "#!/bin/sh\n{ printf 'PWD=%s\\n' \"$PWD\"; env | grep '^WG_' | sort; } > "+shellQuote(capture)+"\n")

	_, stderr, code := runWGCommand(t, bin, target, "setup")
	if code != 0 {
		t.Fatalf("wg setup exited %d: %s", code, stderr)
	}
	values := parseCapturedSetup(t, capture)
	for key, want := range map[string]string{"PWD": target, "WG_WORKTREE_PATH": target, "WG_BRANCH": "feature/setup", "WG_PRIMARY_WORKTREE_PATH": repo, "WG_BASE": ""} {
		if got, exists := values[key]; !exists || got != want {
			t.Errorf("%s = %q (present: %t), want %q", key, got, exists, want)
		}
	}
	if err := os.Remove(capture); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = runWGCommand(t, bin, repo, "setup", "feature/setup")
	if code != 0 {
		t.Fatalf("named wg setup exited %d: %s", code, stderr)
	}
	values = parseCapturedSetup(t, capture)
	if values["PWD"] != target || values["WG_BRANCH"] != "feature/setup" {
		t.Fatalf("named setup ran in the wrong context: %#v", values)
	}
}

func TestSetupMissingScriptFails(t *testing.T) {
	bin := buildWG(t)
	repo := initRepo(t)
	_, stderr, code := runWGCommand(t, bin, repo, "setup")
	if code == 0 || !strings.Contains(stderr, "no .config/setup.sh found") {
		t.Fatalf("expected missing script error, got exit %d: %s", code, stderr)
	}
}
