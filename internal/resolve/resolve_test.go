package resolve_test

import (
	"errors"
	"testing"

	"wg/internal/resolve"
	"wg/internal/worktree"
)

func TestResolveExactMatchesAcrossConcepts(t *testing.T) {
	entries := resolverEntries()
	for _, query := range []string{"feature-alpha", "alpha", "demo.alpha"} {
		t.Run(query, func(t *testing.T) {
			entry, err := resolve.Resolve(entries, query)
			if err != nil {
				t.Fatalf("Resolve returned error: %v", err)
			}
			if entry.Path != "/repo/demo.alpha" {
				t.Fatalf("expected alpha entry, got %#v", entry)
			}
		})
	}
}

func TestResolveExactMatchWinsOverPrefix(t *testing.T) {
	entries := append(resolverEntries(), worktree.Entry{
		Path: "/repo/demo.feature-alpha-long", Branch: "feature-alpha-long", DisplayName: "feature-alpha-long", PathBasename: "demo.feature-alpha-long",
	})
	entry, err := resolve.Resolve(entries, "feature-alpha")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if entry.Path != "/repo/demo.alpha" {
		t.Fatalf("expected exact alpha entry, got %#v", entry)
	}
}

func TestResolveUniquePrefix(t *testing.T) {
	entry, err := resolve.Resolve(resolverEntries(), "bet")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if entry.Path != "/repo/demo.beta" {
		t.Fatalf("expected beta entry, got %#v", entry)
	}
}

func TestResolveMissingNameError(t *testing.T) {
	_, err := resolve.Resolve(resolverEntries(), "missing")
	var missing resolve.MissingError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingError, got %T %[1]v", err)
	}
	if missing.Query != "missing" {
		t.Fatalf("expected query to be carried, got %#v", missing)
	}
}

func TestResolveAmbiguousPrefixErrorCarriesCandidateNames(t *testing.T) {
	_, err := resolve.Resolve(resolverEntries(), "feature")
	var ambiguous resolve.AmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected AmbiguousError, got %T %[1]v", err)
	}
	if len(ambiguous.Candidates) != 2 || ambiguous.Candidates[0] != "alpha" || ambiguous.Candidates[1] != "beta" {
		t.Fatalf("unexpected candidates: %#v", ambiguous.Candidates)
	}
}

func TestResolveDoesNotTreatDuplicateConceptsForOneWorktreeAsAmbiguous(t *testing.T) {
	entries := []worktree.Entry{{
		Path: "/repo/demo.feature", Branch: "feature", DisplayName: "feature", PathBasename: "feature",
	}}
	entry, err := resolve.Resolve(entries, "feat")
	if err != nil {
		t.Fatalf("expected duplicate concepts on one entry to resolve, got %v", err)
	}
	if entry.Path != "/repo/demo.feature" {
		t.Fatalf("expected feature entry, got %#v", entry)
	}
}

func resolverEntries() []worktree.Entry {
	return []worktree.Entry{
		{Path: "/repo/demo", Branch: "main", DisplayName: "main", PathBasename: "demo"},
		{Path: "/repo/demo.alpha", Branch: "feature-alpha", DisplayName: "alpha", PathBasename: "demo.alpha"},
		{Path: "/repo/demo.beta", Branch: "feature-beta", DisplayName: "beta", PathBasename: "demo.beta"},
	}
}
