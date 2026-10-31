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
registry contents and historical claims now constrain admission. Public import preview and
execution remain unimplemented; execution will require fresh preparation under
the approved transaction protocol. Strict registry v2 reading and pending-import gates are implemented;
public `dots recover` provides separate administrative recovery, but no public
command produces import intents. Private Stage 2 preparation
now creates durable intent and source-parent evidence in isolated integration
fixtures; it has no public caller.

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
previously unknown alias visible. Private preparation and same-invocation
completion below implement import execution internally. A public `dots add`
command remains unimplemented.

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

Input and source slots must also be distinct in recorded physical namespaces.
For a shared parent identity, exact or potentially case/Unicode-equivalent
names are refused, including an input that occupies a missing source-parent
slot. Each endpoint is also compared with every opposite ancestry route entry;
a regular-file endpoint cannot double as an opposite directory or symlink route.
Parsing never visits historical paths to query directory capabilities;
ambiguous spellings are refused even if a live case-sensitive directory could
distinguish them. Historical ownership comparisons use the same slot rule.
For missing source parents, the anchor reserves the first missing component;
a complete source binding separately reserves the final source slot. Distinct
names and different recorded parents remain valid.

Every ordinary mutation, including empty-plan installation and failed-up
ownership revalidation, refuses pending imports across all scopes. Refusal
preserves registry and destination contents and reports both file locations.
Down preview also refuses removal authority. Read-only inspection takes no
lock and performs no recovery; ls, status, and up preview remain unchanged.

Ordinary writes preserve the loaded version, including an empty v2 imports
array. They cannot upgrade or downgrade it. The shared 16 MiB bound includes
import evidence and recovery expansion; reserved byte counts reject negative
or overflowing requests. Revision headroom is checked before writes and
ordinary transaction entry. Private Stage 2 now reserves the complete proposed import transaction before
parent creation; public import execution remains unimplemented.

Import phases are diagnostic evidence, never restart authority. Preserve both
reported locations and ownership state, stop affected writers, and inspect with
`dots recover` before explicitly selecting recovery. Do not edit registry JSON,
replay temporary snapshots, delete the lock, or retry by recreating a symlink.
The administrative recovery operations below never resume the import.

## Private transaction preparation (Stage 2)

The private `prepareImport` handle first refreshes resolution and performs full
read-only filesystem, namespace and ownership admission. Rejected preliminary
admission and every preview create nothing and perform no synchronization.
Existing pending imports, unfinished M3 transactions and unresolved
creation/removal records are refused without recovery.

After preliminary admission, missing private state directories and the stable
lock are initialized through M3's descriptor-relative bootstrap. Existing
ancestor bindings remain pinned and revalidated. New directories require 0700;
the exclusively created lock requires 0600, both subject to umask. Existing
permissions, files and symlinks are never repaired, replaced or removed. A
registry without its stable lock remains an error. Competing first-use attempts
may encounter a changed preliminary observation or lock contention; retry must
start fresh. Only the validated exclusive-lock holder may advance preparation.

Bootstrap may leave private directories and the stable lock after a later
failure. It never rolls them back, publishes an empty initialization snapshot,
or changes the original input. Partial initialization or synchronization
uncertainty fails conservatively and reports retained artifacts. Fresh admission
must validate them on retry; unsafe artifacts require separate investigation.

Preparation retains the exclusive lock, reloads state and invokes the resolver
callback again under that lock. Bootstrap refreshes only intentionally obsolete
absence observations; unexpected registry appearance, including during refreshed
observation capture, refuses admission without overwriting it. Existing registry
identity evidence is retained across lock acquisition. The locked registry/profile/target and
selected paths must match the preview. It repeats path and ownership-claim
inspection, retains descriptors/recheck evidence, hashes the original with
identity/metadata validation, then reserves snapshot and revision capacity.
No lower package imports the resolver; public CLI integration remains unwired.

The size bound includes realized routes with maximum identity widths,
relocated evidence, simultaneous import and M3 ownership capture, counter
growth and global invalidation. Eleven revision increments cover seven forward
publications plus invalidation, retirement and fresh begin/finish. This finite
logical reservation does not reserve disk space or promise unlimited recovery
retries; later stages must account for any additional publication.

After synchronizing the original file, held stable lock and state directory, preparation
publishes an unfinished barrier in the loaded version, then atomically publishes
v2 with a complete prepared intent. Existing ownership records are preserved.
Only then may it create source parents, using the selected mount-route descriptor,
exclusive mkdir, nofollow opens, exact spelling and private 0700 permissions.
Each new directory and containing binding is synchronized before proceeding.
The complete source binding becomes durable before preparation returns.

