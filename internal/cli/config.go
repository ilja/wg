package cli

import (
	"fmt"
	"strings"

	"wg/internal/shell"
	"wg/internal/worktree"
)

type ConfigCmd struct {
	Shell ConfigShellCmd `cmd:"" help:"Print shell integration helpers."`
}

type ConfigShellCmd struct {
	Init     ConfigShellInitCmd     `cmd:"" help:"Print shell initialization code."`
	Complete ConfigShellCompleteCmd `cmd:"" hidden:"" help:"Print shell completion candidates."`
}

type ConfigShellInitCmd struct {
	Shell string `arg:"" enum:"zsh" help:"Shell to initialize."`
}

type ConfigShellCompleteCmd struct {
	Remove ConfigShellCompleteRemoveCmd `cmd:"" hidden:"" help:"Print remove completion candidates."`
}

type ConfigShellCompleteRemoveCmd struct {
	Prefix string `arg:"" optional:"" name:"prefix" help:"Branch prefix to complete."`
}

func (c *ConfigShellInitCmd) Run(rt *runtime) error {
	if c.Shell != "zsh" {
		return fmt.Errorf("unsupported shell %q", c.Shell)
	}
	_, _ = fmt.Fprint(rt.stdout, shell.ZshInit("wg"))
	return nil
}

func (c *ConfigShellCompleteRemoveCmd) Run(rt *runtime) error {
	repo, err := rt.loadRepository()
	if err != nil {
		return err
	}
	for _, branch := range removeCompletions(repo.Entries, c.Prefix) {
		_, _ = fmt.Fprintln(rt.stdout, branch)
	}
	return nil
}

func removeCompletions(entries []worktree.Entry, prefix string) []string {
	matches := make([]string, 0)
	for _, entry := range entries {
		if entry.IsPrimary || entry.Branch == "" || entry.IsDetached {
			continue
		}
		if strings.HasPrefix(entry.Branch, prefix) {
			matches = append(matches, entry.Branch)
		}
	}
	return matches
}
