package remove

import (
	"context"
	"errors"
	"strings"
	"testing"

	"wg/internal/git"
)

func TestRunAllRecordsPartialRemovalAndStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &cancelDuringBranchDeletionRunner{cancel: cancel}
	service := New(runner, nil)

	result, err := service.RunAll(ctx, RemoveAllOptions{Cwd: "/repo"})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("expected one failed target, got %#v", result.Failed)
	}
	failure := result.Failed[0]
	if failure.Name != "feature/first" || !failure.WorktreeRemoved {
		t.Fatalf("expected partial removal for feature/first, got %#v", failure)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/repo.later status --porcelain") {
			t.Fatalf("expected cancellation to stop before later target, calls: %#v", runner.calls)
		}
	}
}

type cancelDuringBranchDeletionRunner struct {
	cancel context.CancelFunc
	calls  []string
}

func (r *cancelDuringBranchDeletionRunner) Run(_ context.Context, dir string, args ...string) (git.Result, error) {
	call := strings.TrimSpace(dir + " " + strings.Join(args, " "))
	r.calls = append(r.calls, call)

	switch {
	case call == "/repo rev-parse --show-toplevel":
		return git.Result{Stdout: "/repo\n"}, nil
	case call == "/repo worktree list --porcelain":
		return git.Result{Stdout: strings.Join([]string{
			"worktree /repo",
			"HEAD 1111111111111111111111111111111111111111",
			"branch refs/heads/main",
			"",
			"worktree /repo.first",
			"HEAD 2222222222222222222222222222222222222222",
			"branch refs/heads/feature/first",
			"",
			"worktree /repo.later",
			"HEAD 3333333333333333333333333333333333333333",
			"branch refs/heads/feature/later",
			"",
		}, "\n")}, nil
	case call == "/repo symbolic-ref --quiet --short refs/remotes/origin/HEAD":
		return git.Result{Stdout: "origin/main\n"}, nil
	case call == "/repo ls-remote --exit-code origin refs/heads/main":
		return git.Result{ExitCode: 1}, nil
	case call == "/repo.first status --porcelain":
		return git.Result{}, nil
	case call == "/repo merge-base --is-ancestor feature/first origin/main":
		return git.Result{}, nil
	case call == "/repo worktree remove /repo.first":
		return git.Result{}, nil
	case call == "/repo branch -d feature/first":
		r.cancel()
		return git.Result{}, context.Canceled
	default:
		return git.Result{ExitCode: 1, Stderr: "unexpected test command: " + call}, nil
	}
}
