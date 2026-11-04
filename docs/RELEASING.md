# GitHub release procedure

This is a future release procedure. No release tag, GitHub Release, environment,
or repository setting is created by implementing this document. A release and
each repository-settings change require separate explicit authorization.

## One-time prerequisites

Protect the default branch and workflow files from unreviewed changes. Create
an environment named `release` with Fernando's personal GitHub account as its
sole required reviewer, **Prevent self-review disabled**, and administrator
bypass disabled. Restrict deployments to the `master` branch with a custom
deployment branch policy named exactly `master`. GitHub's native environment
gate, not the supplemental run-history check, is the publishing approval gate.
The public repository supports required-reviewer environment protection. If a
required field or protection cannot be verified, the workflow refuses to publish.

Create an active tag ruleset including `refs/tags/v*`, with no exclusions and
creation, update and deletion restrictions enabled. Add only Fernando's personal
user account as an always-allowed bypass actor so he can create an explicitly
authorized tag. Do not add the GitHub Actions application, integration, a broad
repository role or any other bypass actor. Do not assume repository administrators
are automatically bypass actors. Do not add an administrative credential to CI.

The Actions read token may not receive the ruleset's bypass-actor list. The
workflow checks the visible active rules and, when the bypass list is returned,
checks it too. **It cannot prove an omitted bypass list is safe.** Before
approving each protected deployment, the maintainer must inspect GitHub settings
and confirm that the active ruleset covers the exact tag, all three restrictions
are enabled, the Actions identity cannot bypass, and the maintainer is the only
intended bypass actor. The maintainer must also verify the exact tag-to-commit
mapping and that no conflicting release or draft exists.

## Request a release

1. Obtain explicit approval for the stable `vMAJOR.MINOR.PATCH` version and full
   source commit SHA. Confirm that commit is in reviewed master history and has
   passed the complete master CI matrix.
2. After separate authorization, the maintainer creates the version tag pointing
   to that exact commit, then verifies the tag and ruleset on GitHub. Automation
   does not create, update, force-push or delete tags.
3. Manually dispatch **Release candidate** on `master`, supplying that version
   and full commit SHA. Select `publish: false` for nonpublishing validation, or
   `publish: true` only after publication has been explicitly authorized.
4. Validation checks workflow/run identity, the live master, tag, source history,
   previous release conflicts and successful CI. The read-only token may not see
   drafts; the protected publishing job repeats conflict checks with its write
   token before creating anything. Publishing requests additionally require the
   protected environment and visible tag rules. Packaging checks out
   the selected commit, so the Stage 2 packager, installation instructions,
   license and binary source all come from that commit.
5. One artifact set is generated with Go 1.26.10 and uploaded as an internal
   workflow artifact. Four native jobs download that same artifact ID, verify
   SHA256SUMS and extract their native binary, then run acceptance through PATH
   with isolated HOME/XDG. The publishing job receives the same artifact ID and
   checksums; it does not rebuild. Cross-filesystem and capability skips remain
   explicit. macOS 12 native compatibility has not been verified.
6. For a publishing request, inspect the tag, release conflicts, ruleset and
   bypass actors as above **before** approving the pending `release` deployment.
   Only the protected publication job has `contents: write`. A rerun attempt is
   refused; use a fresh dispatch and obtain fresh approval.

The protected job rechecks environment, tag, history and artifacts, creates a
draft through GitHub's release API after verifying the existing tag, uploads
the exact tested files by the returned release ID,
checks remote metadata and downloads each asset for a byte-hash comparison, then
revalidates before publishing. Existing releases or drafts cause refusal; assets
are never overwritten. A failure or cancellation after draft creation can leave an
incomplete draft or uploaded assets. GitHub may reject an older selected commit
whose workflow files differ from master because the Actions token cannot obtain
workflow-write permissions. Preserve incomplete drafts for explicit operator review;
do not automatically delete, retry over or replace them. After separately
authorized cleanup or disposition, start a fresh workflow dispatch.

Tag verification, ruleset checks, upload checks and environment checks are not a
single atomic transaction. Settings or refs can be changed between checks by a
privileged actor. Native tag rules protect against an Actions-token tag creation
race only while correctly configured and without an Actions bypass. Maintainer
verification is therefore a mandatory operational prerequisite, not a claim that
the workflow automatically proves hidden bypass settings.

## Verify and pause after publication

Download the official assets and SHA256SUMS through GitHub, verify the manifest
and each archive checksum, and run the extracted executable on the intended
platform. Do not describe a release as immutable unless repository immutability
settings have separately been enabled and verified. If an already published
artifact is defective, prepare a reviewed correction and **new version** rather
than replacing published bytes or moving its tag.

After the first official v0.1.0 GitHub release, **stop distribution work**. The
maintainer will personally download the macOS binaries and try installation,
version/help, configuration, previews, import, list/status, down/up, recovery,
and errors on their own machines using harmless files and disposable settings.
Homebrew and AUR work require explicit approval after that personal testing;
a GitHub release is not authorization to begin package-manager distribution.
