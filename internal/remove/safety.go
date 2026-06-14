package remove

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"wg/internal/git"
)

type Proof struct {
	Method string
}

func IsIntegrated(ctx context.Context, runner git.Runner, repoPath string, branch string, target string) (Proof, bool, error) {
	result, err := runner.Run(ctx, repoPath, "merge-base", "--is-ancestor", branch, target)
	if err != nil {
		return Proof{}, false, err
	}
	if result.ExitCode == 0 {
		return Proof{Method: "ancestry"}, true, nil
	}
	if result.ExitCode > 1 {
		return Proof{}, false, nil
	}

	result, err = runner.Run(ctx, repoPath, "diff", "--quiet", target, branch, "--")
	if err != nil {
		return Proof{}, false, err
	}
	if result.ExitCode == 0 {
		return Proof{Method: "content"}, true, nil
	}
	if result.ExitCode > 1 {
		return Proof{}, false, nil
	}

	result, err = runner.Run(ctx, repoPath, "cherry", "-v", target, branch)
	if err != nil {
		return Proof{}, false, err
	}
	if result.ExitCode == 0 && cherryShowsApplied(result.Stdout) {
		return Proof{Method: "cherry"}, true, nil
	}

	if ok, err := cumulativePatchIDEquivalent(ctx, runner, repoPath, branch, target); err != nil {
		return Proof{}, false, err
	} else if ok {
		return Proof{Method: "patch-id"}, true, nil
	}

	return Proof{}, false, nil
}

func cherryShowsApplied(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "-") {
			return false
		}
	}
	return strings.TrimSpace(output) != ""
}

func cumulativePatchIDEquivalent(ctx context.Context, runner git.Runner, repoPath string, branch string, target string) (bool, error) {
	mergeBaseOutput, err := git.Output(ctx, runner, repoPath, "merge-base", branch, target)
	if err != nil {
		return false, nil
	}
	mergeBase := strings.TrimSpace(mergeBaseOutput)
	if mergeBase == "" {
		return false, nil
	}

	branchID, err := cumulativePatchID(ctx, runner, repoPath, mergeBase, branch)
	if err != nil || branchID == "" {
		return false, err
	}
	targetIDs, err := patchIDsForRange(ctx, runner, repoPath, mergeBase, target)
	if err != nil {
		return false, err
	}
	_, ok := targetIDs[branchID]
	return ok, nil
}

func cumulativePatchID(ctx context.Context, runner git.Runner, repoPath string, base string, ref string) (string, error) {
	patch, err := git.Output(ctx, runner, repoPath, "diff", base, ref, "--")
	if err != nil {
		return "", nil
	}
	return stablePatchID(ctx, runner, repoPath, patch)
}

func patchIDsForRange(ctx context.Context, runner git.Runner, repoPath string, base string, ref string) (map[string]struct{}, error) {
	output, err := git.Output(ctx, runner, repoPath, "log", "--format=%H", "--reverse", base+".."+ref)
	if err != nil {
		return nil, nil
	}
	ids := make(map[string]struct{})
	for _, commit := range strings.Fields(output) {
		patch, err := git.Output(ctx, runner, repoPath, "show", "--format=", "--patch", commit)
		if err != nil {
			return nil, err
		}
		id, err := stablePatchID(ctx, runner, repoPath, patch)
		if err != nil {
			return nil, err
		}
		if id != "" {
			ids[id] = struct{}{}
		}
	}
	return ids, nil
}

func stablePatchID(ctx context.Context, runner git.Runner, repoPath string, patch string) (string, error) {
	var stdout, stderr bytes.Buffer
	if err := git.RunWithInput(ctx, runner, repoPath, strings.NewReader(patch), &stdout, &stderr, "patch-id", "--stable"); err != nil {
		return "", fmt.Errorf("git patch-id --stable failed: %w", err)
	}
	fields := strings.Fields(stdout.String())
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}
