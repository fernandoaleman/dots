#!/usr/bin/env python3
"""Exercise the built dots CLI through PATH; retain isolated fixtures and logs."""

import base64
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import tempfile
import time


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def snapshot(root):
    """Ignore access times; never traverse symlinks or read special files."""
    result = {}

    def visit(path):
        info = path.lstat()
        value = [info.st_mode, info.st_uid, info.st_gid, info.st_dev,
                 info.st_ino, info.st_nlink, info.st_size,
                 info.st_mtime_ns, info.st_ctime_ns]
        if stat.S_ISLNK(info.st_mode):
            value.append(os.readlink(path))
        elif stat.S_ISREG(info.st_mode):
            value.append(hashlib.sha256(path.read_bytes()).hexdigest())
        result[str(path.relative_to(root))] = value
        if stat.S_ISDIR(info.st_mode):
            for child in sorted(path.iterdir()):
                visit(child)

    visit(root)
    return result


class Fixture:
    def __init__(self, suite, name):
        self.suite = suite
        self.root = suite.root / name
        self.root.mkdir(mode=0o700)
        self.home = self.root / "home"
        self.source = self.root / "repository"
        self.target = self.home
        self.state = self.root / "state"
        self.config = self.root / "config" / "dots" / "config.toml"
        for directory in (self.home, self.source, self.config.parent,
                          self.root / "tmp"):
            directory.mkdir(parents=True, mode=0o700)
        self.env = {"PATH": str(suite.bin) + os.pathsep + os.defpath,
                    "HOME": str(self.home), "XDG_CONFIG_HOME": str(self.root / "config"),
                    "XDG_STATE_HOME": str(self.state), "TMPDIR": str(self.root / "tmp"),
                    "PWD": str(self.root), "XDG_DATA_HOME": str(self.root / "data"),
                    "XDG_CACHE_HOME": str(self.root / "cache"),
                    "XDG_RUNTIME_DIR": str(self.root / "runtime"),
                    "LANG": "C", "LC_ALL": "C"}
        require(Path(shutil.which("dots", path=self.env["PATH"])) == suite.bin / "dots",
                "PATH did not select the isolated executable")
        self.configure()
        self.unrelated = self.file(self.home / "unrelated.txt", b"leave me untouched\n")
        self.unrelated_before = snapshot(self.unrelated)

    def file(self, path, data=b"fixture config\n"):
        path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        path.write_bytes(data)
        path.chmod(0o600)
        return path

    def configure(self, roots=None, path=None):
        roots = roots or [self.source]
        # JSON string quoting is also valid TOML for these generated ASCII paths.
        return self.file(path or self.config, (
            "dotfiles_dirs = " + json.dumps([str(p) for p in roots]) + "\n"
            + "target_dir = " + json.dumps(str(self.target)) + "\n").encode())

    def run(self, *args, code=0, unchanged=False):
        before = snapshot(self.root) if unchanged else None
        command = ["dots", *map(str, args)]
        started = time.monotonic()
        proc = subprocess.run(command, cwd=self.root, env=self.env,
                              capture_output=True, text=True, timeout=45)
        self.suite.commands.append({"fixture": self.root.name, "command": command,
                                    "exit": proc.returncode, "stdout": proc.stdout,
                                    "stderr": proc.stderr,
                                    "seconds": round(time.monotonic() - started, 3)})
        require(proc.returncode == code,
                f"{command}: expected exit {code}, got {proc.returncode}\n{proc.stdout}{proc.stderr}")
        if unchanged:
            require(snapshot(self.root) == before, f"{command} wrote to the fixture")
        require(snapshot(self.unrelated) == self.unrelated_before,
                f"{command} changed unrelated content or identity")
        return proc.stdout + proc.stderr

    def registry(self):
        return json.loads((self.state / "dots" / "ownership.json").read_text())

    def owned(self, original, source):
        data = self.registry()
        require(data["version"] == 2 and not data["unfinished"] and not data.get("imports"),
                "import did not finish cleanly in v2")
        records = [r for r in data["records"]
                   if base64.b64decode(r["destination"]) == os.fsencode(original)
                   and r["phase"] == "owned"]
        require(len(records) == 1, "missing or ambiguous ownership record")
        r = records[0]
        require(base64.b64decode(r["text"]) == os.fsencode(source), "wrong recorded link text")
        require(base64.b64decode(r["profile"]) == os.fsencode(self.config), "wrong recorded profile")
        require(base64.b64decode(r["target"]) == os.fsencode(self.target), "wrong recorded target")
        link = original.lstat()
        require(r["leaf"]["id"] == {"device": link.st_dev, "inode": link.st_ino},
                "record does not identify the live symlink")

    def imported(self, original, source, content, identity):
        require(original.is_symlink() and os.readlink(original) == str(source), "wrong symlink")
        require(original.resolve(strict=True) == source, "link resolves elsewhere")
        info = source.lstat()
        require(stat.S_ISREG(info.st_mode) and (info.st_dev, info.st_ino) == identity,
                "original regular-file identity not preserved")
        require(source.read_bytes() == content and original.read_bytes() == content,
                "import changed content")
        self.owned(original, source)


