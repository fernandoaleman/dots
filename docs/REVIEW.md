# Review ledger

Herdr specialists performed read-only architecture, implementation, and test
reviews. The lead remains the only file editor.

| Finding family | Scenarios | Implementation guard |
| --- | --- | --- |
| Path traversal | Stored link targets, explicit configuration, source and target paths | Preserve symlink-plus-parent traversal until filesystem resolution |
| Destination namespace | Directory aliases, Unicode normalization, case-equivalent structural paths, and empty source directories | Preserve directory evidence through native destination inspection without changing source winners |
| Explicit configuration | Empty, missing, malformed, or unreadable explicit paths | Reject errors without canonical fallback |
| Source identity | Separate hard-link names sharing an inode | Compare resolved parent directories and selected entry names; preserve directory aliases |
| Coverage | Permission failures, losing candidates, enumeration order, collision rows and summaries | Temporary fixtures, explicit assertions, and filesystem snapshots |
| Collision-check scaling | Thousands of unrelated sources and destinations | Index literal source destinations and normalized physical prefixes before comparing possible collisions; retain scaling benchmarks |

Native name queries use macOS path configuration and filesystem type for
APFS/HFS+, and Linux ext4 inode flags. Canonical Unicode normalization and
full folding provide conservative comparisons; filesystem Unicode versions
may differ. Unsupported filesystems report uncertainty when resolving a
potentially equivalent collision requires unavailable capability information.
Validation outcomes are reported separately after running the checks.

## Milestone 1 design reviews

The architecture and testing specialists reviewed prepared operations before
implementation. The approved design retains real symbolic links and the three
package boundaries. It uses Go 1.23-compatible `golang.org/x/sys v0.35.0`
descriptor-relative primitives, without a filesystem framework.

| Finding family | Required implementation or test |
| --- | --- |
| Check/use races | Single-component operations, pinned directories, named identity revalidation, and late ancestor-substitution sentinel tests |
| Partial work | Whole-plan preflight, fail-fast application, no rollback, and counts updated immediately after successful mkdir or symlink calls |
| Source stability | Detect observed entry/parent changes without fallback; permit in-place edits and document path-based symlink limits |
| Resource ownership | Deduplicated close-on-exec handles, single-use Apply, idempotent Close, and exhaustion/cleanup tests |
| Namespace limits | Test renamed pinned directories and directory substitution before and after the first identity observation |

The first implementation review found an existing-link validation gap: unchanged
link text and inode do not prove that aliases inside its target still resolve to
the selected source. Both the architecture and independent safety reviewers
identified this issue. Validation now compares target-to-source named entries
before any writes and before reporting an existing link as correct. Regression
fixtures retarget an alias to a different file or a missing target, both before
application and after an earlier successful operation.

The testing review requested additional coverage for ancestor substitution at
the mkdir boundary, dry-run independence from prepared directory resources,
runtime descriptor exhaustion, and aliased-parent handle deduplication. These
fixtures are included.

All three specialists completed follow-up static reviews with no remaining
actionable findings. Architecture confirmed package boundaries, descriptor
ownership, and portability; testing found coverage for all 14 requested
scenarios; independent review confirmed the corrected existing-link validation
and the approved create-only safety contract. Specialists did not edit files
or execute tests.

