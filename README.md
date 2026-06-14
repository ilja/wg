# wg

`wg` is a small Git worktree manager that keeps native Git as the source of truth. Its first version is intentionally focused on the commands needed to create, inspect, enter, update, copy local ignored files between, and safely remove worktrees.

## Commands

- `wg list` — list known worktrees with the current worktree marked.
- `wg switch [name]` — select a worktree; with zsh integration this changes the parent shell directory.
- `wg path <name>` — print exactly the resolved worktree path.
- `wg new <branch> [base]` — create a sibling worktree and branch from an explicit or resolved default base.
- `wg rebase [base]` — fetch the base when available and run native `git rebase` in the current worktree.
- `wg copy-ignored --from <name> --to <name>` — copy allowlisted ignored local files.
- `wg env [name]` — print deterministic `WG_*` setup context values.
- `wg remove [-D] [name]` — remove one non-primary worktree when it is proven integrated, or force-remove one named target with `-D`.

## Shell setup

For zsh parent-shell directory changes, initialize the function in your shell startup:

```zsh
eval "$(wg config shell init zsh)"
```

The zsh wrapper intercepts `wg switch` and `wg remove`. Successful `wg switch` changes the caller directory to the selected worktree. Successful removal of the current worktree changes the caller directory back to the primary worktree.

## Project setup hook

`wg new` looks for `.config/setup.sh` in the primary worktree. When present, it runs that script with the new worktree as the current directory. This keeps project-specific setup in the repository instead of baking it into `wg`.

The setup script receives these values:

- `WG_BRANCH`
- `WG_WORKTREE_PATH`
- `WG_WORKTREE_NAME`
- `WG_REPO`
- `WG_REPO_PATH`
- `WG_PRIMARY_WORKTREE_PATH`
- `WG_DEFAULT_BRANCH`
- `WG_BASE`
- `WG_PORT`

Example:

```sh
#!/bin/sh
set -eu

wg copy-ignored --from main --to "$WG_WORKTREE_NAME"
direnv allow || true
export PORT="$WG_PORT"
export PUMA_METRICS_PORT="$((WG_PORT + 1))"
ln -sfn /path/to/shared/plans "$WG_WORKTREE_PATH/.plans"
ln -sfn /path/to/shared/wiki "$WG_WORKTREE_PATH/.wiki"
```

## Out of scope for v1

`wg` does not implement bulk prune, merge workflows, LLM commit/squash/push flows, generic hook DSLs, dev-server management, URL/status columns, automatic migration from other tools, or unrelated Git workflow replacement. Use native Git and project-local scripts for those workflows.