class Suite:
    def __init__(self):
        # Makefile's BINARY recipe is unquoted: use the standard system temp
        # base rather than a caller TMPDIR that may contain shell metacharacters.
        self.root = Path(tempfile.mkdtemp(prefix="dots-acceptance-", dir="/tmp")).resolve()
        require(all(c.isascii() and (c.isalnum() or c in "/._-") for c in str(self.root)),
                "temporary build path is unsafe for the Makefile BINARY recipe")
        self.bin = self.root / "bin"
        self.bin.mkdir(mode=0o700)
        self.commands = []
        self.passed = []
        self.skipped = []

    def check(self, name, test):
        started = time.monotonic()
        test()
        self.passed.append(name)
        print(f"PASS {name} ({time.monotonic() - started:.2f}s)", flush=True)

    def skip(self, name, reason):
        self.skipped.append({"scenario": name, "reason": reason})
        print(f"SKIP {name}: {reason}", flush=True)

    def lifecycle(self):
        f = Fixture(self, "lifecycle")
        for args in [("--help",), ("add", "--help"), ("recover", "--help"),
                     ("ls",), ("status",), ("up", "--dry-run"),
                     ("down", "--dry-run"), ("recover",),
                     ("recover", "--settle", "--revision", "0", "--dry-run"),
                     ("recover", "--settle", "--revision", "0")]:
            f.run(*args, unchanged=True)
        require(not f.state.exists(), "read-only setup initialized private state")
        imported = []
        for relative, name in [(".zshrc", "zshrc"), (".config/nvim/init.lua", "config/nvim/init.lua")]:
            original = f.file(f.home / relative, b"known config\x00bytes\n")
            content = original.read_bytes()
            info = original.stat()
            source = f.source / name
            f.run("add", "--dry-run", original, unchanged=True)
            f.run("add", original)
            f.imported(original, source, content, (info.st_dev, info.st_ino))
            imported.append((original, source, content))
        require(f.state.exists(), "first-use bootstrap missing")
        listing = f.run("ls", unchanged=True)
        status = f.run("status", unchanged=True)
        for original, source, _ in imported:
            require(f'SELECTED "{original}" -> "{source}"' in listing, "ls omitted imported mapping")
            require(f'OK "{original}" -> "{source}"' in status, "status did not recognize imported link")
        f.run("recover", unchanged=True)
        revision = f.registry()["revision"]
        # Recovery must not require TOML, even when state already exists.
        parked = f.config.with_suffix(".parked")
        f.config.rename(parked)
        f.run("recover", unchanged=True)
        parked.rename(f.config)
        f.run("recover", "--settle", "--revision", revision, "--dry-run", unchanged=True)
        f.run("recover", "--settle", "--revision", revision, unchanged=True)
        # A correct manual link must survive down; it is never adopted.
        manual_source = f.file(f.source / "manual", b"manual data\n")
        manual = f.home / ".manual"
        manual.symlink_to(manual_source)
        manual_before = snapshot(manual)
        source_before = snapshot(f.source)
        f.run("down", "--dry-run", unchanged=True)
        f.run("down")
        f.run("recover", "--settle", "--revision", revision, code=1, unchanged=True)
        for original, source, content in imported:
            require(not os.path.lexists(original), "down left an owned link")
            require(source.read_bytes() == content and source.parent.is_dir(), "down lost source or parents")
        require(snapshot(f.source) == source_before, "down changed repository data")
        require(snapshot(manual) == manual_before, "down removed or adopted a manual link")
        f.run("down")
        f.run("up", "--dry-run", unchanged=True)
        f.run("up")
        f.run("status", unchanged=True)
        for original, source, _ in imported:
            f.owned(original, source)
        require(snapshot(manual) == manual_before, "up adopted or changed a manual link")
        f.run("down")
        require(snapshot(f.source) == source_before and snapshot(manual) == manual_before,
                "second down changed sources/manual link")

    def arguments(self):
        f = Fixture(self, "arguments")
        original = f.file(f.home / ".gitconfig")
        second = f.root / "alternate-repository"
        second.mkdir(mode=0o700)
        alternate = f.configure(roots=[second], path=f.root / "alternate.toml")
        for args in [("--config", alternate, "add", "-n", original),
                     ("add", "--config", alternate, "-n", original)]:
            output = f.run(*args, unchanged=True)
            require(str(second / "gitconfig") in output, "explicit config selected wrong repository")
        for args in [("add",), ("add", original, original), ("add", "--force", original),
                     ("--config", f.config, "add", "--config", alternate, original),
                     ("--config", f.config, "add", "--config", f.config, original),
                     ("add", "--config", f.config, "--config", alternate, original),
                     ("--config", f.config, "--config", alternate, "add", original),
                     ("add", "--config"), ("recover", "--retire", "bad", "--revision", "0"),
                     ("recover", "--settle"), ("recover", "--config", alternate)]:
            f.run(*args, code=2, unchanged=True)
        for path in ["../escape", "missing", f.home]:
            f.run("add", path, code=1, unchanged=True)
        f.run("recover", "--settle", "--revision", "1", code=1, unchanged=True)
        require(not f.state.exists(), "rejected arguments initialized state")

    def conflicts(self):
        f = Fixture(self, "collisions")
        original = f.file(f.home / ".zshrc")
        f.file(f.source / "zshrc", b"existing repository bytes\n")
        for flags in [("--dry-run",), ()]:
            f.run("add", *flags, original, code=1, unchanged=True)
        outside = f.file(f.root / "manual-data", b"unmanaged bytes\n")
        manual = f.home / ".manual"
        manual.symlink_to(outside)
        f.run("add", manual, code=1, unchanged=True)
        require(not f.state.exists(), "rejected collision initialized state")
        g = Fixture(self, "duplicate-history")
        original = g.file(g.home / ".gitconfig")
        g.run("add", original)
        g.run("add", original, code=1, unchanged=True)
        # Another profile selecting the same link must not remove its ownership.
        other = g.configure(path=g.root / "other-profile.toml")
        g.run("--config", other, "down")
        require(original.is_symlink(), "unrelated profile removed an owned link")
        g.run("--config", other, "add", original, code=1, unchanged=True)
        # Restore only fixture topology, leaving recorded history intact and the
        # source slot vacant so a source collision cannot mask the claim check.
        original.rename(g.root / "previous-link")
        (g.source / "gitconfig").rename(original)
        output = g.run("add", original, code=1, unchanged=True)
        require("overlaps recorded ownership claim" in output, "historical ownership check not reached")
        require(original.read_bytes() == b"fixture config\n", "historical refusal lost original")

    def permissions(self):
        if os.geteuid() == 0:
            self.skip("source permissions", "requires an unprivileged process")
            return
        f = Fixture(self, "permissions")
        original = f.file(f.home / ".zshrc")
        f.source.chmod(0o500)
        for flags in [("--dry-run",), ()]:
            f.run("add", *flags, original, code=1, unchanged=True)
        require(not f.state.exists(), "permission refusal initialized state")
        self.passed.append("source permissions")
        print("PASS source permissions", flush=True)

    def roots(self):
        f = Fixture(self, "roots")
        second = f.root / "second-repository"
        second.mkdir(mode=0o700)
        f.configure([f.source, second])
        original = f.file(f.home / ".zshrc")
        f.run("add", original, code=1, unchanged=True)
        f.run("add", "--root", second, "--dry-run", original, unchanged=True)
        info = original.stat()
        f.run("add", "--root", second, original)
        f.imported(original, second / "zshrc", b"fixture config\n", (info.st_dev, info.st_ino))
        nested = f.file(f.home / ".gitconfig")
        f.file(f.source / "gitconfig", b"higher priority provider\n")
        f.run("add", "--root", second, nested, code=1, unchanged=True)

    def cross_filesystem(self):
        # Never mount or change a runner's layout; use an existing writable tmpfs.
        f = Fixture(self, "cross-filesystem")
        candidate = Path("/dev/shm")
        if sys.platform != "linux" or not candidate.is_dir():
            self.skip("cross-filesystem refusal", "no supported existing Linux /dev/shm fixture")
            return
        if candidate.stat().st_dev == f.root.stat().st_dev or not os.access(candidate, os.W_OK | os.X_OK):
            self.skip("cross-filesystem refusal", "second device unavailable or not writable")
            return
        try:
            second = Path(tempfile.mkdtemp(prefix="dots-acceptance-", dir=candidate))
        except OSError as error:
            self.skip("cross-filesystem refusal", str(error))
            return
        print(f"Retained second-filesystem fixture: {second}", flush=True)
        f.configure([second])
        original = f.file(f.home / ".zshrc")
        before = snapshot(second)
        for flags in [("--dry-run",), ()]:
            output = f.run("add", *flags, original, code=1, unchanged=True)
            require("filesystem" in output, "cross-filesystem check not reached")
            require(snapshot(second) == before, "cross-filesystem refusal wrote to source")
        self.passed.append("cross-filesystem refusal")
        print("PASS cross-filesystem refusal", flush=True)


