#!/usr/bin/env python3
"""Construct and verify local release candidates; never publish anything."""

import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import platform
import re
import shutil
import struct
import subprocess
import sys
import tarfile
import tempfile
import zlib

GO_VERSION = "1.26.10"
ROOT = Path(__file__).resolve().parents[1]
TARGETS = ("darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64")
MEMBERS = ("INSTALL.md", "LICENSE", "dots")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def metadata(version, commit):
    require(re.fullmatch(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", version),
            "version must be stable vMAJOR.MINOR.PATCH, not a development identifier")
    require(re.fullmatch(r"[0-9a-f]{40}", commit), "commit must be the full lowercase 40-character source SHA")


def names(version):
    return {target: f"dots_{version[1:]}_{target.replace('/', '_')}.tar.gz" for target in TARGETS}


def run(args, **kwargs):
    return subprocess.check_output(list(map(str, args)), **kwargs)


def environment():
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull, GIT_OPTIONAL_LOCKS="0", GIT_TERMINAL_PROMPT="0")
    env.pop("GOROOT", None)  # A caller's unrelated compiler must not replace --go.
    env.update(GOENV="off", GOTOOLCHAIN="local", GOWORK="off", GOFLAGS="", CGO_ENABLED="0",
               GOEXPERIMENT="", GOFIPS140="off", GOAMD64="v1", GOARM64="v8.0")
    return env


def compiler(go):
    env = environment()
    # Hostile target settings must not change the compiler/version queries.
    env.pop("GOOS", None)
    env.pop("GOARCH", None)
    got = run([go, "env", "GOVERSION"], env=env).decode().strip()
    require(got == "go" + GO_VERSION, f"packaging requires Go {GO_VERSION}; got {got}; select --go explicitly")
    return env


def ignored_destination(path):
    path = Path(path).absolute()
    require(not path.exists() and not path.is_symlink(), f"output already exists: {path}")
    resolved = path.resolve()
    require(resolved.is_relative_to(ROOT), "output must be inside this checkout's ignored build directories")
    relative = resolved.relative_to(ROOT)
    require(relative.parts and relative.parts[0] in ("dist", "build"), "output must be under dist/ or build/")
    subprocess.run(["git", "check-ignore", "--no-index", "-q", str(relative)], cwd=ROOT, env=environment(), check=True)
    return resolved


def snapshot(commit, destination):
    env = environment()
    options = ["git", "-c", "core.hooksPath=" + os.devnull, "-c", "core.autocrlf=false", "-c", "gc.auto=0"]
    # Independent objects, no registered worktree, alternates, submodule recursion
    # or execution of checkout hooks. The original repository is read only.
    subprocess.run([*options, "clone", "--no-local", "--no-hardlinks", "--no-checkout", "--quiet", str(ROOT), str(destination)],
                   env=env, check=True)
    # A clone may omit commits referenced only by remote-tracking or other refs.
    # Fetch the exact already-validated object from the local source, without
    # updating refs or FETCH_HEAD; never substitute the clone's default HEAD.
    subprocess.run([*options, "-C", str(destination), "fetch", "--quiet", "--no-tags", "--no-write-fetch-head",
                    "--no-recurse-submodules", str(ROOT), commit], env=env, check=True)
    subprocess.run([*options, "-C", str(destination), "checkout", "--quiet", "--detach", commit], env=env, check=True)
    require(run(["git", "rev-parse", "HEAD"], cwd=destination, env=env).decode().strip() == commit, "wrong source snapshot")
    require(not run(["git", "status", "--porcelain", "--untracked-files=all"], cwd=destination, env=env), "unclean source snapshot")


