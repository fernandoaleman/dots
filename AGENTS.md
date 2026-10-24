# AGENTS.md — dots

Instructions for Codex and other coding agents contributing to **dots**, a lightweight Go dotfiles manager.

## Read first

1. Read [`docs/HANDOFF.md`](docs/HANDOFF.md) before planning or editing code. It is the source of truth for requirements and milestone scope.
2. Read `README.md`, `go.mod`, and relevant package tests before modifying behavior.
3. If instructions conflict, follow the user's latest explicit request, then this file, then the handoff. Call out any consequential conflict rather than guessing.

## Product principles

- One Git repository, one Go module, **one `dots` executable**.
- Prefer small, idiomatic Go code and the standard library. Add dependencies only with a clear rationale.
- Keep responsibilities separated into three logical packages under `internal/`:
  - `cli`: commands, flags, user-facing output.
  - `resolver`: configuration discovery, path mapping, priority, and desired-link planning.
  - `filesystem`: filesystem inspection and (in later milestones) safe symlink operations.
- Do not introduce microservices, extra binaries, templates, or speculative abstractions.
- Use `gofmt` and idiomatic Go naming: `PascalCase` for exported identifiers and `camelCase` for unexported identifiers.

## Configuration and precedence contract

- **TOML only**. Default configuration is `$XDG_CONFIG_HOME/dots/config.toml`, falling back to `$HOME/.config/dots/config.toml` when `XDG_CONFIG_HOME` is unset, empty, or not absolute.
- Support explicit `--config` to select a configuration file; do **not** automatically fall back to `~/.rcrc` or `~/.dots.toml`.
- The ordered `dotfiles_dirs` array determines precedence: **the first directory wins**. `~/dotfiles-local` can override `~/dotfiles`.
- Both dotted and undotted source names are supported: `gitconfig` and `.gitconfig` map to `~/.gitconfig`; `config/nvim/init.lua` and `.config/nvim/init.lua` map to `~/.config/nvim/init.lua`.
- **Directory precedence wins before filename preference.** Within the *same* source directory, the dotted variant wins; warn if both map to the same destination. Normal overrides between directories are intentional and do not require warnings.
- Respect the target directory and exclusions defined in the handoff. Do not force end users to relocate their own app configurations to XDG paths.
- Never execute shell statements from configuration files. `docs/MIGRATING_FROM_RCM.md` describes manual migration from rcm.

## Current milestone: M0 only

Implement **`dots up --dry-run`** as the first vertical slice. It should load and validate TOML, resolve sources to intended destinations, report conflicts and duplicate-name warnings, and print a deterministic proposed action plan.

**Dry-run is strictly read-only**: no symlink creation, directory creation, backup, deletion, configuration generation, or changes to real user dotfiles. Use temporary fixture directories in tests.

Do not implement mutating `dots up`, `dots down`, `dots add`, or other future features until explicitly asked. CLI names and future flags may be documented without implementing them.

## Development and checks

- Write or update tests **with every behavior change**, including positive cases, conflicts, malformed TOML, directory ordering, same-directory dotted-name preference, nested `.config` mapping, and read-only dry-run behavior.
- Prefer table-driven tests and `t.TempDir()`; do not depend on real `$HOME` contents.
- Before reporting completion, run where the toolchain is available:

```bash
gofmt -w .
go test ./...
go vet ./...
go build ./...
prek run --all-files
```

- If the project Makefile provides equivalent targets, prefer `make check` for the full local suite. Do not state that checks passed unless they actually ran; report any unavailable tool or failed check explicitly.
- `prek.toml` configures local hooks; `committed.toml` enforces Conventional Commits; `.typos.toml` configures spelling checks. Respect them when present.
- CI must independently validate Go formatting, tests, vet, and build; local hooks alone are not sufficient.

## Git, documentation, and releases

- Use Conventional Commits, e.g., `feat(resolver): apply local precedence`, `test(cli): cover dry-run conflicts`, or `docs: clarify config location`.
- Keep `README.md` accurate, update `CHANGELOG.md` for user-visible changes, and document rcm migration separately.
- Use Semantic Versioning (`v0.x.y` during early development). Do not create tags, publish releases, or push changes without explicit approval.
- Future release targets include macOS `arm64`/`amd64` and Linux `arm64`/`amd64`; do not add complex release tooling for M0.

## Working with Herdr agents

- **Lead/orchestrator:** owns project changes, task breakdown, integration, and final reporting.
- **Testing agent:** read-only test-plan and coverage review in M0.
- **Architecture agent:** read-only package/API review in M0.
- **Review agent:** read-only correctness, safety, and regression review in M0.
- Delegation must be explicit. Specialists must not edit files, create commits, or change branches unless the user authorizes a different collaboration model.
- Use Herdr's supported agent commands only after confirming the installed CLI's syntax. Treat specialist findings as recommendations to verify, not automatic instructions to apply.
- Avoid simultaneous edits to the same checkout. Consider isolated Git worktrees only in a later milestone.

## Safety and reporting

- Never overwrite, unlink, or delete unmanaged files. Do not run destructive cleanup commands against the user's home directory.
- Ask for explicit permission before modifying real dotfiles, changing Git remotes, pushing commits, or publishing releases.
- Summarize changed files, key decisions, checks run and results, unresolved issues, and next recommended milestone at the end of each task.
