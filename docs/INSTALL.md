# Installing a dots archive

These archives may be local release candidates. Packaging does not publish a
release. Use only artifacts from an explicitly trusted source.

Select darwin/amd64 for Intel macOS, darwin/arm64 for Apple Silicon, or the
matching Linux architecture. Verify the selected archive against SHA256SUMS
before extracting it, using `shasum -a 256` on macOS or `sha256sum` on Linux.
For example, with a local candidate and its checksum manifest:

```sh
(
  checksum_line="$(grep '  dots_0.1.0_darwin_arm64.tar.gz$' SHA256SUMS)" &&
  printf '%s\n' "$checksum_line" | shasum -a 256 -c - &&
  extract_dir="$(mktemp -d)" &&
  tar -xzf dots_0.1.0_darwin_arm64.tar.gz -C "$extract_dir" &&
  printf 'Extracted candidate: %s\n' "$extract_dir" &&
  "$extract_dir/dots" version &&
  "$extract_dir/dots" help
)
```

The archive contains dots, LICENSE and this document. Copy only the executable
to a writable directory on PATH, then confirm which executable is selected and
run `dots version`. Installation does not require running up, add or recover.
For upgrades, verify the replacement first and retain the previous executable
separately. Replacing a binary does not roll back dotfile operations or registry
state. Do not restore stale ownership snapshots or manually edit ownership.json.

Go 1.26 supports macOS 12, but this alone does not establish application support
on Monterey. Check the project's recorded native test results for the actual OS
versions exercised. Darwin binaries use system libraries; CGO-disabled does not
mean universally static. Linux requires compatible kernel/filesystem operations;
cross-compilation alone does not establish runtime compatibility.

No automatic updates, package-manager installation, signing, notarization or
recovery actions are performed by this archive.
