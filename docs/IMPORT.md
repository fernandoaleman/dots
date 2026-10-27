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
Excluded entries can still occupy physical source paths. Read-only filesystem
path preflight now independently observes filesystem conditions. Existing v1/v2
registry contents and historical claims now constrain admission. Public CLI preview and
mutation remain unimplemented; execution will require fresh preparation under
the approved transaction protocol. Strict registry v2 reading and pending-import gates are implemented; no command
produces v2 import state or performs recovery.

The filesystem API accepts value-only requests and returns observations,
including missing source parents. It retains no handles or mutation authority.
It reuses M3 descriptor walks, native name checks, reserved-path protection,
and reserved source, configuration, and state-path protection. It never
initializes state, acquires a lock, synchronizes, recovers, migrates, creates
directories, or probes by writing files. It reads existing v1/v2 registry snapshots and validates lock metadata,
without acquiring the lock or granting locking authority.

Input paths belong to the configured named target route. As with M1,
observed existing destination-directory symlinks are inspected and pinned,
including aliases outside the physical target root. Configured source-root
aliases are supported; symlinks below a source root remain opaque and cannot
be used as import parents. Raw configured symlink-plus-parent traversal is
preserved. All observed bindings are revalidated before inspection returns.
Existing source parents must match their enumerated spelling; native lookup
aliases are refused so later resolution preserves the proposed mapping.
Write/search access is checked where entries would be added or removed;
a readable root need not be writable when the nested source parent exists.

Preflight rejects known permission, read-only-mount, or device incompatibilities.
Permission and mount checks reopen the selected named route and verify its
identity; inode-equivalent bind mounts can have different read-only flags.
These observations cannot guarantee later rename support, permissions, resource
availability, or durability. Existing regular input files must be user-owned,
single-linked, and free of privileged mode bits. Source occupants are refused
even when excluded from resolver evidence. Configuration and reserved state
paths are protected. Missing configured roots reserve their projected paths;
imports cannot occupy, contain, or fall beneath them. Registry validity and
recorded ownership claims are checked without recovery. Successful observations
never authorize a later write.

Reserved state and source directories are protected by pinned ancestry as well
as projected names. Path-layer state metadata is separate from the versioned registry-claim checks below. Configuration protection compares pinned parent namespaces and regular-file
device/inode identity, including file and directory bind aliases. Named and
referent evidence is revalidated before returning; unavailable evidence blocks
inspection. Portable identity tests run on macOS and Linux. Native Linux bind
fixtures run only in a child private mount namespace when existing capabilities
permit it; they skip explicitly otherwise, without requesting privileges.

All resolver evidence is retained. A blocked destination unrelated to the
requested input may be ignored only after its first obstruction is inspected.
Related slots, input inode aliases, uncertain referents, and broken aliases
that depend on prospective source parents still block import. Observed blockers
are revalidated before returning. Source-resolution diagnostics are unchanged.

## Bounded path and known-object safety

Path preflight protects the active configuration, ownership registry, stable
`ownership.lock`, configured source namespaces, and every regular source
provider included in scanning evidence, including precedence losers. Provider
identity checks are independent of destination reachability. Source symlink
leaves stay opaque; they are never recursively followed. Required unavailable,
changed, or ambiguous evidence refuses the affected import. Path-layer registry and lock
checks are metadata-only. The additional registry-claim layer reads existing
v1/v2 snapshots; neither layer acquires a lock or initializes state.

On macOS, a private `F_GETPATH_NOFIRMLINK` query supplements pinned-directory
inspection with backing-path ancestry. The result must be a bounded, complete,
absolute path whose opened identity matches the original descriptor. Missing
suffixes are projected only below an existing pinned directory. Query failures,
missing returned paths, identity mismatches, and observed late binding changes
refuse inspection. Named paths, resolver mappings, profile identity, and M3
operation paths are preserved; backing paths are comparison evidence only.
Apple documents the query in its [fcntl definitions](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/fcntl.h).

Linux retains descriptor-relative namespace and known-file identity checks,
including file/directory bind aliases where the observed identities expose
those relationships. It does not require procfs or mount privileges for normal
preflight. Tests may use already available mount capabilities in an isolated
child namespace, with explicit skips otherwise. Unrelated mounts and proven
independent blocked destinations do not cause blanket refusal.

Critical-file identities, provider entries, native path observations, and
relevant pinned bindings are rechecked before returning, including subsequent
error exits where evidence has already been collected. Inspection creates no
files, directories, locks, or temporary state and is never mutation authority.

This is targeted protection, not universal alias discovery. No global mount
inventory or protected-tree scan is performed. Unseen inverse aliases into
excluded files, other private-state entries, or unscanned objects may not be
detected. A legitimate bind mount or unusual layout is not automatically
adversarial. If a known required safety observation fails, inspection refuses
that import rather than certifying the missing evidence.

