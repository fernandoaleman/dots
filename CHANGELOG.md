# Changelog

## Unreleased

### Added

- Strict ownership-registry v2 reading and pending-import mutation gates,
  preserving v1 behavior without producing import intents or exposing recovery.

- Isolated restrictive-umask bootstrap tests and documented permission recovery
  for newly created ownership directories and stable locks.
- Registry-backed `dots down` and strictly read-only `dots down --dry-run`,
  with current-target selection, recorded identity checks, and partial-failure
  reporting. Newly created links gain ownership; existing links are never adopted.
- Private XDG ownership state, stable process locking, durable transaction
  snapshots, conservative interrupted-operation recovery, and temporary fixtures.
- Documented cooperative concurrency requirements and legacy-link limitations.

- Read-only `dots ls` for resolver-selected mappings and `dots status` for
  deterministic destination classifications, summaries, and documented exits.
- Structured inspection diagnostics separating wrong/broken links, occupied
  paths, blocked ancestors, and metadata or name-capability failures.
- Temporary inspection fixtures covering no-write execution and shared source
  selection across listing, status, and existing dry-run behavior.

- Create-only `dots up` with prepared directory handles, exclusive symlink
  creation, safe parent handling, idempotency, and partial-failure reports.
- Temporary installation fixtures covering conflict preservation, controlled
  filesystem races, descriptor cleanup, permissions, and retry behavior.
- Documented Milestone 1 safety guarantees and macOS/Linux race limitations.

- Shared Staticcheck configuration with recommended defaults and selected
  Go documentation, naming, and error-formatting checks.
- Pinned Staticcheck installation, module-wide linting, a staged-Go
  pre-commit gate, and Go-version-aware CI lint and hook checks.
- Shared Go development standards for Codex and Claude Code.

- Read-only `dots up --dry-run`, explicit TOML configuration, and XDG discovery.
- Ordered source precedence, dotted-name preference, and file-level merging.
- Literal exclusions, missing-root warnings, and source-symlink validation.
- Destination and ancestor inspection, structural and alias collision reports,
  deterministic output, and nonzero exit statuses for errors and conflicts.
- Temporary-directory tests, read-only fixture snapshots, macOS/Linux CI,
  development checks, and manual rcm migration instructions.

### Fixed

- Block removal when another recorded profile or target claims an equivalent
  physical destination, including failed name-capability checks.
- Report oversized ownership registries explicitly rather than as nil read errors.
- Suppress the empty-ownership message when removal inspection fails, preserving
  the distinction between unknown ownership and successfully inspected empty state.
- Reject ownership snapshots with umask-restricted permissions before publication,
  preserving the previous usable registry and managed links on failure.
- Preserve non-directory source error classification so recorded broken links
  remain removable when configured source roots or their referents become files.
- Classify symlinks with non-directory target components as broken links,
  while retaining inspection errors for permission and capability failures.
- Detect Unicode normalization and full case-folding destination collisions.
- Retain empty source directories through filesystem-aware structural checks.
