# AGENTS.md — dots

Instructions for Codex and other coding agents contributing to **dots**, a lightweight Go dotfiles manager.

`CLAUDE.md` imports this file. Keep shared standards here so they apply to
Codex and Claude Code equally.

## Read first

1. Read [`docs/HANDOFF.md`](docs/HANDOFF.md) before planning or editing code. It is the source of truth for requirements and milestone scope.
2. Read `README.md`, `go.mod`, and relevant package tests before modifying behavior.
3. If instructions conflict, follow the user's latest explicit request, then this file, then the handoff. Call out any consequential conflict rather than guessing.

## Product principles

- One Git repository, one Go module, **one `dots` executable**.
- Prefer small, idiomatic Go code and the standard library. Add dependencies only with a clear rationale.
- Keep responsibilities separated into three logical packages under `internal/`:
  - `cli`: commands, flags, user-facing output.
  - `resolver`: TOML discovery/loading, source traversal, exclusions, destination mapping, precedence, and structural collision detection.
  - `filesystem`: destination inspection, prepared symlink installation, and safe parent creation.
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

## Current milestone: M1 only

Milestone 0 is complete. Implement **`dots up`** as create-only symlink
installation, preserving the existing resolution and dry-run behavior.
Follow the approved Milestone 1 contract in `docs/HANDOFF.md`.

**Dry-run is strictly read-only**: no symlink creation, directory creation, backup, deletion, configuration generation, or changes to real user dotfiles. Use temporary fixture directories in tests.

Do not implement `dots down`, `dots add`, replacement, ownership-based
relinking, backups, or other future features. Work on the M1 feature branch;
only the lead edits files. Tests must isolate HOME, XDG, sources, and targets.

## Development and checks

- Follow idiomatic Go naming and preserve conventional initialisms.
- Every nontrivial package must have a meaningful documentation comment
  beginning with `// Package <name>`.
- Error strings should normally start with lowercase letters and have no
  trailing punctuation, except for proper nouns and acronyms.
- Keep implementation changes focused on the current milestone.
- Do not bypass prek hooks or suppress Staticcheck warnings without
  documented justification.

- Write or update tests **with every behavior change**, including positive cases, conflicts, malformed TOML, directory ordering, same-directory dotted-name preference, nested `.config` mapping, and read-only dry-run behavior.
- Prefer table-driven tests and `t.TempDir()`; do not depend on real `$HOME` contents.
- Before reporting completion, run where the toolchain is available:

```bash
make fmt
make vet
make test
make lint
make check
prek run --all-files
```

- Run the individual formatting, vet, test, and lint targets before reporting completion, and `make check` for final validation. It also builds the executable and runs all configured hooks. Report unavailable tools and failed checks explicitly.
- `staticcheck.conf` inherits recommended defaults and adds selected documentation, naming, and error-formatting checks. Keep the Staticcheck version pinned in the Makefile and use the same installer in CI.
- `prek.toml` configures local hooks; `committed.toml` enforces Conventional Commits; `.typos.toml` configures spelling checks. Respect them when present.
- CI must independently validate Go formatting, vet, tests, Staticcheck, build, and applicable prek checks; local hooks alone are not sufficient.

## Git, documentation, and releases

- Use Conventional Commits, e.g., `feat(resolver): apply local precedence`, `test(cli): cover dry-run conflicts`, or `docs: clarify config location`.
- Keep `README.md` accurate, update `CHANGELOG.md` for user-visible changes, and document rcm migration separately.
- Use Semantic Versioning (`v0.x.y` during early development). Do not create tags, publish releases, or push changes without explicit approval.
- Future release targets include macOS `arm64`/`amd64` and Linux `arm64`/`amd64`; do not add complex release tooling for M0.

## Working with Herdr agents

- **Lead/orchestrator:** owns project changes, task breakdown, integration, and final reporting.
- **Testing agent:** read-only test-plan and coverage review.
- **Architecture agent:** read-only package/API review.
- **Review agent:** read-only correctness, safety, and regression review.
- Delegation must be explicit. Specialists must not edit files, create commits, or change branches unless the user authorizes a different collaboration model.
- Use Herdr's supported agent commands only after confirming the installed CLI's syntax. Treat specialist findings as recommendations to verify, not automatic instructions to apply.
- Avoid simultaneous edits to the same checkout. Consider isolated Git worktrees only in a later milestone.

## Safety and reporting

- Never overwrite, unlink, or delete unmanaged files. Do not run destructive cleanup commands against the user's home directory.
- Ask for explicit permission before modifying real dotfiles, changing Git remotes, pushing commits, or publishing releases.
- Summarize changed files, key decisions, checks run and results, unresolved issues, and next recommended milestone at the end of each task.

### Approved Milestone 0 contract

- `cli` owns commands, flags, orchestration, and output.
- `resolver` owns TOML discovery/loading, source traversal, exclusions,
  destination mapping, precedence, and structural collision detection.
- `filesystem` performs read-only destination and ancestor inspection.
- Dependencies are `main → cli → {resolver, filesystem}`; neither lower
  package depends on the other. Resolve desired links before inspecting
  destinations; occupancy never changes the selected source.
- Missing source roots warn when another usable root exists; otherwise
  error. Empty roots are usable. Unreadable and non-directory roots error.
- Exclusions match literal original source-root-relative paths before
  normalization and precedence. Excluded directories prune subtrees.
  Glob matching is deferred.
- Nested source symlinks are opaque entries and are never traversed.
  Dangling source links error. Configured roots may be symlinks to
  accessible directories.
- Detect file/directory structural collisions within and across roots.
  Leaf precedence cannot hide ancestor/descendant conflicts. Report all
  participants, block affected entries, and display unrelated entries.
- Broken destination symlinks are occupied conflicts with a distinct
  reason. Do not relink or remove anything.
- Plans and diagnostics are deterministic regardless of enumeration order.
- Preserve symlink-plus-`..` traversal in configured paths; do not collapse
  it lexically. Report unresolved traversal beneath missing ancestors.
- Inspect target-directory case sensitivity and Unicode normalization
  without writing probe files. Preserve distinctions supported by directory
  capabilities; block equivalent destinations, including conflicts with
  empty source directories. If capability queries fail, block potentially
  ambiguous entries while retaining unrelated entries.
- Warning-only plans succeed; errors and conflicts return nonzero.
- Dry-run performs no filesystem modifications.