def binary_format(data, target):
    os_name, arch = target.split("/")
    if os_name == "darwin":
        require(data[:4] == b"\xcf\xfa\xed\xfe" and len(data) >= 32, "expected a 64-bit Mach-O executable")
        require(struct.unpack_from("<I", data, 4)[0] == {"amd64": 0x1000007, "arm64": 0x100000c}[arch],
                "wrong Mach-O architecture")
    else:
        require(data[:6] == b"\x7fELF\x02\x01" and len(data) >= 64, "expected a little-endian ELF64 executable")
        require(struct.unpack_from("<H", data, 18)[0] == {"amd64": 62, "arm64": 183}[arch], "wrong ELF architecture")
        offset = struct.unpack_from("<Q", data, 32)[0]
        size, count = struct.unpack_from("<HH", data, 54)
        require(size >= 4 and offset + size * count <= len(data), "invalid ELF program headers")
        require(all(struct.unpack_from("<I", data, offset + i * size)[0] not in (2, 3) for i in range(count)),
                "unexpected ELF dynamic section or interpreter")


def build_info(go, binary, target, version, commit, env):
    text = run([go, "version", "-m", binary], env=env).decode()
    require(text.splitlines()[0].endswith("go" + GO_VERSION), "wrong embedded compiler")
    for key, value in [("GOOS", target.split('/')[0]), ("GOARCH", target.split('/')[1]), ("CGO_ENABLED", "0")]:
        require(f"\tbuild\t{key}={value}\n" in text, f"missing build setting {key}={value}")
    require(f"\tbuild\tvcs.revision={commit}\n" in text and "\tbuild\tvcs.modified=false\n" in text,
            "missing or changed source-commit evidence")
    require(version.encode() in binary.read_bytes(), "missing release-version injection")
    require("\tbuild\t-trimpath=true\n" in text, "missing -trimpath")
    return text


def write_archive(path, payload):
    with path.open("xb") as raw:
        with gzip.GzipFile(filename="", fileobj=raw, mode="wb", mtime=0, compresslevel=9) as compressed:
            with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as archive:
                for name in MEMBERS:
                    member = tarfile.TarInfo(name)
                    member.size, member.mode = len(payload[name]), 0o755 if name == "dots" else 0o644
                    member.uid = member.gid = member.mtime = 0
                    member.uname = member.gname = ""
                    archive.addfile(member, io.BytesIO(payload[name]))


def archive_payload(path, target):
    require(path.read_bytes()[:10] == b"\x1f\x8b\x08\x00\x00\x00\x00\x00\x02\xff", "unstable gzip header")
    with tarfile.open(path, "r:gz") as archive:
        members = archive.getmembers()
        require(tuple(m.name for m in members) == MEMBERS, "unexpected archive inventory/order")
        for m in members:
            require(m.isfile() and m.mode == (0o755 if m.name == "dots" else 0o644), "unsafe member type or permissions")
            require((m.uid, m.gid, m.mtime, m.uname, m.gname) == (0, 0, 0, "", ""), "unstable archive metadata")
        payload = {m.name: archive.extractfile(m).read() for m in members}
    require(payload["LICENSE"] and payload["INSTALL.md"], "missing license/installation content")
    binary_format(payload["dots"], target)
    return payload