Local formatting, vet, tests, Staticcheck, build, race tests, and prek passed,
including hooks over untracked commit candidates. Go 1.23 tests, Linux vet,
and macOS/Linux cross-builds passed. These were the local review-readiness
results. Milestone 1 subsequently merged through
[the installation PR](https://github.com/fernandoaleman/dots/pull/1), with
[successful post-merge macOS/Linux CI](https://github.com/fernandoaleman/dots/actions/runs/40157259345).

## Milestone 2 design reviews

Architecture and testing reviewed the authoritative listing and status issues
before Go implementation. Both recommended resolver-only listing, shared
inspection for status and up, and additive structured classifications without
parsing human-readable reasons. The implementation keeps the three-package
dependency direction and adds no dependencies.

| Finding family | Implementation or verification |
| --- | --- |
| Inspection uncertainty | Error-aware named-entry comparisons and typed capability failures distinguish ERROR from confirmed wrong links and collisions |
| Combined diagnostics | Source scan failures remain distinct from structural conflicts; error classifications take priority while reasons are retained |
| Read-only behavior | Snapshot fixtures cover mapping, destination states, configuration errors, output failures, native permissions, and physical collisions |
| Compatibility | Existing up and dry-run tests remain, with cross-command selected-source assertions and unchanged legacy action labels |
| Output and exits | Explicit status labels, fixed summary order, deterministic enumeration, output-failure handling, and missing-link status exits |

Final read-only implementation and coverage reviews are separate from these
initial design recommendations. GitHub issues remain the progress record;
local validation alone does not complete the milestone.

The testing review requested combined-diagnostic fixtures. Source failure plus
physical collision now verifies retained explanations, source identity, exact
counts, and ERROR priority. Confirmed collision plus capability uncertainty is
tested in both input orders with an unaffected unrelated entry.

Independent review found that a symlink target traversing a regular file returns
ENOTDIR and was incorrectly classified as an inspection error. Destination links
now report BROKEN and ancestor links BLOCKED for that condition. Permission and
resource failures remain ERROR. Filesystem and cross-command snapshot fixtures
cover both forms, including the legacy nonzero dry-run conflict behavior.

Final follow-up reviews from all three specialists found no remaining actionable
findings or material coverage gaps. Local formatting, vet, tests, Staticcheck,
build, race tests, and prek passed, including the new untracked files. Proposed
commit messages passed the committed hook. Go 1.23 tests, Linux vet, and supported
cross-builds passed. These are local review-readiness results; native macOS/Linux
CI and formal closure follow the approved commit/push/PR/merge workflow.

## Milestone 3 design and implementation reviews

The user approved a private creation registry and cooperative concurrency
during both installation/registration and removal. Architecture and safety
specialists reviewed the transaction protocol before mutation implementation.
Their unfinished-transaction refinement prevents failed revocation persistence
from silently reactivating old authority. Receipt/quarantine designs and
automatic adoption were explicitly excluded.

| Finding family | Resolution and verification |
| --- | --- |
| Uncertain authority | Durable unfinished barrier precedes validation; interrupted work invalidates prior authority, and occupied creation intents never promote |
| Schema ambiguity | Exact complete key sets reject duplicates, capitalization variants, unknown/null fields, and missing required evidence |
| Snapshot capacity | Read/write limits agree; every publication reserves the worst-case global invalidation encoding, with extra fingerprint capacity before creation |
| Scope and aliases | Historical claims use recorded slot identities without visiting old paths; source protection covers both root aliases and resolved contents |
| Removal boundary | Use the validated pinned parent directly, retaining immediate identity/text/ancestry checks and the documented final-name race limitation |
| Failure reporting | Preserve every selected row, successful syscall counts, recovery warnings on early returns, and completed/unattempted distinctions |
| Failed preflight authority | Local Codex review identified a CLI bypass; failed actual installation preflights now validate/revoke existing ownership without destination writes |
| Ancestor durability | Pre-push review identified missing synchronization of newly created parent bindings; synchronize each containing directory and test failures at every level before ownership publication |
| Source-boundary error classification | Copilot identified lost ENOTDIR identity when a source directory becomes a file; preserve the classified error so recorded broken links remain removable, with no-write and permission-error regressions |
| Snapshot permissions | Copilot identified that owner-restricting umasks could publish unusable state; validate each new snapshot before publication, with isolated up/down subprocess tests preserving prior registry bytes, identity, permissions, and removal authority |
| Inspection uncertainty reporting | Copilot identified an empty-records message after failed ownership inspection; require success before asserting empty state, with actual/dry-run error, empty-state, destination-preservation, and no-write regressions |
| Cross-scope removal claims | Copilot identified selected-only collision checks; compare all recorded slots/routes at preflight and before unlink, without visiting historical targets. Test case/Unicode equivalence, capability failures, distinct names, historical preservation, and late capability changes |
| Registry read diagnostics | Copilot identified a nil read-error diagnostic for oversized state; distinguish read failures from the 16 MiB limit, with no-mutation tests for both removal modes |
| Recovery coverage | Phase-targeted persistence failures, actual child-process interruptions, independent command contention, and cross-profile/historical invalidation fixtures |
| Read-only behavior | Snapshot tests include absent, malformed, unfinished, unsafe, and locked state; inspection commands remain independent of ownership state |

The lead incorporated verified findings and remained the only editor.
Final independent reviews from architecture, testing, and safety specialists
reported no remaining actionable findings or material coverage gaps under
the approved model. Specialists did not execute tests or modify files.
Process-exit fixtures test interruption recovery, not universal power-loss
durability. Required local validation and the closeout audit are reported
separately; formal completion still requires approved publication and CI.
