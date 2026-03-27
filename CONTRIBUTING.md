# Contributing

Thanks for your interest in contributing to git-better-branch!

## Getting started

1. Fork and clone the repository
2. Make sure you have Go 1.26+ installed
3. Run `make build` to verify everything compiles

## Making changes

1. Create a branch for your change
2. Make your changes
3. Run `make build` to verify it compiles
4. Test manually against a repository with a variety of branches, remotes, and worktrees
5. Submit a pull request

## Guidelines

- Keep it small and fast. This is a utility people run many times a day.
- Minimize dependencies. Prefer the Go standard library where practical.
- Follow existing code style and conventions.
- Update the CHANGELOG.md with your changes under an `[Unreleased]` section.

## Reporting bugs

Open an issue with:

- What you expected to happen
- What actually happened
- Your `git --version` and `git better-branch --version` output
- Your OS and terminal emulator

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
