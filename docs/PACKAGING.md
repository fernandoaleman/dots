# Local release-candidate packaging

This is nonpublishing validation, not an official v0.1.0 release. No command here
creates tags, GitHub Releases, package-manager definitions or public assets.

The compiler pin is centralized in `scripts/package.py`; print it with:

```sh
python3 scripts/package.py compiler
```

It currently requires Go 1.26.10. Provision that compiler and the modules in
go.sum first; packaging never silently downloads or switches compilers. Select
its executable explicitly if your ordinary `go` is another version. Go 1.23
source compatibility and ordinary source-build development remain unchanged.

```sh
make package VERSION=v0.1.0 COMMIT="$(git rev-parse HEAD)" GO=/path/to/go1.26.10/bin/go
```

Only stable `vMAJOR.MINOR.PATCH` versions and complete lowercase source commit
IDs are accepted. The source commit must exist locally. The script builds a
clean independent temporary clone at that exact commit, without using dirty or
untracked application files, registering a worktree, sharing Git objects, running
checkout hooks or changing the original repository. The current packager and
docs/INSTALL.md are separate packaging inputs; their hashes are recorded in the
printed build report. The LICENSE and executable come from the requested commit.

Go's `-trimpath` intentionally omits linker flags from build info. The script
injects version/full commit with the existing linker strings, checks standard
Go `vcs.revision` and `vcs.modified=false` evidence, and verifies compiler,
target, CGO and trimpath settings. Go may record the stable source commit time;
no current wall-clock timestamp is injected. `go version -m EXTRACTED_BINARY`
can inspect this evidence. Native acceptance separately checks both version forms.

The default new output directory is `dist/v0.1.0`. Override it with
`DIST=dist/repeat`. Outputs must be under this checkout's ignored dist/ or build/
directories. Existing paths are refused, including empty directories and symlinks.
Construction uses private staging, then exclusive output creation. SHA256SUMS
is copied last. Failures can leave incomplete outputs; verify them and choose a
fresh output directory rather than treating an existing directory as success.

The five output files are:

- dots_0.1.0_darwin_amd64.tar.gz
- dots_0.1.0_darwin_arm64.tar.gz
- dots_0.1.0_linux_amd64.tar.gz
- dots_0.1.0_linux_arm64.tar.gz
- SHA256SUMS

Each archive has exactly INSTALL.md, LICENSE and dots, in that order. It uses
USTAR, fixed modes (0644 documents, 0755 executable), zero uid/gid/timestamps,
empty owner names and a gzip header without a filename or current time.

## Verify and execute the extracted artifact

```sh
python3 scripts/package.py verify --version v0.1.0 --directory dist/v0.1.0 \
  --extract-target darwin/arm64 --extract-to build/extracted &&
python3 scripts/acceptance.py --binary build/extracted/dots \
  --expected-version v0.1.0 --license build/extracted/LICENSE
```

Select the actual host OS/architecture; never count foreign or emulated execution
as native. Verification checks all four checksums before opening any archive,
then checks exact inventory, metadata and binary formats before extraction to a
new ignored directory. Checksums detect corruption, not independent authenticity.
The acceptance mode copies those executable bytes into its private PATH and
never falls back to a build. Missing, incompatible, non-executable or wrong-version
binaries fail. Ordinary `python3 scripts/acceptance.py` still builds from source.
See [installation notes](INSTALL.md) for manual checksum and binary placement.

## Repeatability and platform evidence

Run packaging twice with the same version/commit into two new output directories
and compare SHA256SUMS. The build report records compiler, source commit,
Python/zlib versions, packaging-input hashes and relevant Go settings. Builds use
CGO=0, trimpath, read-only modules, a clean source snapshot and fixed architecture
baselines. After dependencies/toolchains are provisioned, set GOPROXY=off and
GOSUMDB=off for offline builds and verification.

Repeated hashes establish repeatability for those recorded inputs/environment;
they do not promise identical results across arbitrary compilers, Python/zlib
versions or hosts. No local checkout path is intentionally embedded.

The read-only CI packaging matrix builds twice and executes matching extracted
artifacts on Intel/ARM macOS and amd64/ARM64 Ubuntu runners. It preserves existing
checks and source acceptance, uploads no assets and needs no publishing secrets.
Cross-build success alone is not runtime evidence. Go 1.26 supports macOS 12,
but neither newer native macOS runners nor a Mach-O deployment target prove that
dots works on Monterey. Record that gap until native macOS 12 testing exists.
Darwin binaries depend on system libraries; Linux filesystem/kernel limitations
and capability-dependent acceptance skips remain documented in TESTING.md.