The handle retains the lock until Close. Closing or failing never clears the
barrier, retires intent, removes parents, restores files or resumes work.
Failures after publication preserve potentially visible snapshots and partial
parents; unrecorded parent progress requires conservative future investigation.
Preparation alone never relocates files or creates links. The private Stage 3
completion described below may consume its retained authority. Crash tests cover
process interruption, not physical power-loss behavior; synchronization guarantees
retain the documented filesystem limits.

## Private completion (Stage 3)

The private, single-use completion consumes only an uninterrupted preparation
holding the same exclusive lock. It repeats namespace, configuration, provider,
claim, permissions, source-slot, original identity and SHA-256 checks. Selected
parent routes are retained independently of inode deduplication. Metadata must
remain stable during each digest observation; only rename comparison permits
ctime changes.

macOS uses descriptor-relative `renameatx_np(RENAME_EXCL)`; Linux uses
`renameat2(RENAME_NOREPLACE)`. Existing occupants, cross-device operations and
unsupported exclusive semantics fail with no weaker fallback. After rename,
completion verifies the relocated identity/content and original-slot absence,
synchronizes the file and both parents, then publishes `relocated` evidence.
It durably publishes the associated M3 `creating` record before create-only
symlink installation, captures the live-created link through M3's shared helper,
synchronizes its parent, and durably publishes `owned` while retaining the import.
Only then does one atomic snapshot retire the intent and clear `unfinished`.
Every snapshot remains within preparation's byte and revision reservation.

The live handle retains the last durable registry file's identity, revision and
canonical content digest across preparation and completion. Strict read-only
reloads verify the entire expected snapshot without replacing proposed state.
Checks precede relocation and each publication, including immediately before
snapshot replacement. Evidence advances only after directory synchronization and
verification against the inode of the transaction's own temporary snapshot.
Missing, replaced or changed state stops the invocation; no new baseline is
adopted. Failed publication retains prior evidence and uncertainty diagnostics.

A failure closes authority without retry or rollback. Before any rename attempt,
the original remains in place. A rename error can leave its outcome uncertain;
a successful rename followed by sync/publication failure can leave a `prepared`
intent although the original is now at the source path. A failed link or owned
publication preserves the source, any created link, and pending evidence; an
observed matching link is never adopted. A failed final directory sync can leave
a visibly completed snapshot whose durability is uncertain. All such failures
report both locations and preserve available state; no automatic recovery or
parent cleanup occurs. Phases are diagnostics, never restart authority.

The implementation has no public caller. First-use bootstrap is private;
public `dots add` remains deferred; administrative recovery is exposed separately through `dots recover`. Native filesystem capabilities
are required; cross-builds do not substitute for native Linux validation.

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

Public commands are `dots recover`, `dots recover --retire INTENT_ID --revision N`
and `dots recover --settle --revision N`; append `--dry-run` to either action
for a strictly read-only preview. The default command first reads a summary,
then selects its observed intent ID/revision for detailed inspection. A change
between observations refuses; neither observation grants authority. No live
TOML is required and explicit `--config` is rejected. Stored paths are printed
with quoting; file contents are never printed.

