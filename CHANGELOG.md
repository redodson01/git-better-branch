# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Passthrough of unrecognized flags and arguments to `git branch`

### Changed

- `-v` is no longer an alias for `--version`; it now passes through to `git branch -v`

## [0.1.0] - 2026-03-27

### Added

- Local branch listing with color-coded indicators (`*` for HEAD, `+` for worktree)
- Smart truncation of branch names and commit messages based on terminal width
- Separate columns for commit deviation (`↑N`, `↓N`, `↑N↓N`, `gone`) and remote tracking ref
- Tracking ref shown only when it differs from the local branch name
- Worktree name shown as `[name]` tag instead of full absolute path
- Remote branch listing with `-a` flag, displayed inline with red coloring
- Auto-pager via `less -RFX` when output exceeds terminal height
- Interactive branch picker (`-i`) with bubbletea TUI
- Fuzzy search in interactive mode (`/` to search branch and remote names)
- Checkout on `Enter` in interactive mode (creates local tracking branch for remotes)
- Branch deletion in interactive mode (`d` for safe delete, `D` for force delete) with merge pre-check and confirmation prompt
- Reverse-video selection with colored column gaps for clean highlighting
- `--no-color` flag and `NO_COLOR` environment variable support
- Makefile with `build`, `install`, `uninstall`, `clean`, `test`, and `lint` targets

[Unreleased]: https://github.com/redodson01/git-better-branch/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/redodson01/git-better-branch/releases/tag/v0.1.0
