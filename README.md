# git-better-branch

[![CI](https://github.com/redodson01/git-better-branch/actions/workflows/ci.yml/badge.svg)](https://github.com/redodson01/git-better-branch/actions/workflows/ci.yml)

A better `git branch` viewer for repositories with long branch names, worktrees, and many remotes.

## The problem

`git branch -avv` becomes unreadable when working with long branch names (e.g., ticket IDs), worktree paths, and hundreds of remote branches:

```
+ jdoe/APP-2847-add-multi-factor-authentication-to-user-login-flow        a1b2c3d4 (/Users/jdoe/code/webapp/.worktrees/APP-2847) [origin/jdoe/APP-2847-add-multi-factor-authentication-to-user-login-flow] Add TOTP and WebAuthn support to login controller
+ msmith/APP-3021-refactor-notification-preferences-api-with-bulk-update  f5e6d7c8 (/Users/jdoe/code/webapp/.worktrees/APP-3021) [origin/msmith/APP-3021-refactor-notification-preferences-api-with-bulk-update] Replace N+1 queries with batch upsert for notification preferences
* main                                                                    90ab12cd [origin/main] Merge pull request #847 from webapp/fix-session-timeout
```

## The solution

`git better-branch` gives you the same information in a compact, readable format:

```
* main                              origin  90ab12c Merge pull request #847 from webapp/fix-session-tim…
+ jdoe/APP-2847-add-multi-facto…    origin  a1b2c3d Add TOTP and WebAuthn support to login controller [APP-2847]
+ msmith/APP-3021-refactor-noti…    origin  f5e6d7c Replace N+1 queries with batch upsert for notifica… [APP-3021]
```

Features:

- **Smart truncation** of branch names and commit messages to fit your terminal
- **Compact tracking status**: `↑3` (ahead), `↓2` (behind), `↑3↓2` (diverged), `gone`, `local`
- **Tracking remote** shown in its own column; full upstream ref shown only when it differs from the local branch name
- **Worktree indicator**: `+` prefix with `[worktree-name]` tag instead of full absolute paths
- **Remote branches** listed with `-a`, colored red to distinguish from local branches
- **Color-coded** output, automatically disabled when piped
- **Terminal-width aware** column layout
- **Auto-pager** when output exceeds terminal height
- **Interactive mode** (`-i`): TUI branch picker with fuzzy search and checkout

## Installation

Requires Go 1.26+.

```bash
git clone https://github.com/redodson01/git-better-branch.git
cd git-better-branch
make install  # installs to ~/.local/bin by default
```

Or specify a different prefix:

```bash
make install PREFIX=/usr/local
```

Ensure the install directory is in your `PATH`.

## Usage

```bash
git better-branch            # local branches
git better-branch -a         # include remote branches
git better-branch -i         # interactive branch picker
git better-branch -i -a      # interactive with remote branches
git better-branch --no-color
git better-branch --version
```

### Passthrough

Flags and arguments not recognized by `git better-branch` are passed through to `git branch`:

```bash
git better-branch -m old-name new-name   # rename a branch
git better-branch -d feature             # delete a branch
git better-branch -v                     # verbose git branch output
git better-branch --sort=-committerdate  # sort by recent commit
```

### Interactive mode

Use the `-i` flag to launch a TUI branch picker:

- **`/`** to fuzzy search (matches branch names and remote names)
- **`j`/`k`** or **`↑`/`↓`** to navigate
- **`Enter`** to checkout the selected branch
- **`d`** to delete a branch (`git branch -d`); **`D`** to force-delete (`git branch -D`) or delete a remote branch
- **`Esc`/`q`** to quit

### Aliases

```bash
# git bb
git config --global alias.bb better-branch

# gbb (shell alias — add to your .bashrc/.zshrc)
alias gbb='git better-branch'
```

## License

[MIT](LICENSE)
