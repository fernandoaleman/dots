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

On startup with an unfinished snapshot, mutating commands conservatively
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

Preflight the complete selected plan before unlinking. Reject blocked or
changed ancestry and observed identity/text changes. Protect current source
roots. Immediately before unlinking, revalidate state/lock bindings, parent
routes, leaf fingerprint, symlink type, and exact text. Use a pinned parent
descriptor and one basename with `unlinkat`, without directory flags.
Never follow the leaf referent, remove files/directories under non-racing
operation, recursively delete contents, or clean up parents. Owned broken
links remain eligible when their own evidence matches.

## Read-only operation and recovery

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

Present invalidated or legacy links require independent user verification
before any manual removal. M3 has no adoption, force, or repair command.
After a destination is independently made absent, dots may retire its record
only through unchanged recorded ancestry. A later `up` can create a fresh
tracked link. Missing/replaced ancestry remains a preserved conflict rather
than evidence of successful removal.
