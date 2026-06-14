package rebase

import (
	"context"
	"fmt"
	"io"
	"strings"

	"wg/internal/git"
	"wg/internal/worktree"
)

type Options struct {
	Cwd  string
	Base string
}

type Service struct {
	runner git.Runner
	stdout io.Writer
	stderr io.Writer
}

func New(runner git.Runner, stdout io.Writer, stderr io.Writer) *Service {
	if runner == nil {
		runner = git.ExecRunner{Binary: "git"}
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	return &Service{runner: runner, stdout: stdout, stderr: stderr}
}

func (s *Service) Run(ctx context.Context, opts Options) error {
	repo, err := worktree.LoadRepository(ctx, s.runner, opts.Cwd)
	if err != nil {
		return err
	}

	base, err := worktree.ResolveDefaultBranch(ctx, s.runner, repo, opts.Base)
	if err != nil {
		return err
	}
	target := base.Name
	if fetched, ok := s.fetchRemoteTarget(ctx, repo.Primary.Path, target); ok {
		target = fetched
	}

	if err := git.RunStreaming(ctx, s.runner, repo.CurrentRoot, s.stdout, s.stderr, "rebase", target); err != nil {
		return fmt.Errorf("git rebase %s failed", target)
	}
	return nil
}

func (s *Service) fetchRemoteTarget(ctx context.Context, repoPath string, base string) (string, bool) {
	remoteBranch, ok := remoteBranchName(base)
	if !ok {
		return "", false
	}
	remoteRef := "refs/remotes/origin/" + remoteBranch
	remoteHead := "refs/heads/" + remoteBranch
	check, err := s.runner.Run(ctx, repoPath, "ls-remote", "--exit-code", "origin", remoteHead)
	if err != nil || check.ExitCode != 0 {
		return "", false
	}
	if err := git.RunStreaming(ctx, s.runner, repoPath, s.stdout, s.stderr, "fetch", "origin", remoteHead+":"+remoteRef); err != nil {
		return "", false
	}
	return "origin/" + remoteBranch, true
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
