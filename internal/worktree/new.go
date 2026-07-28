package worktree

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"wg/internal/git"
)

type NewOptions struct {
	Cwd     string
	Branch  string
	Base    string
	Stdout  io.Writer
	Stderr  io.Writer
	Environ []string
}

type NewPlan struct {
	Repository    Repository
	Branch        string
	WorktreeName  string
	WorktreePath  string
	DefaultBranch DefaultBranch
	Base          string
}

type NewResult struct {
	Plan  NewPlan
	Setup SetupResult
}

type Creator struct {
	Runner git.Runner
}

func (c Creator) Plan(ctx context.Context, opts NewOptions) (NewPlan, error) {
	runner := c.runner()
	repo, err := LoadRepository(ctx, runner, opts.Cwd)
	if err != nil {
		return NewPlan{}, err
	}

	defaultBranch, err := ResolveDefaultBranch(ctx, runner, repo, opts.Base)
	if err != nil {
		return NewPlan{}, err
	}

	name, path, err := SiblingWorktreePath(repo.Primary.Path, opts.Branch)
	if err != nil {
		return NewPlan{}, err
	}

	plan := NewPlan{
		Repository:    repo,
		Branch:        opts.Branch,
		WorktreeName:  name,
		WorktreePath:  path,
		DefaultBranch: defaultBranch,
		Base:          defaultBranch.Name,
	}
	if err := c.checkCollisions(ctx, runner, plan); err != nil {
		return NewPlan{}, err
	}
	return plan, nil
}

func (c Creator) Create(ctx context.Context, opts NewOptions) (NewResult, error) {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	plan, err := c.Plan(ctx, opts)
	if err != nil {
		return NewResult{}, err
	}

	result, err := c.runner().Run(ctx, plan.Repository.Primary.Path, "worktree", "add", "--no-track", "-b", plan.Branch, plan.WorktreePath, plan.Base)
	if result.Stdout != "" {
		_, _ = io.WriteString(stderr, result.Stdout)
	}
	if result.Stderr != "" {
		_, _ = io.WriteString(stderr, result.Stderr)
	}
	if err != nil {
		return NewResult{}, err
	}
	if result.ExitCode != 0 {
		return NewResult{}, fmt.Errorf("git worktree add failed for branch %s at %s", plan.Branch, plan.WorktreePath)
	}

	setupContext := BuildSetupContext(plan)
	setupContext.BaseEnv = opts.Environ
	setupResult, err := RunSetup(ctx, setupContext, stderr)
	if err != nil {
		return NewResult{Plan: plan, Setup: setupResult}, err
	}
	if !setupResult.Ran {
		_, _ = fmt.Fprintln(stderr, "warning: no .config/setup.sh found; project setup was not run")
	}

	_, _ = fmt.Fprintln(stdout, plan.WorktreePath)
	return NewResult{Plan: plan, Setup: setupResult}, nil
}

func (c Creator) runner() git.Runner {
	if c.Runner != nil {
		return c.Runner
	}
	return git.ExecRunner{Binary: "git"}
}

func (c Creator) checkCollisions(ctx context.Context, runner git.Runner, plan NewPlan) error {
	if plan.Branch == "" {
		return fmt.Errorf("branch is required")
	}

	branchExists, err := localBranchExists(ctx, runner, plan.Repository, plan.Branch)
	if err != nil {
		return err
	}
	if branchExists {
		return fmt.Errorf("branch %q already exists", plan.Branch)
	}

	pathBasename := filepath.Base(plan.WorktreePath)
	for _, entry := range plan.Repository.Entries {
		if entry.DisplayName == plan.WorktreeName || entry.PathBasename == pathBasename || entry.Path == plan.WorktreePath || entrySanitizesTo(entry, plan.WorktreeName) {
			return fmt.Errorf("worktree name %q already exists at %s", plan.WorktreeName, entry.Path)
		}
	}

	info, err := os.Lstat(plan.WorktreePath)
	if err == nil {
		if info.IsDir() && directoryHasEntries(plan.WorktreePath) {
			return fmt.Errorf("non-empty target location already exists: %s", plan.WorktreePath)
		}
		return fmt.Errorf("path already exists: %s", plan.WorktreePath)
	}
	if !os.IsNotExist(err) {
		return err
	}

	return nil
}

func directoryHasEntries(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func entrySanitizesTo(entry Entry, name string) bool {
	for _, candidate := range []string{entry.DisplayName, entry.Branch} {
		if candidate == "" {
			continue
		}
		sanitized, err := SanitizeWorktreeName(candidate)
		if err == nil && sanitized == name {
			return true
		}
	}
	return false
}
