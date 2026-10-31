# Ownership and removal safety

M3 implements the approved [ownership policy](https://github.com/fernandoaleman/dots/issues/4)
and [removal command](https://github.com/fernandoaleman/dots/issues/5).

## Cooperative concurrency contract

External processes must not modify affected destination paths or ancestors
during `up` creation and ownership registration, or during `down` validation
and removal. The registry lock coordinates cooperating dots processes only,
not editors, shell commands, synchronization tools, or other writers.

Directory descriptors and immediate identity checks reject observed
replacement and alias retargeting. They cannot freeze names. A late rename
can relocate a pinned directory. There is no portable atomic
compare-and-unlink: another process can replace the checked leaf before
`unlinkat`. Creation also has a gap between `symlinkat` and identity capture.
The approved cooperative assumption covers both gaps. This implementation
does not guarantee protection against arbitrary concurrent replacement.

## Registry and evidence

State is machine-local at `$XDG_STATE_HOME/dots/ownership.json`. Unset,
empty, or relative `XDG_STATE_HOME` falls back to
`$HOME/.local/state/dots/ownership.json`. Preserve native symlink-plus-`..`
traversal rather than lexically cleaning locators.

Dedicated dots directories require user ownership and `0700`; regular
state/lock files require user ownership, `0600`, and one hard-link name.
Do not change existing HOME/XDG ancestor permissions. Unsafe permissions,
symlink state files, a symlink dots directory, corruption, and unsupported
versions block mutation. State/source overlap and destination/state overlap
are rejected using physical paths and native name equivalence.

State creation respects the process umask. A mask that removes required owner
permissions can leave a newly created directory or lock unusable. Bootstrap
fails before changing managed destinations; it does not automatically chmod,
delete, or replace these entries. Changing the umask alone will not repair an
entry already created with unsuitable permissions. Use a mask that preserves
owner permissions, such as `0022` or `0077`.

The version 1 JSON snapshot requires `version`, `revision`, `unfinished`,
and `records`. Records contain exact configuration/target locators,
destination, absolute link text, phase, parent identity, ancestor bindings,
and leaf fingerprint. All pathname/text fields use base64 to preserve
filesystem bytes; identity numbers use typed integers, not floating point.
Duplicate keys/destination claims, unknown/null fields, and missing required
evidence are errors.

Leaf evidence includes device/inode, symlink mode, size, and modification/
change timestamps with nanosecond fields. Exclude access time. Ancestors
retain directory identities and alias entry identity/text/resolved identity.
Directory timestamps/link counts need not remain unchanged as children change.

Fingerprints are conservative evidence, not an absolute guarantee against
inode reuse or proof of uninterrupted name binding. Metadata changes,
backups, remounts, or lost state can make links ineligible. User-owned private
state is trusted; malicious same-user/root manipulation is outside the model.
Do not synchronize state between machines or restore old snapshots as current
authority.

Only a successful creation in the current live invocation can grant
ownership. M1/M2 links and manually created links remain untracked even when
`up` reports `OK`. Matching a source proves correctness, not creation history.
Source membership is a safety constraint, never ownership evidence.

## Transaction protocol

Actual commands securely open the stable `ownership.lock`, acquire a
nonblocking exclusive lock, then load authoritative state. Contention fails
clearly. Keep close-on-exec descriptors through persistence, revalidate the
named lock identity, and never replace/unlink the lock. Existing registry
with a missing lock requires manual recovery.

Persist and synchronize `unfinished = true` before ownership validation or
destination mutation. Keep it in every intermediate snapshot; clear it only
as the final successful transaction step.

| Phase | Meaning and recovery |
| --- | --- |
| `creating` | Persist intent after parents are available, before creating the symlink. An occupied name never promotes automatically. Verified absence beneath unchanged ancestry permits retiring the intent. |
| `owned` | This invocation created the link, captured its identity, synchronized its parent, and published ownership. Removal still requires matching identity, text, and recorded ancestry. |
| `removing` | Persist before final validation/unlink. Confirmed absence permits retirement after parent synchronization. Never automatically remove a present entry from an interrupted transaction. |
| `invalidated` | Authority disabled permanently for this record. Preserve a present occupant even if the original fingerprint returns. Verified absence can retire the record; later fresh creation uses new live evidence, not reactivation. |

Pending v2 imports block ordinary mutations before this recovery, including
registry-only changes and empty-plan/history paths. Recovery support for
imports is not yet available; preserve state and both reported file locations.
Do not modify registry JSON or infer authority from recorded import phases.
See [the v2 reader boundary](IMPORT.md#registry-v2-reader-and-mutation-gates).

On startup with an unfinished snapshot **without pending imports**, mutating commands conservatively
invalidate all prior `owned`/`removing` records before continuing, without
visiting historical targets. `creating` remains nonauthorizing. This can
revoke removal authority for intact links unrelated to the failure. There
is no automatic reconstruction or adoption. An `up` retry may skip a correct
invalidated link while installing unrelated missing links; skipping does not
restore ownership.

Persist observed mismatches as invalidated even when destination preflight
fails. If that write fails, the earlier durable unfinished barrier prevents
old authority from silently reviving. Report failed revocation persistence
as uncertain.

Write snapshots to unique, exclusively created `0600` temporary files in the
pinned private directory. Write the complete snapshot, synchronize/close it,
atomically rename it over the registry, then synchronize the directory.
Synchronize newly created state directories and their parents. Synchronize
destination parents before committing creation or retiring removal.
After creating each missing destination directory, synchronize its containing
directory before proceeding, so newly created ancestor bindings precede ownership.
Required synchronization, locking, and publication failures are fatal; no
silently weaker fallback is available.

Snapshots have a shared 16 MiB read/write limit. Admission reserves space
for global invalidation, counter growth, and the captured leaf fingerprint,
so a successful publication cannot exhaust the space needed to revoke authority.

Atomic visibility differs from power-loss durability. Recovery assumes the
stated filesystem/trust model; power-loss persistence depends on filesystem,
OS, and hardware guarantees. Rename followed by failed directory sync may
already have published the snapshot. Report uncertainty rather than claiming
the old snapshot necessarily remains.

Stop on the first runtime/persistence failure. Count successful symlink or
unlink calls even if persistence later fails, retain unresolved records, and
report later entries as unattempted. Do not roll back completed operations.
Failed private temporary snapshots remain for inspection; never replay or
automatically clean them up.

## Selection and filesystem safety

The exact expanded absolute config locator identifies a profile; its exact
expanded target selects the current target. Aliases/moving the config do not
transfer ownership. Config must be readable and valid, but `down` does not
scan sources. Root reordering, exclusions, removed entries, missing roots,
and broken referents do not redefine ownership.

Historical targets remain recorded/reported without automatic visits or
removals. Explicitly returning the same config to a previous target selects
its records again, subject to full validation.

Removal compares selected paths with all recorded physical slots and ancestor
bindings, including other profiles and historical targets. An overlapping claim
or unavailable required name-equivalence capability blocks removal. Historical
locators are not traversed for these comparisons; only selected paths are opened.
Repeat the claim check immediately before unlinking.

Preflight the complete selected plan before unlinking. Reject blocked or
changed ancestry and observed identity/text changes. Protect current source
roots. Immediately before unlinking, revalidate state/lock bindings, parent
routes, leaf fingerprint, symlink type, and exact text. Use a pinned parent
descriptor and one basename with `unlinkat`, without directory flags.
Never follow the leaf referent, remove files/directories under non-racing
operation, recursively delete contents, or clean up parents. Owned broken
links remain eligible when their own evidence matches.

Private import preparation performs preliminary read-only admission before
initializing missing state through these same M3 bootstrap primitives. It keeps
the pinned namespace, validates and acquires the stable lock, reloads state and
repeats resolver/filesystem/claim admission under that lock. It synchronizes the
held lock and directory before publishing the first barrier. No empty registry
is published for initialization; absent state starts in v1 and upgrades only
when the complete prepared v2 intent is published.

Rejected preliminary admission and previews remain zero-write. Once private
bootstrap starts, failures may retain state-only directories and the stable
lock, including partial initialization. They never trigger automatic cleanup,
permission repair, source-parent creation before durable intent, or input-file
changes. Concurrent first-use attempts must retry changed observations or busy
locks; they never replace an existing lock. Existing registry state without its
lock still refuses mutation.

Private import preparation can hold this same lock while publishing an
unfinished barrier, v2 intent and source-parent bindings. It preserves existing
ownership records. Its private Close only releases resources. Stage 3 can
consume the same uninterrupted lock/capability to exclusively relocate the file,
publish relocated evidence, then reuse M3 record construction and live-link
capture. Ordinary pending-import gates are unchanged. The import remains pending
through durable `creating` and `owned` publication; only final atomic completion
retires the intent and clears the barrier. Failures preserve available evidence,
the source file and any created link; they never resume, adopt, roll back or
clean up parents. A rename or final snapshot-sync error may have an uncertain
outcome. No public import command or import recovery is provided. See
[private completion](IMPORT.md#private-completion-stage-3).

Private imports also retain the last durable registry snapshot's identity,
revision and complete-state digest. They revalidate it across the preparation
handoff and before later publications, rather than trusting a new observation.
Only verified durable publication refreshes this evidence. Unexpected state is
preserved and reported; it is never reconstructed or overwritten to resume work.

## Read-only operation and recovery

Private recovery inspection now reuses the strict state reader and lock-free
registry observations using only a registry locator. It reports v1/v2 state,
unfinished barriers and pending intent evidence without consulting current
configuration, acquiring flock, synchronizing, initializing or changing state.
Exact stored ID/revision selection is required to inspect import locations;
history is summarized without visiting unrelated recorded paths. Structured
checks distinguish an observational retirement candidate from mutation authority.
Final registry, lock, route and leaf observations reject detected changes.
Private retirement and barrier settlement are separate operations under the
existing exclusive lock; public recovery commands remain unimplemented.
See [private inspection](IMPORT.md#private-read-only-inspection) for evidence
checks and limitations. Do not edit registry JSON to retire an intent.

Retirement requires the exact stored intent ID and registry revision without
current configuration. It verifies a single intact survivor and an absent
counterpart, reserves both publications, durably invalidates all prior owned/
removing authority across scopes while retaining history and the intent, then
repeats file evidence checks and synchronizes the survivor and existing parents.
Final publication removes only that intent and retains `unfinished=true`.
Creating and invalidated records are preserved; no authority is restored and
no user entry is moved, removed, adopted or replaced. Failure preserves available
registry evidence, including uncertain published outcomes; no retry or settlement
is automatic. See [private retirement](IMPORT.md#private-administrative-retirement).

Private settlement accepts an exact expected revision and clears only an
eligible unfinished barrier using ordinary finish/save, preserving version and
every historical record and phase. Pending imports always block. Unfinished
owned/removing history also blocks: clearing its barrier unchanged could restore
authority, while calling begin would alter the record states. Only empty or
creating/invalidated history qualifies. Preview reuses the read-only summary;
actual settlement acquires the existing lock and independently revalidates.
Missing/clean no-ops initialize and rewrite nothing. Settlement does not repair
user paths or resolve retained reservations; any uncertain publication requires
investigation rather than automatic retry. See [private settlement](IMPORT.md#private-barrier-settlement).

`down --dry-run` opens only existing state. It never initializes or acquires
a lock, retires records, replays transactions, or creates temporary files.
Unfinished state cannot authorize removal of a present entry. Its output is
a point-in-time observation of an atomic snapshot. `up --dry-run`, `ls`, and
`status` retain their existing state-independent read-only behavior.

Actual commands can update administrative state even with no dotfile changes.
Summaries count managed destination changes, not registry bookkeeping.
Untracked paths are not broadly scanned; no eligible records does not mean
no symlinks exist.

For unresolved ownership, stop other dots/namespace writers and preserve the
registry, lock, temporary snapshots, and reported paths for investigation.
Do not replay a temporary snapshot, manually set `owned`, or recreate/delete
a lock while a process may hold it. Fix ordinary access problems without
changing unrelated permissions. Corrupt/unsupported state and missing locks
require deliberate manual recovery; dots never resets them automatically.

For a known failed first-time bootstrap, first stop all dots processes and
other writers. Verify that no authoritative registry was published and that
the affected entries are the new, user-owned directories or single-link regular
lock file from that failed attempt, with no symlink substitutions. Only then
restore required permissions on those specific entries in place: `0700` for
directories and `0600` for the lock. Preserve the lock's identity; never delete
or recreate it. Retry with an owner-permission-preserving umask. Do not apply
this procedure to unverified entries or unrelated HOME/XDG ancestors; an
existing registry or uncertain provenance requires the broader recovery review
above. Bootstrap tests use only disposable fixtures for this manual repair.

Present invalidated or legacy links require independent user verification
before any manual removal. M3 has no adoption, force, or repair command.
After a destination is independently made absent, dots may retire its record
only through unchanged recorded ancestry. A later `up` can create a fresh
tracked link. Missing/replaced ancestry remains a preserved conflict rather
than evidence of successful removal.
