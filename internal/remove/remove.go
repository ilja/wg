package remove

import (
	"context"
	"fmt"
	"io"
	"strings"

	"wg/internal/git"
	"wg/internal/resolve"
	"wg/internal/worktree"
)

type Options struct {
	Cwd   string
	Name  string
	Force bool
}

type Result struct {
	RemovedPath    string
	DeletedBranch  string
	CdTarget       string
	RemovedCurrent bool
}

type Service struct {
	runner git.Runner
	stderr io.Writer
}

func New(runner git.Runner, stderr io.Writer) *Service {
	if runner == nil {
		runner = git.ExecRunner{Binary: "git"}
	}
	if stderr == nil {
		stderr = io.Discard
	}
	return &Service{runner: runner, stderr: stderr}
}

func (s *Service) Run(ctx context.Context, opts Options) (Result, error) {
	if opts.Force && opts.Name == "" {
		return Result{}, fmt.Errorf("wg remove -D requires a single named target")
	}

	repo, err := worktree.LoadRepository(ctx, s.runner, opts.Cwd)
	if err != nil {
		return Result{}, err
	}

	target, err := s.target(repo, opts.Name)
	if err != nil {
		return Result{}, err
	}
	if target.IsBare {
		return Result{}, fmt.Errorf("bare worktrees are not supported by wg remove in v1; use explicit native Git commands if you intentionally need bare-worktree deletion")
	}
	if target.IsPrimary {
		return Result{}, fmt.Errorf("refusing to remove the primary worktree %s", target.Path)
	}

	result := Result{RemovedPath: target.Path, RemovedCurrent: target.IsCurrent}
	if target.IsCurrent {
		result.CdTarget = repo.Primary.Path
	}

	if target.Branch == "" || target.IsDetached {
		if !opts.Force {
			return Result{}, fmt.Errorf("detached worktree %s has no branch to prove integrated; pass -D with its name to remove it", target.DisplayName)
		}
		if err := git.RunStreaming(ctx, s.runner, repo.Primary.Path, s.stderr, s.stderr, "worktree", "remove", "--force", target.Path); err != nil {
			return Result{}, err
		}
		_, _ = fmt.Fprintf(s.stderr, "removed detached worktree %s; no branch was deleted\n", target.Path)
		return result, nil
	}

	if opts.Force {
		if err := git.RunStreaming(ctx, s.runner, repo.Primary.Path, s.stderr, s.stderr, "worktree", "remove", "--force", target.Path); err != nil {
			return Result{}, err
		}
		if err := git.RunStreaming(ctx, s.runner, repo.Primary.Path, s.stderr, s.stderr, "branch", "-D", target.Branch); err != nil {
			return Result{}, err
		}
		result.DeletedBranch = target.Branch
		return result, nil
	}

	defaultBranch, err := worktree.ResolveDefaultBranch(ctx, s.runner, repo, "")
	if err != nil {
		return Result{}, err
	}
	safetyTarget := s.fetchTarget(ctx, repo.Primary.Path, defaultBranch.Name)
	proof, ok, err := IsIntegrated(ctx, s.runner, repo.Primary.Path, target.Branch, safetyTarget)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		return Result{}, fmt.Errorf("branch %s is not integrated into %s; use wg remove -D %s to force removing exactly this target", target.Branch, safetyTarget, target.DisplayName)
	}

	if err := git.RunStreaming(ctx, s.runner, repo.Primary.Path, s.stderr, s.stderr, "worktree", "remove", target.Path); err != nil {
		return Result{}, err
	}
	deleteFlag := "-d"
	if proof.Method != "ancestry" {
		deleteFlag = "-D"
	}
	if err := git.RunStreaming(ctx, s.runner, repo.Primary.Path, s.stderr, s.stderr, "branch", deleteFlag, target.Branch); err != nil {
		return Result{}, err
	}
	result.DeletedBranch = target.Branch
	return result, nil
}

func (s *Service) target(repo worktree.Repository, name string) (worktree.Entry, error) {
	if name != "" {
		return resolve.Resolve(repo.Entries, name)
	}
	for _, entry := range repo.Entries {
		if entry.IsCurrent {
			return entry, nil
		}
	}
	return worktree.Entry{}, fmt.Errorf("current worktree %q not found in git worktree list", repo.CurrentRoot)
}

func (s *Service) fetchTarget(ctx context.Context, repoPath string, base string) string {
	branch, ok := remoteBranchName(base)
	if !ok {
		return base
	}
	remoteHead := "refs/heads/" + branch
	remoteRef := "refs/remotes/origin/" + branch
	check, err := s.runner.Run(ctx, repoPath, "ls-remote", "--exit-code", "origin", remoteHead)
	if err != nil || check.ExitCode != 0 {
		return base
	}
	if err := git.RunStreaming(ctx, s.runner, repoPath, s.stderr, s.stderr, "fetch", "origin", remoteHead+":"+remoteRef); err != nil {
		return base
	}
	return "origin/" + branch
}

func remoteBranchName(base string) (string, bool) {
	base = strings.TrimPrefix(base, "refs/heads/")
	if branch, ok := strings.CutPrefix(base, "refs/remotes/origin/"); ok && branch != "" {
		return branch, true
	}
	if branch, ok := strings.CutPrefix(base, "origin/"); ok && branch != "" {
		return branch, true
	}
	if base == "" || strings.Contains(base, ":") || strings.HasPrefix(base, "-") {
		return "", false
	}
	return base, true
}
