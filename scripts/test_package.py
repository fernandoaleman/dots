"""Isolated tests for nonpublishing packaging and binary installation."""

import contextlib
import gzip
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import struct
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import acceptance
import package as packaging


def binary(target):
    data = bytearray(64)
    if target.startswith("darwin"):
        data[:4] = b"\xcf\xfa\xed\xfe"
        struct.pack_into("<I", data, 4, 0x1000007 if target.endswith("amd64") else 0x100000c)
    else:
        data[:6] = b"\x7fELF\x02\x01"
        struct.pack_into("<H", data, 18, 62 if target.endswith("amd64") else 183)
        struct.pack_into("<H", data, 54, 56)
    return bytes(data)


class PackageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.version, self.commit = "v0.1.0", "0123456789abcdef0123456789abcdef76543210"

    def artifacts(self, root):
        root.mkdir()
        for target, name in packaging.names(self.version).items():
            packaging.write_archive(root / name, {"dots": binary(target), "LICENSE": b"license\n", "INSTALL.md": b"install\n"})
        self.manifest(root)
        return root

    def manifest(self, root):
        (root / "SHA256SUMS").write_text(''.join(f"{packaging.checksum(p)}  {p.name}\n" for p in sorted(root.glob("*.tar.gz"))))

    def test_metadata_and_targets(self):
        packaging.metadata(self.version, self.commit)
        for v in ["", "dev", "0.1.0", "v01.1.0", "v0.1", "v0.1.0/../x", "v0.1.0\n"]:
            with self.subTest(version=v), self.assertRaises(ValueError):
                packaging.metadata(v, self.commit)
        for c in ["", "a" * 39, "a" * 41, "A" * 40, "z" * 40, self.commit + "\n"]:
            with self.subTest(commit=c), self.assertRaises(ValueError):
                packaging.metadata(self.version, c)
        self.assertEqual(set(packaging.names(self.version).values()), {
            "dots_0.1.0_darwin_amd64.tar.gz", "dots_0.1.0_darwin_arm64.tar.gz",
            "dots_0.1.0_linux_amd64.tar.gz", "dots_0.1.0_linux_arm64.tar.gz"})

    def test_archive_repeatability_and_metadata(self):
        a, b = self.artifacts(self.root / "a"), self.artifacts(self.root / "b")
        for p in a.iterdir():
            self.assertEqual(p.read_bytes(), (b / p.name).read_bytes())
        self.assertEqual(set(packaging.verify(a, self.version)), set(packaging.TARGETS))
        for p in a.glob("*.tar.gz"):
            self.assertEqual(p.read_bytes()[:10], b"\x1f\x8b\x08\x00\x00\x00\x00\x00\x02\xff")
            with tarfile.open(p) as archive:
                self.assertEqual(tuple(m.name for m in archive), packaging.MEMBERS)
            self.assertNotIn(str(self.root).encode(), gzip.decompress(p.read_bytes()))

    def test_missing_corrupt_duplicate_and_unsafe_outputs(self):
        for kind in ["missing", "corrupt", "duplicate", "extra", "symlink", "truncated manifest"]:
            with self.subTest(kind=kind):
                root = self.artifacts(self.root / kind)
                p = root / next(iter(packaging.names(self.version).values()))
                if kind == "missing":
                    p.unlink()
                elif kind == "corrupt":
                    p.write_bytes(p.read_bytes()[:-8])
                elif kind == "duplicate":
                    with (root / "SHA256SUMS").open("a") as f:
                        f.write((root / "SHA256SUMS").read_text().splitlines()[0] + "\n")
                elif kind == "extra":
                    (root / "incomplete").touch()
                elif kind == "symlink":
                    copy = self.root / "saved-archive"
                    p.rename(copy)
                    p.symlink_to(copy)
                else:
                    (root / "SHA256SUMS").write_text("")
                with patch.object(packaging, "archive_payload", side_effect=AssertionError("opened before verification")):
                    with self.assertRaises(ValueError):
                        packaging.verify(root, self.version)

    def test_invalid_members_and_architecture(self):
        cases = [("traversal", ("INSTALL.md", "LICENSE", "../dots"), "inventory/order"),
                 ("absolute", ("INSTALL.md", "LICENSE", "/dots"), "inventory/order"),
                 ("duplicate", (*packaging.MEMBERS, "dots"), "inventory/order"),
                 ("symlink", packaging.MEMBERS, "type or permissions"),
                 ("mode", packaging.MEMBERS, "type or permissions"),
                 ("time", packaging.MEMBERS, "unstable archive metadata")]
        for kind, members, reason in cases:
            path = self.root / (kind + ".tar.gz")
            with path.open("wb") as raw, gzip.GzipFile(filename="", fileobj=raw, mode="wb", mtime=0, compresslevel=9) as compressed:
                with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as archive:
                    for name in members:
                        member = tarfile.TarInfo(name)
                        member.mode = 0o755 if name == "dots" else 0o644
                        if name == "dots" and kind == "symlink":
                            member.type, member.linkname = tarfile.SYMTYPE, "elsewhere"
                        if name == "dots" and kind == "mode":
                            member.mode = 0o644
                        if kind == "time":
                            member.mtime = 1
                        archive.addfile(member, io.BytesIO())
            with self.subTest(kind=kind), self.assertRaisesRegex(ValueError, reason):
                packaging.archive_payload(path, "darwin/arm64")
        for target in packaging.TARGETS:
            packaging.binary_format(binary(target), target)
            with self.assertRaises(ValueError):
                packaging.binary_format(binary(target), target.replace("amd64", "arm64") if "amd64" in target else target.replace("arm64", "amd64"))
        data = bytearray(binary("linux/amd64") + bytes(56))
        struct.pack_into("<Q", data, 32, 64)
        struct.pack_into("<H", data, 56, 1)
        struct.pack_into("<I", data, 64, 3)
        with self.assertRaises(ValueError):
            packaging.binary_format(data, "linux/amd64")

    def test_compiler_and_hostile_environment(self):
        with patch.dict(os.environ, {"GOROOT": "wrong", "GOFLAGS": "-race", "GOWORK": "wrong", "GOEXPERIMENT": "wrong", "GOOS": "wrong"}), patch.object(packaging, "run", return_value=b"go1.26.10\n"):
            env = packaging.compiler("selected-go")
            self.assertNotIn("GOROOT", env)
            self.assertNotIn("GOOS", env)
            self.assertEqual((env["CGO_ENABLED"], env["GOFLAGS"], env["GOWORK"], env["GOEXPERIMENT"], env["GOTOOLCHAIN"]), ("0", "", "off", "", "local"))
        with patch.object(packaging, "run", return_value=b"go1.27.3\n"), self.assertRaises(ValueError):
            packaging.compiler("wrong-go")

    def test_snapshot_uses_commit_not_dirty_source(self):
        destination = self.root / "snapshot"
        with patch.object(packaging.subprocess, "run") as called, patch.object(packaging, "run", side_effect=[self.commit.encode(), b""]):
            packaging.snapshot(self.commit, destination)
        clone, fetch, checkout = [c.args[0] for c in called.call_args_list]
        self.assertEqual(fetch[-1], self.commit)
        self.assertIn("--no-write-fetch-head", fetch)
        self.assertIn("--no-local", clone)
        self.assertIn("--no-hardlinks", clone)
        self.assertIn("--no-checkout", clone)
        self.assertNotIn("--shared", clone)
        self.assertEqual(checkout[-1], self.commit)
        self.assertIn("--detach", checkout)
        self.assertEqual(clone[-1], str(destination))
        self.assertIn("core.hooksPath=" + os.devnull, clone)
        with patch.object(packaging.subprocess, "run"), patch.object(packaging, "run", side_effect=[self.commit.encode(), b" M source.go"]), self.assertRaises(ValueError):
            packaging.snapshot(self.commit, destination)

    def test_snapshot_exact_commit_across_ref_types(self):
        source = self.root / "repository"
        source.mkdir()
        env = dict(packaging.environment(), GIT_AUTHOR_NAME="Test", GIT_AUTHOR_EMAIL="test@example.invalid",
                   GIT_COMMITTER_NAME="Test", GIT_COMMITTER_EMAIL="test@example.invalid")

        def git(*args):
            return subprocess.check_output(["git", *args], cwd=source, env=env, stderr=subprocess.PIPE).decode().strip()

        git("init", "-q")
        (source / "file").write_text("base")
        git("add", "file")
        git("commit", "-qm", "base")
        base = git("rev-parse", "HEAD")
        commits = {"branch": base}
        for name, ref in [("remote", "refs/remotes/origin/hidden"), ("other", "refs/archive/hidden")]:
            (source / "file").write_text(name)
            git("add", "file")
            git("commit", "-qm", name)
            commits[name] = git("rev-parse", "HEAD")
            git("update-ref", ref, commits[name])
            git("reset", "--hard", "-q", base)
        before_refs = git("show-ref")
        before_status = git("status", "--porcelain")
        before_index = (source / ".git/index").read_bytes()
        for name, commit in commits.items():
            with self.subTest(ref=name), patch.object(packaging, "ROOT", source):
                destination = self.root / ("snapshot-" + name)
                packaging.snapshot(commit, destination)
                actual = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=destination, env=env).decode().strip()
                self.assertEqual(actual, commit)
                self.assertEqual((destination / "file").read_text(), "base" if name == "branch" else name)
        with patch.object(packaging, "ROOT", source), self.assertRaises(subprocess.CalledProcessError):
            packaging.snapshot("0" * 40, self.root / "snapshot-missing")
        self.assertEqual(git("show-ref"), before_refs)
        self.assertEqual(git("status", "--porcelain"), before_status)
        self.assertEqual((source / ".git/index").read_bytes(), before_index)
        self.assertEqual((source / "file").read_text(), "base")

    def test_snapshot_rejects_wrong_identity_and_fetch_failure(self):
        destination = self.root / "snapshot"
        with patch.object(packaging.subprocess, "run"), patch.object(packaging, "run", return_value=b"different\n"), self.assertRaisesRegex(ValueError, "wrong source snapshot"):
            packaging.snapshot(self.commit, destination)
        with patch.object(packaging.subprocess, "run", side_effect=[None, subprocess.CalledProcessError(1, "fetch")]), self.assertRaises(subprocess.CalledProcessError):
            packaging.snapshot(self.commit, destination)

    def test_existing_output_and_finalization_failure(self):
        staged = self.artifacts(self.root / "staged")
        output = self.root / "existing"
        output.mkdir()
        marker = output / "original"
        marker.write_bytes(b"untouched")
        with self.assertRaises(FileExistsError):
            packaging.complete(staged, output)
        self.assertEqual(marker.read_bytes(), b"untouched")
        with patch.object(packaging.shutil, "copyfileobj", side_effect=OSError("write failed")), self.assertRaises(OSError):
            packaging.complete(staged, self.root / "failed")
        self.assertFalse((self.root / "failed/SHA256SUMS").exists())
        with self.assertRaises(ValueError):
            packaging.verify(self.root / "failed", self.version)

    def test_build_failure_and_invalid_metadata_leave_no_complete_output(self):
        output = self.root / "output"
        with patch.object(packaging, "compiler", side_effect=AssertionError("compiler ran")), self.assertRaises(ValueError):
            packaging.build("dev", self.commit, output, "go")
        with patch.object(packaging, "ignored_destination", return_value=output), patch.object(packaging, "compiler", return_value={}), patch.object(packaging, "run", return_value=self.commit.encode()), patch.object(packaging, "snapshot", side_effect=OSError("build interrupted")), self.assertRaises(OSError):
            packaging.build(self.version, self.commit, output, "go")
        self.assertFalse(output.exists())

    def test_build_pipeline_records_inputs_and_repeats(self):
        def snapshot(commit, path):
            path.mkdir()
            (path / "LICENSE").write_bytes(b"license")

        def compile_binary(args, **kwargs):
            env = kwargs["env"]
            Path(args[args.index("-o") + 1]).write_bytes(binary(env["GOOS"] + "/" + env["GOARCH"]))
            self.assertIn("-trimpath", args)
            self.assertIn("-buildvcs=true", args)
            self.assertIn(self.commit, args[args.index("-ldflags") + 1])

        for name in ["first", "second"]:
            output = self.root / name
            with patch.object(packaging, "ignored_destination", return_value=output), patch.object(packaging, "compiler", return_value={}), patch.object(packaging, "run", return_value=self.commit.encode()), patch.object(packaging, "snapshot", side_effect=snapshot), patch.object(packaging, "build_info"), patch.object(packaging.subprocess, "run", side_effect=compile_binary), contextlib.redirect_stdout(io.StringIO()) as printed:
                packaging.build(self.version, self.commit, output, "selected-go")
            report = json.loads(printed.getvalue())
            self.assertEqual((report["go"], report["commit"], report["version"]), ("1.26.10", self.commit, self.version))
            self.assertEqual(set(report["artifacts"]), set(packaging.names(self.version).values()) | {"SHA256SUMS"})
        for p in (self.root / "first").iterdir():
            self.assertEqual(p.read_bytes(), (self.root / "second" / p.name).read_bytes())

    def test_build_info_rejects_changed_evidence(self):
        p = self.root / "dots"
        p.write_bytes(binary("darwin/arm64") + self.version.encode())
        text = f"dots: go1.26.10\n\tbuild\tGOOS=darwin\n\tbuild\tGOARCH=arm64\n\tbuild\tCGO_ENABLED=0\n\tbuild\t-trimpath=true\n\tbuild\tvcs.revision={self.commit}\n\tbuild\tvcs.modified=false\n"
        with patch.object(packaging, "run", return_value=text.encode()):
            packaging.build_info("go", p, "darwin/arm64", self.version, self.commit, {})
        for term in ["go1.26.10", "GOOS=darwin", "GOARCH=arm64", "CGO_ENABLED=0", "-trimpath=true", self.commit, "vcs.modified=false"]:
            with patch.object(packaging, "run", return_value=text.replace(term, "invalid").encode()), self.assertRaises(ValueError):
                packaging.build_info("go", p, "darwin/arm64", self.version, self.commit, {})

    def test_output_paths_are_new_and_ignored(self):
        with patch.object(packaging, "ROOT", self.root), patch.object(packaging.subprocess, "run"):
            self.assertEqual(packaging.ignored_destination(self.root / "dist/new"), self.root / "dist/new")
            for output in [self.root, self.root / "source", self.root.parent / "outside"]:
                with self.assertRaises(ValueError):
                    packaging.ignored_destination(output)
            (self.root / "link").symlink_to(self.root.parent)
            with self.assertRaises(ValueError):
                packaging.ignored_destination(self.root / "link/dist/new")

    def test_install_example_stops_before_extraction_on_bad_checksum(self):
        doc = (packaging.ROOT / "docs/INSTALL.md").read_text()
        script = doc.split("```sh\n", 1)[1].split("```", 1)[0]
        tools = self.root / "tools"
        tools.mkdir()
        for name, text in [("shasum", "exit 1"), ("tar", "touch forbidden-extraction"), ("mktemp", "touch forbidden-directory")]:
            p = tools / name
            p.write_text("#!/bin/sh\n" + text + "\n")
            p.chmod(0o755)
        (self.root / "SHA256SUMS").write_text("a" * 64 + "  dots_0.1.0_darwin_arm64.tar.gz\n")
        sentinel = self.root / "dots"
        sentinel.write_bytes(b"original must remain")
        result = subprocess.run(["bash", "-c", script], cwd=self.root,
                                env=dict(os.environ, PATH=str(tools) + os.pathsep + os.defpath), capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / "forbidden-extraction").exists())
        self.assertFalse((self.root / "forbidden-directory").exists())
        self.assertEqual(sentinel.read_bytes(), b"original must remain")


