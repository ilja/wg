package worktree

import (
	"context"
	"fmt"
	"strings"

	"wg/internal/git"
)

type DefaultBranch struct {
	Name   string
	Source DefaultBranchSource
}

type DefaultBranchSource string

const (
	DefaultBranchSourceExplicit    DefaultBranchSource = "explicit"
	DefaultBranchSourceOriginHEAD  DefaultBranchSource = "origin-head"
	DefaultBranchSourceRemoteHEAD  DefaultBranchSource = "remote-head"
	DefaultBranchSourcePrimary     DefaultBranchSource = "primary"
	DefaultBranchSourceLocalBranch DefaultBranchSource = "local-branch"
)

type AmbiguousDefaultBranchError struct {
	Branches []string
}

func (e AmbiguousDefaultBranchError) Error() string {
	return fmt.Sprintf("ambiguous default branch; found %s, pass an explicit base", strings.Join(e.Branches, " and "))
}

type MissingDefaultBranchError struct{}

func (e MissingDefaultBranchError) Error() string {
	return "could not resolve default branch; pass an explicit base"
}

func ResolveDefaultBranch(ctx context.Context, runner git.Runner, repo Repository, explicitBase string) (DefaultBranch, error) {
	if explicitBase != "" {
		result, err := runner.Run(ctx, repo.Primary.Path, "rev-parse", "--verify", "--quiet", explicitBase+"^{commit}")
		if err != nil {
			return DefaultBranch{}, err
		}
		if result.ExitCode != 0 {
			return DefaultBranch{}, fmt.Errorf("base %q is not a commit", explicitBase)
		}
		return DefaultBranch{Name: explicitBase, Source: DefaultBranchSourceExplicit}, nil
	}

	if name, ok, err := localOriginHEAD(ctx, runner, repo); err != nil {
		return DefaultBranch{}, err
	} else if ok {
		return DefaultBranch{Name: name, Source: DefaultBranchSourceOriginHEAD}, nil
	}

	if name, ok, err := remoteHEAD(ctx, runner, repo); err != nil {
		return DefaultBranch{}, err
	} else if ok {
		return DefaultBranch{Name: name, Source: DefaultBranchSourceRemoteHEAD}, nil
	}

	if repo.Primary.Branch == "main" || repo.Primary.Branch == "master" {
		return DefaultBranch{Name: repo.Primary.Branch, Source: DefaultBranchSourcePrimary}, nil
	}

	hasMain, err := localBranchExists(ctx, runner, repo, "main")
	if err != nil {
		return DefaultBranch{}, err
	}
	hasMaster, err := localBranchExists(ctx, runner, repo, "master")
	if err != nil {
		return DefaultBranch{}, err
	}

	switch {
	case hasMain && hasMaster:
		return DefaultBranch{}, AmbiguousDefaultBranchError{Branches: []string{"main", "master"}}
	case hasMain:
		return DefaultBranch{Name: "main", Source: DefaultBranchSourceLocalBranch}, nil
	case hasMaster:
		return DefaultBranch{Name: "master", Source: DefaultBranchSourceLocalBranch}, nil
	default:
		return DefaultBranch{}, MissingDefaultBranchError{}
	}
}

func localOriginHEAD(ctx context.Context, runner git.Runner, repo Repository) (string, bool, error) {
	result, err := runner.Run(ctx, repo.Primary.Path, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", false, err
	}
	if result.ExitCode != 0 {
		return "", false, nil
	}
	name := strings.TrimSpace(result.Stdout)
	if name == "" {
		return "", false, nil
	}
	return name, true, nil
}

func remoteHEAD(ctx context.Context, runner git.Runner, repo Repository) (string, bool, error) {
	result, err := runner.Run(ctx, repo.Primary.Path, "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return "", false, err
	}
	if result.ExitCode != 0 {
		return "", false, nil
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		if !strings.HasPrefix(line, "ref: ") || !strings.Contains(line, "\tHEAD") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ref := strings.TrimPrefix(fields[1], "refs/heads/")
		if ref != "" {
			return "origin/" + ref, true, nil
		}
	}
	return "", false, nil
}

func localBranchExists(ctx context.Context, runner git.Runner, repo Repository, branch string) (bool, error) {
	result, err := runner.Run(ctx, repo.Primary.Path, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		return false, err
	}
	return result.ExitCode == 0, nil
}
