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
and macOS/Linux cross-builds passed. Native Linux execution and formal milestone
closure still require the approved commit/push and a successful CI run.
