package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"

	"wg/internal/app"
	"wg/internal/copyignored"
	wgenv "wg/internal/env"
	"wg/internal/git"
	"wg/internal/rebase"
	"wg/internal/remove"
	"wg/internal/resolve"
	"wg/internal/tui"
	"wg/internal/worktree"
)

type PickerFunc func(context.Context, []tui.PickerOption, io.Reader, io.Writer) (tui.PickerOption, error)

type Options struct {
	Cwd       string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Environ   []string
	GitRunner git.Runner
	Picker    PickerFunc
}

type rootCmd struct {
	List        listCmd        `cmd:"" help:"List worktrees."`
	Path        pathCmd        `cmd:"" help:"Print a worktree path."`
	Env         envCmd         `cmd:"" help:"Print worktree environment context."`
	Switch      SwitchCmd      `cmd:"" help:"Select a worktree."`
	New         NewCmd         `cmd:"" help:"Create a new branch/worktree from an explicit or resolved base. Usage: wg new <branch> [base]."`
	Rebase      RebaseCmd      `cmd:"" help:"Fetch and rebase the current worktree onto a base."`
	CopyIgnored CopyIgnoredCmd `cmd:"" name:"copy-ignored" help:"Copy allowlisted ignored files between worktrees."`
	Remove      RemoveCmd      `cmd:"" help:"Safely remove one integrated non-primary worktree."`
	Config      ConfigCmd      `cmd:"" help:"Print configuration helpers."`
}

type listCmd struct{}

type pathCmd struct {
	Name string `arg:"" name:"name" help:"Worktree name, branch, basename, or unique prefix."`
}

type envCmd struct {
	Name string `arg:"" optional:"" name:"name" help:"Optional worktree name, branch, basename, or unique prefix."`
}

type SwitchCmd struct {
	Reference  string `arg:"" optional:"" name:"reference" help:"Optional worktree name, branch, basename, or unique prefix."`
	PathOutput bool   `name:"path-output" help:"Print only the selected path."`
}

type NewCmd struct {
	Branch string `arg:"" name:"branch" help:"Branch to create."`
	Base   string `arg:"" optional:"" name:"base" help:"Optional base commit, branch, or ref."`
}

type RebaseCmd struct {
	Base string `arg:"" optional:"" name:"base" help:"Optional base branch, remote ref, or commit. Defaults to the repository default branch."`
}

type RemoveCmd struct {
	Name          string `arg:"" optional:"" name:"name" help:"Optional worktree name, branch, basename, or unique prefix. Defaults to the current worktree."`
	Force         bool   `short:"D" help:"Force removal of exactly one single named non-primary target."`
	PrintCdTarget bool   `name:"print-cd-target" hidden:"" help:"Print the primary path when removing the current worktree."`
}

type CopyIgnoredCmd struct {
	From   string `name:"from" help:"Source worktree name, branch, basename, or unique prefix."`
	To     string `name:"to" help:"Destination worktree name, branch, basename, or unique prefix."`
	Force  bool   `name:"force" help:"Overwrite existing destination files."`
	DryRun bool   `name:"dry-run" help:"Print planned copy actions without writing files."`
}

type runtime struct {
	ctx       context.Context
	cwd       string
	stdin     io.Reader
	stdout    io.Writer
	stderr    io.Writer
	environ   []string
	gitRunner git.Runner
	picker    PickerFunc
}

func Run(ctx context.Context, args []string, opts Options) int {
	rt, err := newRuntime(ctx, opts)
	if err != nil {
		_, _ = fmt.Fprintln(opts.Stderr, err)
		return 1
	}

	var root rootCmd
	parser, err := kong.New(&root,
		kong.Name("wg"),
		kong.Description("Fast Git worktree manager."),
		kong.Writers(rt.stdout, rt.stderr),
	)
	if err != nil {
		_, _ = fmt.Fprintln(rt.stderr, err)
		return 1
	}

	parsed, err := parser.Parse(args)
	if err != nil {
		_, _ = fmt.Fprintln(rt.stderr, err)
		return 2
	}

	if err := parsed.Run(rt); err != nil {
		writeDiagnostic(rt.stderr, err)
		return 1
	}
	return 0
}

func newRuntime(ctx context.Context, opts Options) (*runtime, error) {
	cwd := opts.Cwd
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	stdin := opts.Stdin
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	runner := opts.GitRunner
	if runner == nil {
		runner = git.ExecRunner{Binary: "git"}
	}
	picker := opts.Picker
	if picker == nil {
		picker = tui.RunPicker
	}

	return &runtime{
		ctx:       ctx,
		cwd:       cwd,
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		environ:   opts.Environ,
		gitRunner: runner,
		picker:    picker,
	}, nil
}

