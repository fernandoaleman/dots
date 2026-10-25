# Changelog

## Unreleased

### Added

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

- Detect Unicode normalization and full case-folding destination collisions.
- Retain empty source directories through filesystem-aware structural checks.
