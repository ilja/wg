package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wg/internal/copier"
	"wg/internal/copyignored"
	"wg/internal/git"
	"wg/internal/resolve"
	"wg/internal/worktree"
)

type App struct {
	Cwd       string
	Environ   []string
	GitRunner git.Runner
}

type CopyIgnoredOptions struct {
	From   string
	To     string
	Force  bool
	DryRun bool
}

type CopyIgnoredResult struct {
	Plan   copyignored.Plan
	Result copyignored.Result
}

type gitInspector interface {
	git.Runner
	copyignored.GitInspector
}

func (a *App) CopyIgnored(ctx context.Context, opts CopyIgnoredOptions) (CopyIgnoredResult, error) {
	runner := a.runner()
	inspector, ok := runner.(gitInspector)
	if !ok {
		return CopyIgnoredResult{}, fmt.Errorf("git runner does not support copy inspection")
	}

	repo, err := worktree.LoadRepository(ctx, runner, a.cwd())
	if err != nil {
		return CopyIgnoredResult{}, err
	}

	source := repo.Primary
	if opts.From != "" {
		source, err = resolve.Resolve(repo.Entries, opts.From)
		if err != nil {
			return CopyIgnoredResult{}, err
		}
	}
	destination, err := a.destination(repo, opts.To)
	if err != nil {
		return CopyIgnoredResult{}, err
	}

	plan, err := copyignored.BuildPlan(ctx, inspector, copyignored.PlanOptions{
		SourceRoot:      source.Path,
		DestinationRoot: destination,
		IncludeFile:     filepath.Join(source.Path, ".worktreeinclude"),
		Force:           opts.Force,
	})
	if err != nil {
		return CopyIgnoredResult{}, err
	}
	result := CopyIgnoredResult{Plan: plan}
	if opts.DryRun {
		return result, nil
	}
	applyResult, err := copyignored.Apply(plan, copier.CopyFile)
	result.Result = applyResult
	return result, err
}

func (a *App) destination(repo worktree.Repository, to string) (string, error) {
	if to != "" {
		entry, err := resolve.Resolve(repo.Entries, to)
		if err != nil {
			return "", err
		}
		return entry.Path, nil
	}
	if path := envValue(a.environ(), "WG_WORKTREE_PATH"); path != "" {
		return filepath.Clean(path), nil
	}
	for _, entry := range repo.Entries {
		if entry.IsCurrent {
			return entry.Path, nil
		}
	}
	return "", fmt.Errorf("current worktree %q not found in git worktree list", repo.CurrentRoot)
}

func (a *App) runner() git.Runner {
	if a.GitRunner != nil {
		return a.GitRunner
	}
	return git.ExecRunner{Binary: "git"}
}

func (a *App) cwd() string {
	if a.Cwd != "" {
		return a.Cwd
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func (a *App) environ() []string {
	if a.Environ != nil {
		return a.Environ
	}
	return os.Environ()
}

func envValue(environ []string, key string) string {
	prefix := key + "="
	for _, value := range environ {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return ""
}
