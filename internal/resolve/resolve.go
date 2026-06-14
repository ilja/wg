package resolve

import (
	"fmt"
	"strings"

	"wg/internal/worktree"
)

type MissingError struct {
	Query      string
	Candidates []string
}

func (e MissingError) Error() string {
	return fmt.Sprintf("worktree %q not found", e.Query)
}

type AmbiguousError struct {
	Query      string
	Candidates []string
}

func (e AmbiguousError) Error() string {
	return fmt.Sprintf("worktree %q is ambiguous: %s", e.Query, strings.Join(e.Candidates, ", "))
}

func Resolve(entries []worktree.Entry, query string) (worktree.Entry, error) {
	exact := matchingEntries(entries, query, exactMatch)
	if len(exact) == 1 {
		return entries[exact[0]], nil
	}
	if len(exact) > 1 {
		return worktree.Entry{}, AmbiguousError{Query: query, Candidates: candidateNames(entries, exact)}
	}

	prefix := matchingEntries(entries, query, prefixMatch)
	if len(prefix) == 1 {
		return entries[prefix[0]], nil
	}
	if len(prefix) > 1 {
		return worktree.Entry{}, AmbiguousError{Query: query, Candidates: candidateNames(entries, prefix)}
	}

	return worktree.Entry{}, MissingError{Query: query, Candidates: candidateNames(entries, allIndexes(entries))}
}

type matcher func(concept string, query string) bool

func matchingEntries(entries []worktree.Entry, query string, match matcher) []int {
	seen := make(map[int]struct{})
	var indexes []int
	for i, entry := range entries {
		for _, concept := range concepts(entry) {
			if concept == "" || !match(concept, query) {
				continue
			}
			if _, ok := seen[i]; ok {
				continue
			}
			seen[i] = struct{}{}
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func exactMatch(concept string, query string) bool {
	return concept == query
}

func prefixMatch(concept string, query string) bool {
	return strings.HasPrefix(concept, query)
}

func concepts(entry worktree.Entry) []string {
	return []string{entry.Branch, entry.DisplayName, entry.PathBasename}
}

func candidateNames(entries []worktree.Entry, indexes []int) []string {
	names := make([]string, 0, len(indexes))
	for _, index := range indexes {
		entry := entries[index]
		switch {
		case entry.DisplayName != "":
			names = append(names, entry.DisplayName)
		case entry.Branch != "":
			names = append(names, entry.Branch)
		case entry.PathBasename != "":
			names = append(names, entry.PathBasename)
		default:
			names = append(names, entry.Path)
		}
	}
	return names
}

func allIndexes(entries []worktree.Entry) []int {
	indexes := make([]int, 0, len(entries))
	for i := range entries {
		indexes = append(indexes, i)
	}
	return indexes
}