These coverage limits are separate from the cooperative concurrency contract:
external processes must not modify affected paths, ancestors, input contents,
metadata, or source selection during authoritative operations. Final checks
catch observed changes; they cannot close every TOCTOU window or make a
previously unknown alias visible. M4 has not implemented authoritative import
execution, registry v2 production, or a public `dots add` command.

## Read-only registry-claim preflight

Import inspection reuses the bounded strict versioned loader and private-state
validation. A missing state directory, missing registry and lock, or a safe
lock without a registry is empty history. A valid snapshot with no records or pending imports
is also empty history. An existing empty file is not a valid empty snapshot.
Malformed, duplicate, unsupported, oversized, unsafe, or inaccessible state
refuses inspection. A registry without its stable lock requires independent
recovery; inspection never creates or repairs that lock.

Both the input and proposed source slot are compared with every recorded
ownership reservation, regardless of profile, current target, historical
target, or symlink presence. Comparisons reuse recorded parent and ancestor
identities and native name rules, including prospective missing parents.
Literal ancestor/descendant reservations and verified backing ancestry are
also retained. Historical paths and referents are not traversed: disappearing
symlinks, unavailable historical parents, and changed configuration do not
silently discard recorded claims. Unrelated owned or invalidated history does
not block an import. Relevant ambiguous name evidence refuses it.

The global unfinished marker blocks admission without recovery. Unresolved
`creating` or `removing` records also block admission, even with a clear
unfinished marker, because they retain incomplete-operation evidence. An
`invalidated` record is still a reservation but does not create a global
barrier by itself. Inspection never adopts links, retires stale records,
invalidates authority, or changes phase markers. Existing M0–M3 commands
retain their existing behavior.

The preview contract remains **no lock acquisition**. An atomically published,
clean snapshot may be inspected while a cooperating writer holds the
exclusive lock; this neither steals that lock nor proves the snapshot is the
newest one. Existing lock metadata and the snapshot's named identity are
observed and rechecked around loading and before return. Missing-state
frontiers and known ancestry are rechecked as well. Observed publication,
replacement, disappearance, or uncertainty refuses inspection; later errors
are joined with earlier failures. All descriptors are closed.

These checks are read-only observations, not serialization or mutation
authority. They cannot detect every intervening publication or a change after
return. Actual relocation still requires separate transactional authorization,
an exclusive stable lock, fresh authoritative state, and repeated filesystem
preflight. No registry upgrade, import-intent production, synchronization, recovery write,
or public `dots add` command is implemented here.

## Registry v2 reader and mutation gates

V1 retains exactly `version`, `revision`, `unfinished`, and `records`; it
prohibits `imports`. V2 requires those fields plus a nonnull `imports` array
with zero or one entry. A pending import requires `unfinished = true`.
Parsing dispatches by version and structural position, rejecting duplicate,
unknown, missing, null, malformed, and contradictory evidence. Existing M3
ownership-record validation is unchanged.

An intent contains `id`, `phase`, `profile`, `target`, `root`, `input`,
`source`, `input_binding`, `source_anchor`, `original_file`, and `digest`.
Pathnames retain M3 base64 encoding. The identifier is 32 lowercase hex
characters; the digest is `sha256-v1` with 64 lowercase hex characters.
Bindings contain `parent` and `routes`; the source anchor also contains a
literal `missing` parent suffix. File observations contain `fingerprint`,
`uid`, `gid`, and `nlink`, requiring a regular, single-linked file without
privileged mode bits. Source bindings must extend the recorded anchor through
exact directory components. `source_binding` is optional only while source
parents are missing. `prepared` prohibits `relocated_file`; `relocated`
requires both complete fields and rename-stable original-file equality.
Change timestamps remain evidence but may differ across rename.

Every ordinary mutation, including empty-plan installation and failed-up
ownership revalidation, refuses pending imports across all scopes. Refusal
preserves registry and destination contents and reports both file locations.
Down preview also refuses removal authority. Read-only inspection takes no
lock and performs no recovery; ls, status, and up preview remain unchanged.

Ordinary writes preserve the loaded version, including an empty v2 imports
array. They cannot upgrade or downgrade it. The shared 16 MiB bound includes
import evidence and recovery expansion; reserved byte counts reject negative
or overflowing requests. Revision headroom is checked before writes and
ordinary transaction entry. Complete future-import capacity reservation must
be added with its separately authorized producer, before parent creation.

Import phases are diagnostic evidence, never restart authority. Recovery
support is not yet available: preserve both reported locations and ownership
state, stop affected writers, and seek explicit recovery guidance. Do not edit
registry JSON, replay temporary snapshots, delete the lock, or retry by
recreating a symlink. The protocol below describes future work, not an
implemented recovery mechanism.

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
