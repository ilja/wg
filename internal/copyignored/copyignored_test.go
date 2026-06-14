package copyignored

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type fakeInspector struct {
	tracked         map[string]struct{}
	ignored         map[string]struct{}
	nestedWorktrees map[string]struct{}
}

func (f fakeInspector) TrackedPaths(ctx context.Context, root string, rels []string) (map[string]struct{}, error) {
	return subset(f.tracked, rels), nil
}

func (f fakeInspector) IgnoredPaths(ctx context.Context, root string, rels []string) (map[string]struct{}, error) {
	return subset(f.ignored, rels), nil
}

func (f fakeInspector) NestedWorktreePaths(ctx context.Context, root string) (map[string]struct{}, error) {
	out := make(map[string]struct{}, len(f.nestedWorktrees))
	for rel := range f.nestedWorktrees {
		out[filepath.Join(root, filepath.FromSlash(rel))] = struct{}{}
	}
	return out, nil
}

func TestBuildPlanMissingAndEmptyIncludeNoop(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()

	plan, err := BuildPlan(context.Background(), fakeInspector{}, PlanOptions{
		SourceRoot: source, DestinationRoot: dest, IncludeFile: filepath.Join(source, ".worktreeinclude"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 0 || plan.NoopReason == "" {
		t.Fatalf("expected missing include no-op, got %#v", plan)
	}

	writeFile(t, filepath.Join(source, ".worktreeinclude"), "# comments\n\n")
	plan, err = BuildPlan(context.Background(), fakeInspector{}, PlanOptions{
		SourceRoot: source, DestinationRoot: dest, IncludeFile: filepath.Join(source, ".worktreeinclude"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 0 || plan.NoopReason == "" {
		t.Fatalf("expected empty include no-op, got %#v", plan)
	}
}

func TestBuildPlanMatchesAllowlistStylesAndIgnoredOnly(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	writeFile(t, filepath.Join(source, ".worktreeinclude"), ".env\nnode_modules/\nconfig/credentials/*.key\n**/pulse.css\n")
	writeFile(t, filepath.Join(source, ".env"), "env\n")
	writeFile(t, filepath.Join(source, "node_modules", "pkg", "index.js"), "pkg\n")
	writeFile(t, filepath.Join(source, "config", "credentials", "development.key"), "key\n")
	writeFile(t, filepath.Join(source, "config", "credentials", "development.yml.enc"), "not matched\n")
	writeFile(t, filepath.Join(source, "pulse.css"), "root pulse\n")
	writeFile(t, filepath.Join(source, "app", "assets", "pulse.css"), "nested pulse\n")
	writeFile(t, filepath.Join(source, "README.md"), "tracked\n")
	writeFile(t, filepath.Join(source, ".env.local"), "not allowlisted\n")

	plan, err := BuildPlan(context.Background(), fakeInspector{
		tracked: map[string]struct{}{"README.md": {}},
		ignored: map[string]struct{}{
			".env":                               {},
			"node_modules/pkg/index.js":          {},
			"config/credentials/development.key": {},
			"pulse.css":                          {},
			"app/assets/pulse.css":               {},
			".env.local":                         {},
		},
	}, PlanOptions{SourceRoot: source, DestinationRoot: dest, IncludeFile: filepath.Join(source, ".worktreeinclude")})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryRels(plan), []string{".env", "app/assets/pulse.css", "config/credentials/development.key", "node_modules/pkg/index.js", "pulse.css"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("planned rels mismatch\nwant %#v\n got %#v", want, got)
	}
}

func TestBuildPlanExcludesTrackedAndNonIgnoredFiles(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	writeFile(t, filepath.Join(source, ".worktreeinclude"), ".env\nREADME.md\nlocal.txt\n")
	writeFile(t, filepath.Join(source, ".env"), "env\n")
	writeFile(t, filepath.Join(source, "README.md"), "tracked\n")
	writeFile(t, filepath.Join(source, "local.txt"), "not ignored\n")

	plan, err := BuildPlan(context.Background(), fakeInspector{
		tracked: map[string]struct{}{"README.md": {}},
		ignored: map[string]struct{}{".env": {}},
	}, PlanOptions{SourceRoot: source, DestinationRoot: dest, IncludeFile: filepath.Join(source, ".worktreeinclude")})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryRels(plan), []string{".env"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("planned rels mismatch\nwant %#v\n got %#v", want, got)
	}
}

func TestBuildPlanSkipExistingAndForceOverwrite(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	writeFile(t, filepath.Join(source, ".worktreeinclude"), ".env\n")
	writeFile(t, filepath.Join(source, ".env"), "source\n")
	writeFile(t, filepath.Join(dest, ".env"), "dest\n")

	plan, err := BuildPlan(context.Background(), fakeInspector{ignored: map[string]struct{}{".env": {}}}, PlanOptions{
		SourceRoot: source, DestinationRoot: dest, IncludeFile: filepath.Join(source, ".worktreeinclude"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 1 || plan.Entries[0].Action != ActionSkipExisting {
		t.Fatalf("expected skip existing entry, got %#v", plan.Entries)
	}

	plan, err = BuildPlan(context.Background(), fakeInspector{ignored: map[string]struct{}{".env": {}}}, PlanOptions{
		SourceRoot: source, DestinationRoot: dest, IncludeFile: filepath.Join(source, ".worktreeinclude"), Force: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 1 || plan.Entries[0].Action != ActionCopy {
		t.Fatalf("expected force copy entry, got %#v", plan.Entries)
	}
}

func TestBuildPlanBroadPatternExcludesUnsafeGitFile(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	writeFile(t, filepath.Join(source, ".worktreeinclude"), "**\n")
	writeFile(t, filepath.Join(source, ".git"), "gitdir: ../demo.git/worktrees/linked\n")
	writeFile(t, filepath.Join(source, ".env"), "env\n")

	plan, err := BuildPlan(context.Background(), fakeInspector{
		ignored: map[string]struct{}{".git": {}, ".env": {}},
	}, PlanOptions{SourceRoot: source, DestinationRoot: dest, IncludeFile: filepath.Join(source, ".worktreeinclude")})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryRels(plan), []string{".env"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("planned rels mismatch\nwant %#v\n got %#v", want, got)
	}
}

func TestBuildPlanBroadPatternsExcludeUnsafeAndNestedWorktrees(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	writeFile(t, filepath.Join(source, ".worktreeinclude"), "**\n")
	writeFile(t, filepath.Join(source, ".env"), "env\n")
	unsafe := []string{".git/config", ".config/state", ".plans/story", ".wiki/page", ".pi/session", ".context/local", "nested-worktree/local"}
	for _, rel := range unsafe {
		writeFile(t, filepath.Join(source, filepath.FromSlash(rel)), "unsafe\n")
	}

	ignored := map[string]struct{}{".env": {}}
	for _, rel := range unsafe {
		ignored[rel] = struct{}{}
	}
	plan, err := BuildPlan(context.Background(), fakeInspector{
		ignored:         ignored,
		nestedWorktrees: map[string]struct{}{"nested-worktree": {}},
	}, PlanOptions{SourceRoot: source, DestinationRoot: dest, IncludeFile: filepath.Join(source, ".worktreeinclude")})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryRels(plan), []string{".env"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("planned rels mismatch\nwant %#v\n got %#v", want, got)
	}
}

func TestApplyCopiesOnlyCopyEntriesAndRepeatPlanSkipsExisting(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	writeFile(t, filepath.Join(source, ".worktreeinclude"), ".env\n")
	writeFile(t, filepath.Join(source, ".env"), "source\n")
	inspector := fakeInspector{ignored: map[string]struct{}{".env": {}}}

	plan, err := BuildPlan(context.Background(), inspector, PlanOptions{SourceRoot: source, DestinationRoot: dest, IncludeFile: filepath.Join(source, ".worktreeinclude")})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(plan, func(src string, dst string, mode fs.FileMode) error {
		content, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, content, mode)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Copied != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected apply result %#v", result)
	}
	assertContent(t, filepath.Join(dest, ".env"), "source\n")

	writeFile(t, filepath.Join(source, ".env"), "changed\n")
	plan, err = BuildPlan(context.Background(), inspector, PlanOptions{SourceRoot: source, DestinationRoot: dest, IncludeFile: filepath.Join(source, ".worktreeinclude")})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 1 || plan.Entries[0].Action != ActionSkipExisting {
		t.Fatalf("expected second plan to skip existing, got %#v", plan.Entries)
	}
	result, err = Apply(plan, func(src string, dst string, mode fs.FileMode) error {
		t.Fatalf("copy function should not be called for skipped entry")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Copied != 0 || result.Skipped != 1 {
		t.Fatalf("unexpected second apply result %#v", result)
	}
	assertContent(t, filepath.Join(dest, ".env"), "source\n")
}

func subset(values map[string]struct{}, rels []string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, rel := range rels {
		if _, ok := values[rel]; ok {
			out[rel] = struct{}{}
		}
	}
	return out
}

func entryRels(plan Plan) []string {
	rels := make([]string, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		rels = append(rels, entry.RelPath)
	}
	return rels
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s content mismatch: want %q got %q", path, want, content)
	}
}