func (c *listCmd) Run(rt *runtime) error {
	repo, err := rt.loadRepository()
	if err != nil {
		return err
	}

	for _, entry := range repo.Entries {
		marker := " "
		if entry.IsCurrent {
			marker = "*"
		}
		_, _ = fmt.Fprintf(rt.stdout, "%s %s %s %s\n", marker, entry.DisplayName, entryState(entry), entry.Path)
	}
	return nil
}

func (c *pathCmd) Run(rt *runtime) error {
	repo, err := rt.loadRepository()
	if err != nil {
		return err
	}
	entry, err := resolve.Resolve(repo.Entries, c.Name)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(rt.stdout, entry.Path)
	return nil
}

func (c *envCmd) Run(rt *runtime) error {
	repo, err := rt.loadRepository()
	if err != nil {
		return err
	}

	var target worktree.Entry
	if c.Name == "" {
		var ok bool
		target, ok = currentEntry(repo.Entries)
		if !ok {
			return fmt.Errorf("current worktree %q not found in git worktree list", repo.CurrentRoot)
		}
	} else {
		target, err = resolve.Resolve(repo.Entries, c.Name)
		if err != nil {
			return err
		}
	}

	defaultBranch, err := worktree.ResolveDefaultBranch(rt.ctx, rt.gitRunner, repo, "")
	if err != nil {
		return err
	}

	for _, line := range wgenv.Render(wgenv.BuildContext(repo, target, defaultBranch.Name, "")) {
		_, _ = fmt.Fprintln(rt.stdout, line)
	}
	return nil
}

func (c *SwitchCmd) Run(rt *runtime) error {
	repo, err := rt.loadRepository()
	if err != nil {
		return err
	}

	var selected worktree.Entry
	if c.Reference != "" {
		selected, err = resolve.Resolve(repo.Entries, c.Reference)
		if err != nil {
			return err
		}
	} else {
		integratedBranches := switchIntegratedBranches(rt.ctx, rt.gitRunner, repo)
		option, err := rt.picker(rt.ctx, pickerOptions(repo.Entries, integratedBranches), rt.stdin, rt.stderr)
		if err != nil {
			return err
		}
		selected = worktree.Entry{Path: option.Path}
	}

	_, _ = fmt.Fprintln(rt.stdout, selected.Path)
	return nil
}

func (c *NewCmd) Run(rt *runtime) error {
	creator := worktree.Creator{Runner: rt.gitRunner}
	_, err := creator.Create(rt.ctx, worktree.NewOptions{
		Cwd:     rt.cwd,
		Branch:  c.Branch,
		Base:    c.Base,
		Stdout:  rt.stdout,
		Stderr:  rt.stderr,
		Environ: rt.environ,
	})
	return err
}

func (c *RebaseCmd) Run(rt *runtime) error {
	service := rebase.New(rt.gitRunner, rt.stdout, rt.stderr)
	return service.Run(rt.ctx, rebase.Options{Cwd: rt.cwd, Base: c.Base})
}

func (c *RemoveCmd) Run(rt *runtime) error {
	service := remove.New(rt.gitRunner, rt.stderr)
	result, err := service.Run(rt.ctx, remove.Options{Cwd: rt.cwd, Name: c.Name, Force: c.Force})
	if err != nil {
		return err
	}
	if c.PrintCdTarget && result.CdTarget != "" {
		_, _ = fmt.Fprintln(rt.stdout, result.CdTarget)
	}
	return nil
}

func (c *CopyIgnoredCmd) Run(rt *runtime) error {
	application := app.App{Cwd: rt.cwd, Environ: rt.environ, GitRunner: rt.gitRunner}
	result, err := application.CopyIgnored(rt.ctx, app.CopyIgnoredOptions{
		From:   c.From,
		To:     c.To,
		Force:  c.Force,
		DryRun: c.DryRun,
	})
	if err != nil {
		return err
	}

	if c.DryRun {
		for _, entry := range result.Plan.Entries {
			_, _ = fmt.Fprintf(rt.stdout, "%s %s\n", entry.Action, entry.RelPath)
		}
		if len(result.Plan.Entries) == 0 {
			writeNoop(rt.stderr, result.Plan.NoopReason)
			return nil
		}
		copyCount, skipCount := countPlanActions(result.Plan.Entries)
		_, _ = fmt.Fprintf(rt.stderr, "dry-run: %d planned, copied %d, skipped %d\n", len(result.Plan.Entries), copyCount, skipCount)
		return nil
	}

	if len(result.Plan.Entries) == 0 {
		writeNoop(rt.stderr, result.Plan.NoopReason)
		return nil
	}
	_, _ = fmt.Fprintf(rt.stderr, "copied %d, skipped %d\n", result.Result.Copied, result.Result.Skipped)
	return nil
}

