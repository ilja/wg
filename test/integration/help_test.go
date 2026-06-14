package integration

import (
	"strings"
	"testing"
)

func TestHelpShowsScopedFirstVersionSurface(t *testing.T) {
	bin := buildWG(t)
	stdout, stderr, code := runWGCommand(t, bin, t.TempDir(), "--help")
	if code != 0 {
		t.Fatalf("wg --help exited %d, stderr: %s", code, stderr)
	}
	for _, want := range []string{"list", "switch", "path", "new", "rebase", "copy-ignored", "env", "remove"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected wg --help to mention %q, got %q", want, stdout)
		}
	}
	assertOutOfScopeHelpAbsent(t, stdout)
}

func TestRebaseHelpIsScoped(t *testing.T) {
	bin := buildWG(t)
	stdout, stderr, code := runWGCommand(t, bin, t.TempDir(), "rebase", "--help")
	if code != 0 {
		t.Fatalf("wg rebase --help exited %d, stderr: %s", code, stderr)
	}
	for _, want := range []string{"rebase", "base"} {
		if !strings.Contains(strings.ToLower(stdout), want) {
			t.Fatalf("expected rebase help to mention %q, got %q", want, stdout)
		}
	}
	assertOutOfScopeHelpAbsent(t, stdout)
}

func TestRemoveHelpDocumentsSingleTargetForce(t *testing.T) {
	bin := buildWG(t)
	stdout, stderr, code := runWGCommand(t, bin, t.TempDir(), "remove", "--help")
	if code != 0 {
		t.Fatalf("wg remove --help exited %d, stderr: %s", code, stderr)
	}
	for _, want := range []string{"remove", "-D", "single", "target"} {
		if !strings.Contains(strings.ToLower(stdout), strings.ToLower(want)) {
			t.Fatalf("expected remove help to mention %q, got %q", want, stdout)
		}
	}
	assertOutOfScopeHelpAbsent(t, stdout)
}

func assertOutOfScopeHelpAbsent(t *testing.T, help string) {
	t.Helper()
	lower := strings.ToLower(help)
	for _, forbidden := range []string{
		"bulk prune",
		"prune",
		"merge workflow",
		"llm",
		"commit/squash/push",
		"generic hook",
		"hook dsl",
		"dev-server",
		"dev server",
		"url",
		"status columns",
		"automatic migration",
		"git workflow replacement",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("expected help not to mention out-of-scope term %q, got %q", forbidden, help)
		}
	}
}
