# Milestone 0 review ledger

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