class ExistingBinaryTests(unittest.TestCase):
    def test_install_never_builds_and_preserves_supplied_bytes(self):
        with tempfile.TemporaryDirectory() as tmp:
            source = Path(tmp) / "dots"
            source.write_bytes(b"existing executable bytes")
            source.chmod(0o755)
            suite = acceptance.Suite("v0.1.0")
            self.addCleanup(shutil.rmtree, suite.root)
            with patch.object(acceptance.subprocess, "run", side_effect=AssertionError("must not build")):
                suite.install(source)
            self.assertEqual((suite.bin / "dots").read_bytes(), source.read_bytes())
            for name in [Path(tmp) / "missing", Path(tmp), Path(tmp) / "link", Path(tmp) / "not-executable"]:
                if name.name == "link":
                    name.symlink_to(source)
                if name.name == "not-executable":
                    name.write_bytes(b"not executable")
                with patch.object(acceptance.subprocess, "run", side_effect=AssertionError("must not rebuild")) as invoked:
                    with self.assertRaises((OSError, AssertionError)):
                        suite.install(name)
                    invoked.assert_not_called()

    def test_incompatible_and_wrong_version_fail_without_fallback(self):
        with tempfile.TemporaryDirectory() as tmp:
            source = Path(tmp) / "invalid-executable"
            source.write_bytes(b"not an executable format")
            source.chmod(0o755)
            suite = acceptance.Suite("v0.1.0")
            self.addCleanup(shutil.rmtree, suite.root)
            suite.install(source)
            fixture = acceptance.Fixture(suite, "incompatible")
            before = acceptance.snapshot(fixture.root)
            with self.assertRaises(OSError):
                fixture.run("--help")
            self.assertEqual(before, acceptance.snapshot(fixture.root))
            with patch.object(acceptance.Fixture, "run", return_value="dots wrong-version\n"), self.assertRaises(AssertionError):
                suite.lifecycle()
