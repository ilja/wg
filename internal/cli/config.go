package cli

import (
	"fmt"

	"wg/internal/shell"
)

type ConfigCmd struct {
	Shell ConfigShellCmd `cmd:"" help:"Print shell integration helpers."`
}

type ConfigShellCmd struct {
	Init ConfigShellInitCmd `cmd:"" help:"Print shell initialization code."`
}

type ConfigShellInitCmd struct {
	Shell string `arg:"" enum:"zsh" help:"Shell to initialize."`
}

func (c *ConfigShellInitCmd) Run(rt *runtime) error {
	if c.Shell != "zsh" {
		return fmt.Errorf("unsupported shell %q", c.Shell)
	}
	_, _ = fmt.Fprint(rt.stdout, shell.ZshInit("wg"))
	return nil
}
