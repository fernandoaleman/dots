"""Check the authoritative artifact download contract without network access."""

import hashlib
import inspect
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import release_gate as gate
from test_package import binary

ROOT = Path(__file__).resolve().parents[1]
ACTION = "actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093"
OUTPUT = "${{ needs.package.outputs.artifact_id }}"


class ArtifactLayoutTests(unittest.TestCase):
    def test_download_configuration_and_id_guard(self):
        workflow = ROOT / ".github/workflows/release-validation.yml"
        expected = 1
        if not workflow.exists():
            workflow = ROOT / ".github/workflows/release.yml"
            expected = 2
        text = workflow.read_text()
        blocks = re.findall(r"      - name: Validate authoritative artifact ID\n(.*?)      - uses: " +
                            re.escape(ACTION) + r" # v4.3.0\n        with:\n(.*?)(?=      - |\Z)",
                            text, re.S)
        self.assertEqual(text.count(ACTION), expected)
        self.assertEqual(len(blocks), expected)
        for guard, inputs in blocks:
            self.assertIn("ARTIFACT_ID: " + OUTPUT, guard)
            self.assertEqual(inputs.strip().splitlines(), ["artifact-ids: " + OUTPUT,
                             "          path: dist/release", "          merge-multiple: true"])
            shell = guard.split("        run: |\n", 1)[1]
            for artifact_id, ok in (("123", True), ("", False), ("0", False), ("01", False),
                                    ("1,2", False), ("x", False)):
                with self.subTest(artifact_id=artifact_id):
                    result = subprocess.run(["bash", "-c", shell], env=dict(os.environ, ARTIFACT_ID=artifact_id),
                                            capture_output=True, check=False)
                    self.assertEqual(result.returncode == 0, ok)
        self.assertIn("dist/release/SHA256SUMS", text)
        self.assertIn("needs.package.outputs.sums_sha256", text)
        self.assertIn("--directory dist/release --extract-target", text)
        if expected == 2:
            self.assertIn("python3 scripts/release_gate.py publish", text)
            self.assertIn(chr(34) + "dist/release" + chr(34), inspect.getsource(gate.artifacts))

    def test_manifest_flat_nested_and_corrupt_fail_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            old = os.getcwd()
            self.addCleanup(os.chdir, old)
            os.chdir(tmp)
            directory = Path("dist/release")
            directory.mkdir(parents=True)
            for target, name in gate.package.names("v0.1.0").items():
                gate.package.write_archive(directory / name, {"dots": binary(target), "LICENSE": b"license", "INSTALL.md": b"install"})
            manifest = directory / "SHA256SUMS"
            manifest.write_text("".join(f"{gate.package.checksum(p)}  {p.name}\n" for p in sorted(directory.glob("*.tar.gz"))))
            expected = hashlib.sha256(manifest.read_bytes()).hexdigest()
            with patch.dict(os.environ, {"EXPECTED_SUMS": expected}):
                self.assertEqual(len(gate.artifacts("v0.1.0")), 5)
                original = manifest.read_bytes()
                manifest.write_bytes(b"corrupt")
                with self.assertRaises(ValueError):
                    gate.artifacts("v0.1.0")
                manifest.unlink()
                nested = directory / "unexpected-artifact"
                nested.mkdir()
                (nested / "SHA256SUMS").write_bytes(original)
                with self.assertRaises(OSError):
                    gate.artifacts("v0.1.0")


if __name__ == "__main__":
    unittest.main()