Mutating actions repeat authoritative checks under the exclusive lock. Retirement
prints the verified surviving location and leaves the barrier; settlement must
be separately selected at the new revision. Preview explains intended registry
changes and blockers without locking or syncing. Successful inspection may
report blockers (exit 0); blocked selected previews/actions or operational errors
exit 1; invalid flag combinations/values exit 2. Both actions remain registry-only.
If recovery refuses, preserve occupants and evidence, investigate reported checks,
and do not manually edit registry JSON. See the [usage guide](../README.md#inspecting-and-recovering-interrupted-imports).

### Private read-only inspection

The recovery inspector backs public `dots recover`. It takes only the registry location,
independent of TOML configuration and current source selection. Summary mode
reports snapshot version/revision, the unfinished barrier, stored intent scope
and paths, and the number of owned/removing records whose authority would need
separate consideration. Summary mode never traverses recorded import or
historical ownership paths. Detailed inspection requires the exact stored
intent ID and revision; stale selections refuse before visiting those paths.

Detailed observations compare recorded parent identities and routes, inspect
both leaves without following symlinks, and check regular-file identity,
ownership, link count, metadata and SHA-256. Input-only requires exact original
evidence. Source-only requires a complete recorded source binding and exact
relocated evidence when present; in the prepared rename gap only the original
ctime may differ. Missing source parents are checked at their recorded absent
frontier; newly present unrecorded parents remain blocked. Symlinks, including
apparently correct links, are occupants and do not prove ownership.

Inspection takes no flock, creates no state and performs no synchronization.
It can observe a snapshot while a writer holds its lock, then rechecks registry
identity, revision and full contents, lock/privacy evidence, recorded bindings,
leaf metadata and absence witnesses before returning. Observed changes invalidate
candidate conclusions while preserving earlier diagnostic failures.
Selected named parents retain independent read-only descriptors rather than
using inode-deduplicated views for leaf checks. Final named-parent reopen checks
also compare child evidence through the selected path. Matching
observations cannot prove durability or the absence of every race or alias.
They carry no live capability and never authorize retirement or settlement;
mutating operations must repeat authoritative checks under the exclusive
lock. Barrier-only M3 state is distinguished from a pending import. Reading
valid state does not require spare future write capacity; capacity reservation
and synchronization remain requirements of future authorized mutation.

### Private administrative retirement

Retirement is exposed through `dots recover --retire INTENT_ID --revision N`.
It requires the exact stored intent
ID and expected registry revision, existing private state and the stable lock.
It never initializes state or consults current TOML, resolver output or current
source-root selection. A returned inspection result cannot authorize retirement.

The uninterrupted invocation acquires and validates the existing exclusive
lock, reloads strict v2 state and pins complete snapshot continuity. It verifies
the recorded file/ancestry/hash evidence before any publication, including the
same selected-parent view checks as inspection. It reserves two revisions and
checks the complete invalidation and retirement encodings, including recovery
expansion, before writing. This does not reserve disk space or later settlement
headroom.

Retirement first durably changes every `owned`/`removing` record to `invalidated`
across all scopes, without visiting historical locations. It preserves creating
and already-invalidated records, all recorded history, the complete import
intent and `unfinished=true`. After verifying that the previous file witnesses
still match, it repeats full file/hash inspection, synchronizes the survivor
and the two existing selected parents (or the source's recorded existing anchor
when parents are absent), and rechecks bindings, metadata, privacy and registry
continuity immediately before publication. Only then does it durably remove
the selected intent, keeping the unfinished barrier and invalidated history.

Success reports the verified surviving file location and that explicit barrier
settlement is still required. It does not complete the import, install a link,
restore ownership, remove reservations, move data, or settle the barrier. No
live handle is returned. Settlement requires the separately selected operation
described below; retirement never invokes it.

Known preliminary refusals leave the registry unchanged and create no state.
Failures after invalidation preserve the pending intent and revoked authority;
a failed final publication may leave retirement visibly published with uncertain
durability. Preserve all evidence and investigate rather than automatically
retrying or reconstructing an intent. Abrupt interruption never resumes this
invocation's authority. Any later separately authorized attempt must select the
then-current stored ID/revision and repeat all checks from existing state.

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

### Private barrier settlement

Private settlement clears only the registry's unfinished marker; retirement
removes an eligible import intent after revoking prior authority. Public
`dots recover --settle --revision N` requires an explicit expected registry revision,
existing safe private state and the validated stable exclusive lock, with fresh
strict loading and full snapshot continuity. No live TOML or historical user
path is consulted, and a preview grants no mutation authority.

No pending import may remain. To preserve every record field and phase without
restoring blocked authority, an unfinished snapshot containing any `owned` or
`removing` record refuses settlement across all scopes. Empty history and
`creating`/`invalidated` history may qualify in v1 or v2. This is intentionally
narrower than a general M3 begin/finish recovery: begin would invalidate those
records and violate settlement's record-preservation contract. Such blocked
history needs separate investigation or authorized invalidation, not JSON edits.

Settlement reserves one publication's revision and complete snapshot capacity,
then uses ordinary finish/save to atomically publish `unfinished=false` and the
revision increment. The loaded version and all records remain unchanged. It
never begins an import, retires an intent, removes reservations, repairs user
paths or restores authority. Retained creating/invalidated records still need
their usual handling; a cleared barrier does not certify filesystem repair.

Missing state is a no-op only with revision zero, meaning absence rather than a
verified snapshot revision. An existing clean snapshot still requires its exact
revision and a validated existing lock. No-op results create no state, write no
snapshot and perform no synchronization; they do not retroactively certify an
earlier uncertain publication. Missing locks for existing state, unsafe paths,
stale revisions, and detected changes refuse without repair or initialization.

The existing read-only recovery summary now reports the same settlement blockers
without acquiring flock, writing or synchronizing. Actual settlement repeats
admission under lock. Publication failures preserve available evidence; failure
after replacement may leave clearance visible with uncertain durability. No
retry, reconstruction, temporary-snapshot replay or cleanup is automatic. The
CLI requires explicit authorization for this separate operation; public
`dots add` remains unavailable.

## Command contract

Proposed usage: `dots add [--config FILE] [--root DIR] [--dry-run|-n] PATH`.
Require exactly one input. Exit 0 for completion or a valid preview,
1 for conflicts or operational uncertainty, and 2 for invalid usage.

Preview reports the move, link, selected root, and intermediate data
location. It performs no initialization, locking, synchronization for
recovery, state writes, temporary-file creation, or filesystem mutations.

Actual output distinguishes completed relocation, symlink creation, and
ownership publication. Output or persistence failure does not undo writes.
