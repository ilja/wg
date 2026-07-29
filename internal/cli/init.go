package cli

import (
	"fmt"
	"io"

	"wg/internal/app"
)

type InitCmd struct {
	Force bool `help:"Replace an existing regular setup hook."`
}

func (c *InitCmd) Run(rt *runtime) error {
	application := app.App{Cwd: rt.cwd, Environ: rt.environ, GitRunner: rt.gitRunner}
	result, err := application.Init(rt.ctx, app.InitOptions{Force: c.Force})
	if err != nil {
		return err
	}
	return renderInitResult(rt.stdout, result)
}

func renderInitResult(w io.Writer, result app.InitResult) error {
	_, err := fmt.Fprintf(w, `Initialized setup hook: %s

Tailor it before running wg new:
- Configure .worktreeinclude and use wg copy-ignored
- Create or update local environment files
- Install project dependencies
- Symlink resources shared with the primary worktree
- Enable environment tooling such as direnv
`, result.HookPath)
	return err
}
