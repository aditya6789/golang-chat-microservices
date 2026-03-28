# Contributing

Thanks for your interest in improving this project.

## How to contribute

1. **Open an issue** first for larger changes (architecture, new services) so we can align on direction.
2. **Fork** the repository and create a **feature branch** from `main`.
3. Keep commits **focused** and messages **clear** (conventional style welcome, e.g. `feat:`, `fix:`, `docs:`).
4. Run **`go test ./...`** before opening a PR.
5. If you change behaviour or APIs, update **`README.md`** (and `docs/swagger.yaml` when relevant).

## Pull requests

- Describe **what** changed and **why** (short paragraph is enough).
- Link related **issues** when applicable.
- Avoid drive-by large refactors unless discussed in an issue.

## Code style

- Match existing **Go** layout: `cmd/`, `internal/`, small focused packages.
- Prefer **minimal diffs**; don’t reformat unrelated files.

Questions? Open a discussion or issue on GitHub.
