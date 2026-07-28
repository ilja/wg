# wg

`wg` is a small Git worktree manager that keeps native Git as the source of truth. Its first version is intentionally focused on the commands needed to create, inspect, enter, update, copy local ignored files between, and safely remove worktrees.

## Installation

Build `wg` from the repository root:

```sh
go build -o wg ./cmd/wg
```

Install it into a directory on your `PATH`, for example:

```sh
mkdir -p ~/bin
mv wg ~/bin/wg
```

If `~/bin` is not already on your `PATH`, add this to your shell startup file, such as `~/.zshrc`:

```zsh
export PATH="$HOME/bin:$PATH"
```

Reload the shell and verify the command is available:

```sh
source ~/.zshrc
wg --help
```

Alternatively, install through Go:

```sh
go install ./cmd/wg
```

When using `go install`, make sure `$(go env GOPATH)/bin` or `GOBIN` is on your `PATH`.

## Commands

- `wg list [--json]` — list known worktrees with the current worktree marked.
- `wg switch [name]` — select a worktree; with zsh integration this changes the parent shell directory.
- `wg path <name>` — print exactly the resolved worktree path.
- `wg new <branch> [base]` — create a sibling worktree and branch from an explicit or resolved default base.
- `wg rebase [base]` — fetch the base when available and run native `git rebase` in the current worktree.
- `wg copy-ignored --from <name> --to <name>` — copy allowlisted ignored local files.
- `wg env [name]` — print deterministic `WG_*` setup context values.
- `wg remove [-D] [name]` — remove one non-primary worktree when it is proven integrated, or force-remove one named target with `-D`.

## Shell setup

For zsh parent-shell directory changes and `wg remove` branch completion, initialize the function in your shell startup after `compinit`:

```zsh
autoload -Uz compinit
compinit

eval "$(wg config shell init zsh)"
```

The zsh wrapper intercepts `wg switch` and `wg remove`. Successful `wg switch` changes the caller directory to the selected worktree. Successful removal of the current worktree changes the caller directory back to the primary worktree.

The same zsh setup registers completion for the branch argument to `wg remove`:

```zsh
wg remove feat<Tab>
wg remove -D feat<Tab>
```

Completion offers attached, non-primary worktree branch names. If more than one branch starts with the typed prefix, zsh shows the matching choices; cycling through those choices follows your zsh completion settings. To enable menu selection, add this before the `eval` line:

```zsh
zstyle ':completion:*' menu select
```

After changing `~/.zshrc`, reload the shell:

```zsh
source ~/.zshrc
```

## Project setup hook

`wg new` looks for `.config/setup.sh` in the primary worktree. When present, it runs that script with the new worktree as the current directory. This keeps project-specific setup in the repository instead of baking it into `wg`.

The setup script receives these values:

| Variable                   | Example                                      | Meaning                                           |
| -------------------------- | -------------------------------------------- | ------------------------------------------------- |
| `WG_BRANCH`                | `feature/add-search`                         | Branch being created.                             |
| `WG_WORKTREE_PATH`         | `/Users/<user>/work/demo.feature-add-search` | New worktree path.                                |
| `WG_WORKTREE_NAME`         | `feature-add-search`                         | Sanitized worktree name.                          |
| `WG_REPO`                  | `demo`                                       | Repository name.                                  |
| `WG_PRIMARY_WORKTREE_PATH` | `/Users/<user>/work/demo`                    | Primary worktree path, useful as the setup source. |
| `WG_DEFAULT_BRANCH`        | `main`                                       | Resolved default branch.                          |
| `WG_BASE`                  | `main`                                       | Base passed or resolved for `wg new`.             |
| `WG_PORT`                  | `14832`                                      | Stable port derived from the worktree path.       |

Example setup script:

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
