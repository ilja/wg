package env_test

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	wgenv "wg/internal/env"
	"wg/internal/worktree"
)

func TestDerivePort(t *testing.T) {
	path := "/repo/demo.feature-alpha"
	first := wgenv.DerivePort(path)
	second := wgenv.DerivePort(path)
	if first != second {
		t.Fatalf("expected stable port for same path, got %d and %d", first, second)
	}
	if first < 10000 || first > 19999 {
		t.Fatalf("expected port in 10000-19999, got %d", first)
	}
	other := wgenv.DerivePort("/repo/demo.feature-beta")
	if first == other {
		t.Fatalf("expected path-sensitive ports for fixed test paths, both were %d", first)
	}
}

func TestBuildContextAndRender(t *testing.T) {
	repo := worktree.Repository{
		CurrentRoot: "/repo/demo.feature-alpha",
		Primary: worktree.Entry{
			Path: "/repo/demo", Branch: "main", DisplayName: "main", PathBasename: "demo", IsPrimary: true,
		},
		Entries: []worktree.Entry{
			{Path: "/repo/demo", Branch: "main", DisplayName: "main", PathBasename: "demo", IsPrimary: true},
			{Path: "/repo/demo.feature-alpha", Branch: "feature-alpha", DisplayName: "feature-alpha", PathBasename: "demo.feature-alpha", IsCurrent: true},
		},
	}
	target := repo.Entries[1]
	ctx := wgenv.BuildContext(repo, target, "main", "")
	if ctx.Branch != "feature-alpha" || ctx.WorktreePath != "/repo/demo.feature-alpha" || ctx.WorktreeName != "feature-alpha" {
		t.Fatalf("unexpected worktree context: %#v", ctx)
	}
	if ctx.Repo != "demo" || ctx.RepoPath != "/repo/demo" || ctx.PrimaryWorktreePath != "/repo/demo" {
		t.Fatalf("unexpected repo context: %#v", ctx)
	}
	if ctx.DefaultBranch != "main" || ctx.Base != "" {
		t.Fatalf("unexpected default branch/base context: %#v", ctx)
	}

	lines := wgenv.Render(ctx)
	wantKeys := []string{
		"WG_BRANCH",
		"WG_WORKTREE_PATH",
		"WG_WORKTREE_NAME",
		"WG_REPO",
		"WG_REPO_PATH",
		"WG_PRIMARY_WORKTREE_PATH",
		"WG_DEFAULT_BRANCH",
		"WG_BASE",
		"WG_PORT",
	}
	gotKeys := make([]string, 0, len(lines))
	values := make(map[string]string)
	for _, line := range lines {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid render line %q", line)
		}
		gotKeys = append(gotKeys, key)
		values[key] = value
	}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("render key order mismatch\nwant %#v\n got %#v", wantKeys, gotKeys)
	}
	if values["WG_BRANCH"] != "feature-alpha" || values["WG_WORKTREE_PATH"] != "/repo/demo.feature-alpha" || values["WG_WORKTREE_NAME"] != "feature-alpha" {
		t.Fatalf("unexpected rendered worktree values: %#v", values)
	}
	if values["WG_REPO"] != "demo" || values["WG_REPO_PATH"] != "/repo/demo" || values["WG_PRIMARY_WORKTREE_PATH"] != "/repo/demo" {
		t.Fatalf("unexpected rendered repo values: %#v", values)
	}
	if values["WG_DEFAULT_BRANCH"] != "main" || values["WG_BASE"] != "" {
		t.Fatalf("unexpected default/base values, got %#v", values)
	}
	port, err := strconv.Atoi(values["WG_PORT"])
	if err != nil || port != wgenv.DerivePort("/repo/demo.feature-alpha") {
		t.Fatalf("unexpected rendered port %q", values["WG_PORT"])
	}
}
