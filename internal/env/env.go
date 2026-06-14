package env

import (
	"fmt"
	"path/filepath"

	"wg/internal/ports"
	"wg/internal/worktree"
)

type Context struct {
	Branch              string
	WorktreePath        string
	WorktreeName        string
	Repo                string
	PrimaryWorktreePath string
	DefaultBranch       string
	Base                string
	Port                int
}

func DerivePort(path string) int {
	return ports.DerivePort(path)
}

func BuildContext(repo worktree.Repository, target worktree.Entry, defaultBranch string, base string) Context {
	worktreeName := target.DisplayName
	if worktreeName == "" {
		worktreeName = target.PathBasename
	}
	if worktreeName == "" {
		worktreeName = filepath.Base(target.Path)
	}

	repoPath := repo.Primary.Path
	return Context{
		Branch:              target.Branch,
		WorktreePath:        target.Path,
		WorktreeName:        worktreeName,
		Repo:                filepath.Base(repoPath),
		PrimaryWorktreePath: repoPath,
		DefaultBranch:       defaultBranch,
		Base:                base,
		Port:                DerivePort(target.Path),
	}
}

func Render(ctx Context) []string {
	return []string{
		"WG_BRANCH=" + ctx.Branch,
		"WG_WORKTREE_PATH=" + ctx.WorktreePath,
		"WG_WORKTREE_NAME=" + ctx.WorktreeName,
		"WG_REPO=" + ctx.Repo,
		"WG_PRIMARY_WORKTREE_PATH=" + ctx.PrimaryWorktreePath,
		"WG_DEFAULT_BRANCH=" + ctx.DefaultBranch,
		"WG_BASE=" + ctx.Base,
		fmt.Sprintf("WG_PORT=%d", ctx.Port),
	}
}
