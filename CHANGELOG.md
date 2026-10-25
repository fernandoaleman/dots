# Changelog

## Unreleased

### Added

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
