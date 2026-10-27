# Approved import policy

This document records the approved M4 design. It does not indicate that
`dots add` is implemented. [Issue #6 (import policy)](https://github.com/fernandoaleman/dots/issues/6)
and [Issue #7 (import implementation)](https://github.com/fernandoaleman/dots/issues/7)
define milestone scope.

## Current implementation boundary

The resolver provides read-only prospective import planning. It selects a
configured root, maps a literal target-relative name, applies exclusions,
and checks the proposed candidate through the shared resolution logic.
Returned provider evidence includes every existing nonexcluded leaf,
including precedence losers, plus directory evidence in the resolved plan.

A successful resolver plan is not permission to modify the filesystem.
It does not inspect the input or validate native name equivalence,
physical source-slot occupancy, protected paths, historical ownership
claims, permissions, same-filesystem relocation, locking, or recovery.
Excluded entries can still occupy physical source paths. The later
filesystem preflight must check these conditions independently, under the
approved transaction protocol. Public CLI preview and mutation remain
unimplemented until the required safety checks are integrated.

## Selection and mapping

Import one current-user-owned regular file with one hard link and no
privileged permission bits. Reject directory, symlink, and special-file
inputs, including already managed symlinks. Never adopt existing links.

Use the sole configured source root by default. With multiple roots,
require `--root DIR`. The selected root must already exist and be an
unambiguous configured root; do not select another root on failure.

Require the input to belong to the configured target namespace. Strip
one leading top-level dot only when the resulting source path maps back
exactly through the resolver; otherwise require the original spelling to
round-trip. Preserve nested names. Do not change spelling to evade an
exclusion or collision.

Reject existing providers across all roots, including losing variants.
Reuse resolver exclusions, precedence, and structural collision checks.
Protect source roots, configuration, ownership state, and historical
claims using filesystem identity and native name-equivalence checks.

## Relocation and ownership

Support same-filesystem relocation only, using descriptor-relative
exclusive rename. Refuse unsupported semantics and cross-filesystem
operations without a copy or overwriting-rename fallback.

Hold the existing stable ownership lock throughout preflight, durable
import intent, relocation, symlink creation, ownership recording, and
completion. Registry version 2 adds separate import intents while
preserving existing ownership records and phase meanings.

Record both locations, ancestor evidence, original file identity and
metadata, and a versioned content digest before relocation. Synchronize
the file and required directory changes. Create and register the new
symlink through the M3 live-creation protocol.

External writers must not change input contents or metadata, affected
names, source selection, or ancestors throughout import and recovery.
Identity checks reduce observed races; they do not provide protection
against arbitrary concurrent replacement.

## Failure and recovery

Before relocation, failures preserve the original file in place.
After relocation, preserve the file in the source repository, created
entries, and unresolved evidence. The original pathname may be absent.
Stop on failure; do not automatically roll back or remove parents.

Pending imports imply an unfinished transaction and block fresh mutation.
First persist M3's conservative invalidation of prior ownership authority.
Recovery may retire an intent only when verified recorded ancestry and
file identity, metadata, content, and single-link evidence establish:

- The original regular file is intact and the source slot is absent.
- The source contains that same intact file and the original slot is absent.

Synchronize the verified file and both parents before durable retirement.
Revalidate after hashing and immediately before retirement. Preserve the
unfinished barrier until all required recovery transitions are durable.
Then require fresh ordinary preflight before any new operation.

Only inspect import routes for the explicitly selected profile and target.
Report other pending scopes and require explicit selection.

Present symlinks, competing occupants, changed ancestry, or uncertain
evidence keep the gate blocked. Preserve all occupants and report both
locations for independent recovery. Never infer ownership from an intent,
adopt an interrupted link, or use registry editing as ordinary recovery.

## Command contract

Proposed usage: `dots add [--config FILE] [--root DIR] [--dry-run|-n] PATH`.
Require exactly one input. Exit 0 for completion or a valid preview,
1 for conflicts or operational uncertainty, and 2 for invalid usage.

Preview reports the move, link, selected root, and intermediate data
location. It performs no initialization, locking, synchronization for
recovery, state writes, temporary-file creation, or filesystem mutations.

Actual output distinguishes completed relocation, symlink creation, and
ownership publication. Output or persistence failure does not undo writes.