def checksum(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def verify(directory, version):
    expected = names(version)
    require(set(p.name for p in directory.iterdir()) == set(expected.values()) | {"SHA256SUMS"}, "incomplete or unexpected artifact set")
    manifest = directory / "SHA256SUMS"
    require(manifest.is_file() and not manifest.is_symlink(), "unsafe checksum manifest")
    rows = manifest.read_text().splitlines()
    entries = {}
    for row in rows:
        match = re.fullmatch(r"([0-9a-f]{64})  (dots_[a-zA-Z0-9_.]+\.tar\.gz)", row)
        require(match is not None and match[2] not in entries, "invalid/duplicate checksum entry")
        entries[match[2]] = match[1]
    require(tuple(entries) == tuple(sorted(expected.values())), "checksum manifest must cover exactly four sorted artifacts")
    # Verify every checksum before opening any archive or allowing extraction.
    for name, digest in entries.items():
        p = directory / name
        require(p.is_file() and not p.is_symlink() and checksum(p) == digest, f"checksum mismatch: {name}")
    return {target: archive_payload(directory / name, target) for target, name in expected.items()}


def complete(staging, output):
    output.mkdir(mode=0o700)  # Exclusive reservation; never replace another directory.
    # The complete checksum manifest is the last file: failures remain verifiably incomplete.
    for name in [*sorted(p.name for p in staging.glob("*.tar.gz")), "SHA256SUMS"]:
        with (staging / name).open("rb") as source, (output / name).open("xb") as target:
            shutil.copyfileobj(source, target)
        require(checksum(staging / name) == checksum(output / name), "incomplete artifact copy")


def build(version, commit, output, go):
    metadata(version, commit)
    output = ignored_destination(output)
    env = compiler(go)
    # Resolve the commit before creating output parents or staging files.
    require(run(["git", "rev-parse", "--verify", commit + "^{commit}"], cwd=ROOT, env=env).decode().strip() == commit,
            "source commit is unavailable")
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".dots-package-", dir=output.parent) as tmp:
        tmp = Path(tmp)
        source, artifacts = tmp / "source", tmp / "artifacts"
        artifacts.mkdir()
        snapshot(commit, source)
        install = (ROOT / "docs/INSTALL.md").read_bytes()
        license_text = (source / "LICENSE").read_bytes()
        for target, name in names(version).items():
            os_name, arch = target.split("/")
            binary = tmp / "dots"
            target_env = dict(env, GOOS=os_name, GOARCH=arch)
            ldflags = f"-X github.com/fernandoaleman/dots/internal/cli.version={version} -X github.com/fernandoaleman/dots/internal/cli.commit={commit}"
            subprocess.run([go, "build", "-trimpath", "-buildvcs=true", "-mod=readonly", "-ldflags", ldflags, "-o", str(binary), "."],
                           cwd=source, env=target_env, check=True)
            data = binary.read_bytes()
            binary_format(data, target)
            build_info(go, binary, target, version, commit, env)
            write_archive(artifacts / name, {"dots": data, "LICENSE": license_text, "INSTALL.md": install})
        (artifacts / "SHA256SUMS").write_text(''.join(f"{checksum(p)}  {p.name}\n" for p in sorted(artifacts.glob("*.tar.gz"))))
        verify(artifacts, version)
        complete(artifacts, output)
        verify(output, version)
    print(json.dumps({"version": version, "commit": commit, "go": GO_VERSION, "python": platform.python_version(),
                      "zlib": zlib.ZLIB_RUNTIME_VERSION, "CGO_ENABLED": "0", "GOAMD64": "v1", "GOARM64": "v8.0",
                      "installation_sha256": hashlib.sha256(install).hexdigest(), "packager_sha256": checksum(Path(__file__)),
                      "artifacts": {p.name: checksum(p) for p in sorted(output.iterdir())}}, indent=2))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="action", required=True)
    commands.add_parser("compiler", help="print the single pinned packaging compiler version")
    b = commands.add_parser("build")
    b.add_argument("--version", required=True)
    b.add_argument("--commit", required=True)
    b.add_argument("--output", required=True, type=Path)
    b.add_argument("--go", default="go")
    v = commands.add_parser("verify")
    v.add_argument("--version", required=True)
    v.add_argument("--directory", required=True, type=Path)
    v.add_argument("--extract-target", choices=TARGETS)
    v.add_argument("--extract-to", type=Path)
    args = parser.parse_args()
    try:
        if args.action == "compiler":
            print(GO_VERSION)
        elif args.action == "build":
            build(args.version, args.commit, args.output, args.go)
        else:
            metadata(args.version, "0" * 40)
            require(bool(args.extract_target) == bool(args.extract_to), "supply both extraction options")
            payloads = verify(args.directory, args.version)
            if args.extract_to:
                output = ignored_destination(args.extract_to)
                output.mkdir(parents=True, mode=0o700)
                for name, data in payloads[args.extract_target].items():
                    with (output / name).open("xb") as f:
                        f.write(data)
                    (output / name).chmod(0o755 if name == "dots" else 0o644)
            print("Verified all four archive checksums, inventories, formats and normalized metadata")
    except (OSError, ValueError, subprocess.CalledProcessError, tarfile.TarError) as error:
        parser.exit(1, f"packaging refused: {error}\n")


if __name__ == "__main__":
    main()
