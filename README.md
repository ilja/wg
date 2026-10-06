# wg

`wg` manages Git worktrees with project setup hooks, shell switching, and safe cleanup. Native Git remains the source of truth.

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

## Quick start

After installing `wg`, enable zsh integration in `~/.zshrc` after `compinit`:

```zsh
autoload -Uz compinit
compinit

eval "$(wg config shell init zsh)"
```

Reload your shell, then create a worktree and enter it:

```sh
source ~/.zshrc
wg new feature/add-search main
wg switch feature/add-search
```

Replace `main` with your repository's base branch, or omit it to use the resolved default. Once your branch is integrated, remove the worktree:

```sh
wg remove feature/add-search
```

With zsh integration, removing the current worktree returns your shell to the primary worktree. See [Project setup](#project-setup) to automate dependency installation and local environment setup.

### Integrate `wg` with a coding agent

Give your coding agent this prompt to add native worktree support using `wg`:

```text
Set up Git worktree support for my coding agent using `wg`.

First inspect the agent’s extension or plugin system and current project instructions. Find how it can create a worktree and switch its active session into that directory.

Build the smallest integration that:
- Creates worktrees with `wg new <branch> [base]`.
- Runs `.config/setup.sh` through `wg new`, if present.
- Switches the agent session into the new worktree.
- Provides a way to return to the original worktree and remove the created worktree with `wg remove -D`.
- Handles errors without losing the original session directory.

Add concise instructions for using the integration. Include the `wg` command reference and note that `wg copy-ignored` is needed to copy ignored files; `wg new` does not copy them automatically.

Follow the agent’s native extension conventions. Do not change global configuration or install dependencies without asking. Report the files changed and how to enable the integration.
```

## Commands

| Command | Purpose |
| --- | --- |
| `wg list [--json]` | List worktrees, marking the current one. |
| `wg switch [name]` | Select a worktree; zsh integration changes your shell directory. |
| `wg path <name>` | Print exactly the resolved worktree path. |
| `wg init [--force]` | Install your reusable setup template for this clone. |
| `wg new <branch> [base]` | Fetch `origin` when configured, then create a sibling worktree and branch from the explicit or resolved default base. |
| `wg rebase [base]` | Fetch the base when available and run native `git rebase` in the current worktree. |
| `wg copy-ignored --from <name> --to <name>` | Copy allowlisted ignored local files. |
| `wg env [name]` | Print deterministic `WG_*` setup context values. |
| `wg remove [-D] [name]` | Remove an integrated non-primary worktree, or force-remove a named target with `-D`. |
| `wg remove --all [--dry-run]` | Remove clean, integrated non-primary worktrees, or preview removals. |

### Removing worktrees

`wg remove` normally requires a non-primary worktree whose branch is proven integrated. Use `-D` only when you intend to force-remove a named target.

`wg remove --all` preserves the primary and current worktrees, along with worktrees that are dirty, locked, detached, bare, or whose branches are not integrated into the default branch. It reports each preserved worktree and its reason. Preview removals with:

```sh
wg remove --all --dry-run
```

The `--all` option cannot be combined with a name or `-D`.

## Shell integration

For zsh parent-shell directory changes and `wg remove` branch completion, initialize the function in your shell startup after `compinit`:

```zsh
autoload -Uz compinit
compinit

eval "$(wg config shell init zsh)"
```

The zsh wrapper intercepts `wg switch` and `wg remove`. Successful `wg switch` changes the caller directory to the selected worktree. Successful removal of the current worktree changes the caller directory back to the primary worktree.

### Optional completion settings

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

## Project setup

`wg new` looks for `.config/setup.sh` in the primary worktree. When present, it runs that script with the new worktree as the current directory.

### A simple setup hook

Create `.config/setup.sh` in your primary worktree with the commands your project needs. For example, a Go project could use:

```sh
#!/bin/sh
set -eu

go mod download
```

Make the hook executable:

```sh
chmod u+x .config/setup.sh
```

Then run `wg new` normally. Use the environment variables below to find the primary worktree, copy local files, or configure a worktree-specific port. To keep the hook local to this clone, add `/.config/` to Git's clone-local `info/exclude`, or use `wg init` as described below.

### Initialize a hook from a reusable template

`wg init` installs a user-managed template as the primary worktree's `.config/setup.sh`. It resolves the template from:

1. `$XDG_CONFIG_HOME/wg/setup.sh` when `XDG_CONFIG_HOME` is a non-empty absolute path.
2. `$HOME/.config/wg/setup.sh` when `XDG_CONFIG_HOME` is empty or unset.

Create that reusable template before initializing a repository. The repository's [setup.sh.example](setup.sh.example) is a substantial starting point; adapt its dependency commands, environment files, shared resources, and optional tools for your projects. For example, from this source checkout:

```sh
template_dir="${XDG_CONFIG_HOME:-$HOME/.config}/wg"
mkdir -p "$template_dir"
cp setup.sh.example "$template_dir/setup.sh"
${EDITOR:-vi} "$template_dir/setup.sh"
```

The `wg` executable does not embed or install `setup.sh.example`. Initialization copies only the template at the resolved user configuration path and treats its contents as opaque.

From any directory in a primary or linked worktree, initialize the clone:

```sh
wg init
```

The command always targets the primary worktree, creates its `.config` directory when needed, copies the template bytes, and grants the installed hook owner-execute permission. It also adds the exact `/.config/` pattern to Git's clone-local `info/exclude`, keeping the hook and other local configuration out of `git status` without changing the tracked `.gitignore`.

`wg init` never overwrites an existing `.config/setup.sh` by default. `wg init --force` can replace an existing regular file, but refuses directories, symbolic links, and other non-regular destinations.

Every successful initialization prints the absolute hook path and reminds you to tailor it before running `wg new`:

- Configure `.worktreeinclude` and use `wg copy-ignored` for eligible ignored files.
- Create or update local environment files such as `.env`, `.env.local`, or `.envrc`.
- Install project dependencies, optionally through a tool such as `mise`.
- Symlink resources shared with the primary worktree.
- Enable environment tooling such as `direnv`.

After tailoring the installed hook, create a worktree normally:

```sh
wg new feature/add-search main
```

`wg new` executes the primary worktree's hook inside the new worktree with the setup values below.

### Environment variables

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