def main():
    suite = Suite()
    started = time.monotonic()
    print(f"Retained acceptance fixtures and transcript: {suite.root}", flush=True)
    try:
        subprocess.run(["make", "build", "BINARY=" + str(suite.bin / "dots")],
                       cwd=Path(__file__).resolve().parents[1], check=True, timeout=180)
        for name, test in [("fresh installation and managed lifecycle", suite.lifecycle),
                           ("configuration selection and zero-write usage errors", suite.arguments),
                           ("collisions, duplicate import and ownership history", suite.conflicts),
                           ("ambiguous roots, explicit choice and precedence", suite.roots)]:
            suite.check(name, test)
        suite.permissions()
        suite.cross_filesystem()
        suite.skip("external interrupted-import creation",
                   "no public deterministic fault hook; prepared/relocated lifecycle remains in TestImportCLI and TestRecoveryCLI")
        print(f"Acceptance complete: {len(suite.passed)} groups passed, {len(suite.skipped)} explicit skips; "
              f"{len(suite.commands)} CLI invocations; {time.monotonic() - started:.2f}s", flush=True)
    finally:
        (suite.root / "results.json").write_text(json.dumps({
            "passed": suite.passed, "skipped": suite.skipped, "commands": suite.commands}, indent=2) + "\n")


if __name__ == "__main__":
    main()
