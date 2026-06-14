package worktree

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"wg/internal/git"
)

type Entry struct {
	Path         string
	Head         string
	Branch       string
	IsDetached   bool
	IsBare       bool
	IsLocked     bool
	LockReason   string
	DisplayName  string
	PathBasename string
	IsCurrent    bool
	IsPrimary    bool
}

type Repository struct {
	CurrentRoot string
	Primary     Entry
	Entries     []Entry
}

func ParsePorcelain(output string) ([]Entry, error) {
	var entries []Entry
	var current *Entry

	flush := func() {
		if current == nil {
			return
		}
		deriveBasicNames(current)
		entries = append(entries, *current)
		current = nil
	}

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		if line == "" {
			flush()
			continue
		}

		key, value, hasValue := strings.Cut(line, " ")
		switch key {
		case "worktree":
			flush()
			if !hasValue || strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("invalid worktree record: %q", line)
			}
			current = &Entry{Path: filepath.Clean(value)}
		case "HEAD":
			if current != nil && hasValue {
				current.Head = value
			}
		case "branch":
			if current != nil && hasValue {
				current.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "detached":
			if current != nil {
				current.IsDetached = true
			}
		case "bare":
			if current != nil {
				current.IsBare = true
			}
		case "locked":
			if current != nil {
				current.IsLocked = true
				if hasValue {
					current.LockReason = value
				}
			}
		}
	}
	flush()

	return entries, nil
}

func LoadRepository(ctx context.Context, runner git.Runner, cwd string) (Repository, error) {
	rootResult, err := runner.Run(ctx, cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repository{}, err
	}
	if rootResult.ExitCode != 0 {
		return Repository{}, fmt.Errorf("git rev-parse --show-toplevel failed: %s", strings.TrimSpace(rootResult.Stderr))
	}

	currentRoot := strings.TrimSpace(rootResult.Stdout)
	if currentRoot == "" {
		return Repository{}, fmt.Errorf("git rev-parse --show-toplevel returned empty path")
	}
	if !filepath.IsAbs(currentRoot) {
		currentRoot = filepath.Join(cwd, currentRoot)
	}
	currentRoot = filepath.Clean(currentRoot)

	listResult, err := runner.Run(ctx, currentRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return Repository{}, err
	}
	if listResult.ExitCode != 0 {
		return Repository{}, fmt.Errorf("git worktree list --porcelain failed: %s", strings.TrimSpace(listResult.Stderr))
	}

	entries, err := ParsePorcelain(listResult.Stdout)
	if err != nil {
		return Repository{}, err
	}
	if len(entries) == 0 {
		return Repository{}, fmt.Errorf("git worktree list --porcelain returned no worktrees")
	}

	primaryPath := filepath.Clean(entries[0].Path)
	primaryName := filepath.Base(primaryPath)
	for i := range entries {
		entries[i].Path = filepath.Clean(entries[i].Path)
		entries[i].PathBasename = filepath.Base(entries[i].Path)
		entries[i].IsPrimary = i == 0
		entries[i].IsCurrent = entries[i].Path == currentRoot
		entries[i].DisplayName = displayName(entries[i], primaryName, primaryPath)
	}

	return Repository{
		CurrentRoot: currentRoot,
		Primary:     entries[0],
		Entries:     entries,
	}, nil
}

func deriveBasicNames(entry *Entry) {
	entry.PathBasename = filepath.Base(entry.Path)
	if entry.Branch != "" {
		entry.DisplayName = entry.Branch
		return
	}
	entry.DisplayName = entry.PathBasename
}

func displayName(entry Entry, primaryName string, primaryPath string) string {
	if entry.Path == primaryPath {
		if entry.Branch != "" {
			return entry.Branch
		}
		return entry.PathBasename
	}

	prefix := primaryName + "."
	if suffix, ok := strings.CutPrefix(entry.PathBasename, prefix); ok && suffix != "" {
		return suffix
	}
	if entry.Branch != "" {
		return entry.Branch
	}
	return entry.PathBasename
}
