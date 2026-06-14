package worktree

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"wg/internal/ports"
)

type SetupContext struct {
	Branch              string
	WorktreePath        string
	WorktreeName        string
	Repo                string
	PrimaryWorktreePath string
	DefaultBranch       string
	Base                string
	Port                int
	ScriptPath          string
	BaseEnv             []string
}

type SetupResult struct {
	Ran        bool
	ScriptPath string
	ExitCode   int
}

func BuildSetupContext(plan NewPlan) SetupContext {
	return SetupContext{
		Branch:              plan.Branch,
		WorktreePath:        plan.WorktreePath,
		WorktreeName:        plan.WorktreeName,
		Repo:                filepath.Base(plan.Repository.Primary.Path),
		PrimaryWorktreePath: plan.Repository.Primary.Path,
		DefaultBranch:       plan.DefaultBranch.Name,
		Base:                plan.Base,
		Port:                ports.DerivePort(plan.WorktreePath),
		ScriptPath:          filepath.Join(plan.Repository.Primary.Path, ".config", "setup.sh"),
	}
}

func (c SetupContext) Env(base []string) []string {
	env := make([]string, 0, len(base)+9)
	for _, value := range base {
		if !isWGEnv(value) {
			env = append(env, value)
		}
	}
	env = append(env,
		"WG_BRANCH="+c.Branch,
		"WG_WORKTREE_PATH="+c.WorktreePath,
		"WG_WORKTREE_NAME="+c.WorktreeName,
		"WG_REPO="+c.Repo,
		"WG_PRIMARY_WORKTREE_PATH="+c.PrimaryWorktreePath,
		"WG_DEFAULT_BRANCH="+c.DefaultBranch,
		"WG_BASE="+c.Base,
		fmt.Sprintf("WG_PORT=%d", c.Port),
	)
	return env
}

func isWGEnv(value string) bool {
	return len(value) > 3 && value[:3] == "WG_"
}

func RunSetup(ctx context.Context, setup SetupContext, stderr io.Writer) (SetupResult, error) {
	if stderr == nil {
		stderr = io.Discard
	}
	if setup.ScriptPath == "" {
		setup.ScriptPath = filepath.Join(setup.PrimaryWorktreePath, ".config", "setup.sh")
	}
	if _, err := os.Stat(setup.ScriptPath); err != nil {
		if os.IsNotExist(err) {
			return SetupResult{Ran: false, ScriptPath: setup.ScriptPath}, nil
		}
		return SetupResult{}, err
	}

	cmd := exec.CommandContext(ctx, setup.ScriptPath)
	cmd.Dir = setup.WorktreePath
	baseEnv := setup.BaseEnv
	if baseEnv == nil {
		baseEnv = os.Environ()
	}
	cmd.Env = setup.Env(baseEnv)
	cmd.Stdout = stderr
	cmd.Stderr = stderr

	result := SetupResult{Ran: true, ScriptPath: setup.ScriptPath, ExitCode: 0}
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			return result, fmt.Errorf("setup script %s failed with exit code %d for branch %s at %s", setup.ScriptPath, result.ExitCode, setup.Branch, setup.WorktreePath)
		}
		return result, fmt.Errorf("setup script %s failed for branch %s at %s: %w", setup.ScriptPath, setup.Branch, setup.WorktreePath, err)
	}
	return result, nil
}
