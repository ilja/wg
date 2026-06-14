package copyignored

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Action string

const (
	ActionCopy         Action = "copy"
	ActionSkipExisting Action = "skip"
)

type PlanOptions struct {
	SourceRoot      string
	DestinationRoot string
	IncludeFile     string
	Force           bool
}

type Entry struct {
	RelPath         string
	SourcePath      string
	DestinationPath string
	Mode            fs.FileMode
	Action          Action
}

type Plan struct {
	SourceRoot      string
	DestinationRoot string
	IncludeFile     string
	Entries         []Entry
	NoopReason      string
}

type Result struct {
	Copied  int
	Skipped int
}

type GitInspector interface {
	TrackedPaths(ctx context.Context, root string, rels []string) (map[string]struct{}, error)
	IgnoredPaths(ctx context.Context, root string, rels []string) (map[string]struct{}, error)
	NestedWorktreePaths(ctx context.Context, root string) (map[string]struct{}, error)
}

func LoadPatterns(includeFile string) ([]string, error) {
	content, err := os.ReadFile(includeFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var patterns []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, normalizePattern(line))
	}
	return patterns, nil
}

func BuildPlan(ctx context.Context, git GitInspector, opts PlanOptions) (Plan, error) {
	sourceRoot := filepath.Clean(opts.SourceRoot)
	destinationRoot := filepath.Clean(opts.DestinationRoot)
	includeFile := opts.IncludeFile
	if includeFile == "" {
		includeFile = filepath.Join(sourceRoot, ".worktreeinclude")
	}

	plan := Plan{SourceRoot: sourceRoot, DestinationRoot: destinationRoot, IncludeFile: includeFile}
	patterns, err := LoadPatterns(includeFile)
	if err != nil {
		return Plan{}, err
	}
	if len(patterns) == 0 {
		if _, err := os.Stat(includeFile); os.IsNotExist(err) {
			plan.NoopReason = "no .worktreeinclude found"
		} else {
			plan.NoopReason = "no .worktreeinclude patterns"
		}
		return plan, nil
	}

	nestedRoots, err := git.NestedWorktreePaths(ctx, sourceRoot)
	if err != nil {
		return Plan{}, err
	}

	var candidates []string
	if err := filepath.WalkDir(sourceRoot, func(current string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if current == sourceRoot {
			return nil
		}

		rel, err := filepath.Rel(sourceRoot, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if isUnsafeDir(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if isNestedWorktreeRoot(filepath.Clean(current), nestedRoots) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if matchesAny(patterns, rel) {
			candidates = append(candidates, rel)
		}
		return nil
	}); err != nil {
		return Plan{}, err
	}
	sort.Strings(candidates)

	if len(candidates) == 0 {
		plan.NoopReason = "no allowlisted files to copy"
		return plan, nil
	}

	tracked, err := git.TrackedPaths(ctx, sourceRoot, candidates)
	if err != nil {
		return Plan{}, err
	}
	ignored, err := git.IgnoredPaths(ctx, sourceRoot, candidates)
	if err != nil {
		return Plan{}, err
	}

	for _, rel := range candidates {
		if _, ok := tracked[rel]; ok {
			continue
		}
		if _, ok := ignored[rel]; !ok {
			continue
		}

		sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(rel))
		info, err := os.Stat(sourcePath)
		if err != nil {
			return Plan{}, err
		}
		destinationPath := filepath.Join(destinationRoot, filepath.FromSlash(rel))
		action := ActionCopy
		if _, err := os.Stat(destinationPath); err == nil && !opts.Force {
			action = ActionSkipExisting
		} else if err != nil && !os.IsNotExist(err) {
			return Plan{}, err
		}
		plan.Entries = append(plan.Entries, Entry{
			RelPath:         rel,
			SourcePath:      sourcePath,
			DestinationPath: destinationPath,
			Mode:            info.Mode().Perm(),
			Action:          action,
		})
	}
	if len(plan.Entries) == 0 {
		plan.NoopReason = "no ignored allowlisted files to copy"
	}
	return plan, nil
}

func Apply(plan Plan, copyFile func(src string, dst string, mode fs.FileMode) error) (Result, error) {
	var result Result
	for _, entry := range plan.Entries {
		switch entry.Action {
		case ActionSkipExisting:
			result.Skipped++
		case ActionCopy:
			if err := os.MkdirAll(filepath.Dir(entry.DestinationPath), 0o755); err != nil {
				return result, err
			}
			if err := copyFile(entry.SourcePath, entry.DestinationPath, entry.Mode); err != nil {
				return result, err
			}
			result.Copied++
		default:
			return result, fmt.Errorf("unknown copy action %q for %s", entry.Action, entry.RelPath)
		}
	}
	return result, nil
}

func normalizePattern(pattern string) string {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	pattern = strings.TrimPrefix(pattern, "./")
	pattern = strings.TrimPrefix(pattern, "/")
	return pattern
}

func matchesAny(patterns []string, rel string) bool {
	for _, pattern := range patterns {
		if matchesPattern(pattern, rel) {
			return true
		}
	}
	return false
}

func matchesPattern(pattern string, rel string) bool {
	pattern = normalizePattern(pattern)
	rel = path.Clean(filepath.ToSlash(rel))
	if strings.HasSuffix(pattern, "/") {
		prefix := strings.TrimSuffix(pattern, "/")
		return rel == prefix || strings.HasPrefix(rel, prefix+"/")
	}
	if !strings.ContainsAny(pattern, "*?") {
		return rel == pattern
	}
	matched, err := regexp.MatchString(globRegexp(pattern), rel)
	return err == nil && matched
}

func globRegexp(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch ch {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					b.WriteString("(?:.*/)?")
					i += 2
				} else {
					b.WriteString(".*")
					i++
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	b.WriteString("$")
	return b.String()
}

func isUnsafeDir(name string) bool {
	switch name {
	case ".git", ".config", ".plans", ".wiki", ".pi", ".context":
		return true
	default:
		return false
	}
}

func isNestedWorktreeRoot(path string, nestedRoots map[string]struct{}) bool {
	_, ok := nestedRoots[path]
	return ok
}
