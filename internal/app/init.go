package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"wg/internal/git"
	"wg/internal/initsetup"
	"wg/internal/worktree"
)

type InitOptions struct {
	Force bool
}

type InitResult struct {
	HookPath string
}

func (a *App) Init(ctx context.Context, opts InitOptions) (InitResult, error) {
	runner := a.runner()
	repo, err := worktree.LoadRepository(ctx, runner, a.cwd())
	if err != nil {
		return InitResult{}, err
	}

	templatePath, err := initsetup.ResolveTemplatePath(a.environ())
	if err != nil {
		return InitResult{}, err
	}
	hookPath := filepath.Join(repo.Primary.Path, ".config", "setup.sh")
	if err := initsetup.Install(templatePath, hookPath, opts.Force); err != nil {
		return InitResult{}, err
	}

	excludeOutput, err := git.Output(ctx, runner, repo.Primary.Path, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return InitResult{}, fmt.Errorf("resolve repository Git exclude path: %w", err)
	}
	excludePath := strings.TrimSpace(excludeOutput)
	if excludePath == "" {
		return InitResult{}, fmt.Errorf("git rev-parse --git-path info/exclude returned empty path")
	}
	if !filepath.IsAbs(excludePath) {
		excludePath = filepath.Join(repo.Primary.Path, excludePath)
	}
	if err := initsetup.EnsureExclude(excludePath, "/.config/"); err != nil {
		return InitResult{}, err
	}

	return InitResult{HookPath: hookPath}, nil
}