func (rt *runtime) loadRepository() (worktree.Repository, error) {
	return worktree.LoadRepository(rt.ctx, rt.gitRunner, rt.cwd)
}

func currentEntry(entries []worktree.Entry) (worktree.Entry, bool) {
	for _, entry := range entries {
		if entry.IsCurrent {
			return entry, true
		}
	}
	return worktree.Entry{}, false
}

func entryState(entry worktree.Entry) string {
	switch {
	case entry.Branch != "":
		return entry.Branch
	case entry.IsDetached:
		if len(entry.Head) >= 12 {
			return "detached " + entry.Head[:12]
		}
		return "detached"
	case entry.IsBare:
		return "bare"
	default:
		return "unknown"
	}
}

func pickerOptions(entries []worktree.Entry, integratedBranches map[string]struct{}) []tui.PickerOption {
	options := make([]tui.PickerOption, 0, len(entries))
	for _, entry := range entries {
		options = append(options, tui.PickerOption{
			Label:      entry.DisplayName,
			Branch:     entry.Branch,
			Path:       entry.Path,
			Integrated: entryCanBeMuted(entry) && branchIsIntegrated(entry.Branch, integratedBranches),
		})
	}
	return options
}

func switchIntegratedBranches(ctx context.Context, runner git.Runner, repo worktree.Repository) map[string]struct{} {
	defaultBranch, ok := switchDefaultBranch(ctx, runner, repo)
	if !ok {
		return nil
	}

	result, err := runner.Run(ctx, repo.Primary.Path, "for-each-ref", "--format=%(refname:short)", "--merged="+defaultBranch, "refs/heads")
	if err != nil || result.ExitCode != 0 {
		return nil
	}

	integratedBranches := make(map[string]struct{})
	for _, line := range strings.Split(result.Stdout, "\n") {
		branch := strings.TrimSpace(line)
		if branch == "" || branch == repo.Primary.Branch || branchMatchesDefault(branch, defaultBranch) {
			continue
		}
		integratedBranches[branch] = struct{}{}
	}
	return integratedBranches
}

func switchDefaultBranch(ctx context.Context, runner git.Runner, repo worktree.Repository) (string, bool) {
	result, err := runner.Run(ctx, repo.Primary.Path, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", false
	}
	if result.ExitCode == 0 {
		branch := strings.TrimSpace(result.Stdout)
		return branch, branch != ""
	}

	if repo.Primary.Branch == "main" || repo.Primary.Branch == "master" {
		return repo.Primary.Branch, true
	}

	hasMain := switchLocalBranchExists(ctx, runner, repo, "main")
	hasMaster := switchLocalBranchExists(ctx, runner, repo, "master")
	switch {
	case hasMain && !hasMaster:
		return "main", true
	case hasMaster && !hasMain:
		return "master", true
	default:
		return "", false
	}
}

func switchLocalBranchExists(ctx context.Context, runner git.Runner, repo worktree.Repository, branch string) bool {
	result, err := runner.Run(ctx, repo.Primary.Path, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil && result.ExitCode == 0
}

func entryCanBeMuted(entry worktree.Entry) bool {
	return entry.Branch != "" && !entry.IsPrimary && !entry.IsDetached
}

func branchIsIntegrated(branch string, integratedBranches map[string]struct{}) bool {
	_, ok := integratedBranches[branch]
	return ok
}

func branchMatchesDefault(branch string, defaultBranch string) bool {
	return branch == defaultBranch || branch == strings.TrimPrefix(defaultBranch, "origin/")
}

func countPlanActions(entries []copyignored.Entry) (int, int) {
	copyCount := 0
	skipCount := 0
	for _, entry := range entries {
		switch entry.Action {
		case copyignored.ActionCopy:
			copyCount++
		case copyignored.ActionSkipExisting:
			skipCount++
		}
	}
	return copyCount, skipCount
}

func writeNoop(stderr io.Writer, reason string) {
	if reason == "" {
		reason = "no ignored allowlisted files to copy"
	}
	_, _ = fmt.Fprintln(stderr, reason)
}

func writeDiagnostic(stderr io.Writer, err error) {
	var ambiguous resolve.AmbiguousError
	if errors.As(err, &ambiguous) {
		_, _ = fmt.Fprintf(stderr, "ambiguous worktree %q; candidates: %s\n", ambiguous.Query, strings.Join(ambiguous.Candidates, ", "))
		return
	}

	var missing resolve.MissingError
	if errors.As(err, &missing) {
		if len(missing.Candidates) > 0 {
			_, _ = fmt.Fprintf(stderr, "worktree %q not found; candidates: %s\n", missing.Query, strings.Join(missing.Candidates, ", "))
			return
		}
		_, _ = fmt.Fprintf(stderr, "worktree %q not found\n", missing.Query)
		return
	}

	_, _ = fmt.Fprintln(stderr, err)
}
