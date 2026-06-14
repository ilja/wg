package worktree_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"wg/internal/git"
	"wg/internal/worktree"
)

type fakeRunner struct {
	top       string
	porcelain string
	calls     [][]string
}

func (f *fakeRunner) Run(ctx context.Context, dir string, args ...string) (git.Result, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	switch strings.Join(args, " ") {
	case "rev-parse --show-toplevel":
		return git.Result{Stdout: f.top + "\n", ExitCode: 0}, nil
	case "worktree list --porcelain":
		return git.Result{Stdout: f.porcelain, ExitCode: 0}, nil
	default:
		return git.Result{Stderr: "unexpected command", ExitCode: 1}, nil
	}
}

func TestParsePorcelain(t *testing.T) {
	output := strings.Join([]string{
		"worktree /repo/demo",
		"HEAD aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"branch refs/heads/main",
		"",
		"worktree /repo/demo.feature",
		"HEAD bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"branch refs/heads/feature",
		"locked running setup",
		"",
		"worktree /repo/demo.detached",
		"HEAD cccccccccccccccccccccccccccccccccccccccc",
		"detached",
		"",
		"worktree /repo/demo.bare",
		"bare",
		"",
	}, "\n")

	entries, err := worktree.ParsePorcelain(output)
	if err != nil {
		t.Fatalf("ParsePorcelain returned error: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[0].Path != "/repo/demo" || entries[0].Head != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || entries[0].Branch != "main" {
		t.Fatalf("unexpected branch entry: %#v", entries[0])
	}
	if !entries[1].IsLocked || entries[1].LockReason != "running setup" {
		t.Fatalf("expected locked entry with reason, got %#v", entries[1])
	}
	if !entries[2].IsDetached || entries[2].Branch != "" {
		t.Fatalf("expected detached entry, got %#v", entries[2])
	}
	if !entries[3].IsBare {
		t.Fatalf("expected bare entry, got %#v", entries[3])
	}
}

func TestLoadRepositoryDiscoversPrimaryCurrentAndNames(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo/demo",
		"HEAD 1111111111111111111111111111111111111111",
		"branch refs/heads/main",
		"",
		"worktree /repo/demo.feature-alpha",
		"HEAD 2222222222222222222222222222222222222222",
		"branch refs/heads/feature-alpha",
		"",
		"worktree /other/custom-name",
		"HEAD 3333333333333333333333333333333333333333",
		"branch refs/heads/topic/custom",
		"",
	}, "\n")
	fr := &fakeRunner{top: "/repo/demo.feature-alpha", porcelain: porcelain}

	repo, err := worktree.LoadRepository(context.Background(), fr, "/repo/demo.feature-alpha/nested")
	if err != nil {
		t.Fatalf("LoadRepository returned error: %v", err)
	}
	if repo.CurrentRoot != "/repo/demo.feature-alpha" {
		t.Fatalf("expected current root to be cleaned absolute path, got %q", repo.CurrentRoot)
	}
	if repo.Primary.Path != "/repo/demo" || !repo.Entries[0].IsPrimary {
		t.Fatalf("expected first porcelain record to be primary, repo=%#v", repo)
	}
	if !repo.Entries[1].IsCurrent {
		t.Fatalf("expected linked entry selected as current: %#v", repo.Entries[1])
	}
	if repo.Entries[1].Branch != "feature-alpha" {
		t.Fatalf("expected refs/heads stripped, got %q", repo.Entries[1].Branch)
	}
	if repo.Entries[1].DisplayName != "feature-alpha" || repo.Entries[1].PathBasename != "demo.feature-alpha" {
		t.Fatalf("expected sibling display name and basename, got %#v", repo.Entries[1])
	}
	if repo.Entries[2].DisplayName != "topic/custom" || repo.Entries[2].PathBasename != "custom-name" {
		t.Fatalf("expected branch fallback display name and deterministic basename, got %#v", repo.Entries[2])
	}
	wantCalls := [][]string{{"rev-parse", "--show-toplevel"}, {"worktree", "list", "--porcelain"}}
	if !reflect.DeepEqual(fr.calls, wantCalls) {
		t.Fatalf("git calls mismatch\nwant %#v\n got %#v", wantCalls, fr.calls)
	}
}
