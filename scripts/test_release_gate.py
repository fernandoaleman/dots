"""Nonpublishing tests for release gates and draft failure handling."""

import contextlib
import io
import hashlib
import json
import urllib.error
from io import BytesIO
import os
import re
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import release_gate as gate
from test_package import binary


class GateTests(unittest.TestCase):
    def setUp(self):
        self.sha = "a" * 40
        self.ctx = {"workflow_sha": self.sha, "run_id": 91, "attempt": 1}
        self.version = "v0.1.0"
        self.env = {"id": 8, "name": "release", "can_admins_bypass": False,
                    "protection_rules": [{"type": "required_reviewers", "prevent_self_review": False,
                                          "reviewers": [{"type": "User", "reviewer": {"id": gate.MAINTAINER_ID}}]}],
                    "deployment_branch_policy": {"custom_branch_policies": True}}
        self.rules = {"target": "tag", "enforcement": "active", "id": 4,
                      "conditions": {"ref_name": {"include": ["refs/tags/v*"], "exclude": []}},
                      "rules": [{"type": x} for x in ("creation", "update", "deletion")]}

    def test_tag_identity_missing_moved_and_annotated(self):
        with patch.object(gate, "api", return_value=None), self.assertRaises(ValueError):
            gate.tag_commit(self.version)
        ref = {"ref": "refs/tags/v0.1.0", "object": {"type": "tag", "sha": "b" * 40}}
        tag = {"sha": "b" * 40, "object": {"type": "commit", "sha": self.sha}}
        with patch.object(gate, "api", side_effect=[ref, tag]):
            self.assertEqual(gate.tag_commit(self.version), self.sha)
        with patch.object(gate, "api", side_effect=[{"commit": {"sha": self.sha}},
                                                    {"merge_base_commit": {"sha": self.sha}}]), \
             patch.object(gate, "tag_commit", return_value="c" * 40), self.assertRaises(ValueError):
            gate.check_identity(self.version, self.sha, self.ctx)

    def test_release_conflicts_include_drafts(self):
        for draft in (True, False):
            with patch.object(gate, "api", return_value=[{"tag_name": self.version, "draft": draft}]), \
                 self.assertRaises(ValueError):
                gate.check_no_release(self.version)
        with patch.object(gate, "api", return_value=[]):
            gate.check_no_release(self.version)

    def test_environment_and_exact_tag_rules(self):
        policy = {"total_count": 1, "branch_policies": [{"name": "master"}]}
        with patch.object(gate, "api", side_effect=[self.env, policy]):
            self.assertEqual(gate.check_environment(), 8)
        with patch.object(gate, "api", side_effect=[self.env, policy]), self.assertRaises(ValueError):
            gate.check_environment(9)
        with patch.object(gate, "api", side_effect=[None]), self.assertRaises(ValueError):
            gate.check_environment()
        with patch.object(gate, "api", side_effect=[[self.rules], self.rules]):
            gate.check_tag_protection()
        for kind in ("creation", "update", "deletion"):
            altered = dict(self.rules, rules=[r for r in self.rules["rules"] if r["type"] != kind])
            with self.subTest(kind=kind), patch.object(gate, "api", side_effect=[[altered], altered]), \
                 self.assertRaises(ValueError):
                gate.check_tag_protection()
        broad = dict(self.rules, bypass_actors=[{"actor_type": "Integration", "actor_id": 1, "bypass_mode": "always"}])
        with patch.object(gate, "api", side_effect=[[broad], broad]), self.assertRaises(ValueError):
            gate.check_tag_protection()

    def test_ruleset_summary_details_and_refusals(self):
        summary = {"id": 4, "name": "release tags", "enforcement": "active"}
        with patch.object(gate, "api", side_effect=[[summary], self.rules]) as api:
            gate.check_tag_protection()
        self.assertEqual(api.call_args_list[1].args[0], "/rulesets/4")
        invalid = [dict(self.rules, target="branch"), dict(self.rules, enforcement="disabled"),
                   dict(self.rules, enforcement="evaluate"),
                   dict(self.rules, conditions={"ref_name": {"include": ["refs/tags/other*"], "exclude": []}}),
                   dict(self.rules, id=5), {}, None]
        invalid.extend(dict(self.rules, rules=[r for r in self.rules["rules"] if r["type"] != kind])
                       for kind in ("creation", "update", "deletion"))
        for detail in invalid:
            with self.subTest(detail=detail), patch.object(gate, "api", side_effect=[[{"id": 4}], detail]), \
                 self.assertRaises((ValueError, TypeError)):
                gate.check_tag_protection()
        with patch.object(gate, "api", side_effect=[[summary], OSError("unavailable")]), self.assertRaises(OSError):
            gate.check_tag_protection()
        with patch.object(gate, "api", side_effect=[[summary], dict(self.rules, enforcement="disabled")]), \
             self.assertRaises(ValueError):
            gate.check_tag_protection()
        other = dict(self.rules, id=7, target="branch")
        with patch.object(gate, "api", side_effect=[[{"id": 7}, summary], other, self.rules]):
            gate.check_tag_protection()
        inherited = dict(self.rules, source_type="Organization", source="example")
        with patch.object(gate, "api", side_effect=[[{"id": 4, "source_type": "Organization"}], inherited]) as api:
            gate.check_tag_protection()
        self.assertEqual(api.call_args_list[1].args[0], "/rulesets/4")
        for bad_id in (None, True, 0, "4"):
            with self.subTest(bad_id=bad_id), patch.object(gate, "api", return_value=[{"id": bad_id}]), \
                 self.assertRaises(ValueError):
                gate.check_tag_protection()
        for field in ("target", "enforcement"):
            missing = dict(self.rules)
            del missing[field]
            with self.subTest(field=field), patch.object(gate, "api", side_effect=[[{"id": 4}], missing]), \
                 self.assertRaises(ValueError):
                gate.check_tag_protection()

    def test_ruleset_pagination_reaches_later_applicable_ruleset(self):
        class Response(BytesIO):
            def __init__(self, payload, link=None):
                super().__init__(json.dumps(payload).encode())
                self.headers = {"Link": link} if link else {}
            def __enter__(self):
                return self
            def __exit__(self, *_):
                self.close()
        next_page = "<" + gate.API + "/rulesets?page=2>; rel=\"next\""
        responses = [Response([{"id": 7}], next_page), Response([{"id": 4}]),
                     Response(dict(self.rules, id=7, target="branch")), Response(self.rules)]
        with patch.dict(os.environ, {"GH_TOKEN": "fixture"}), \
             patch.object(gate.urllib.request, "urlopen", side_effect=responses):
            gate.check_tag_protection()

    def test_required_ci_jobs_must_all_succeed(self):
        names = ["check (ubuntu-latest)", "check (macos-latest)", "Acceptance (ubuntu-latest)",
                 "Acceptance (macos-latest)", "Packaging (darwin/amd64)", "Packaging (darwin/arm64)",
                 "Packaging (linux/amd64)", "Packaging (linux/arm64)"]
        runs = {"workflow_runs": [{"id": 9, "run_attempt": 1, "head_sha": self.sha,
                                    "head_branch": "master", "conclusion": "success"}]}
        jobs = {"total_count": 8, "jobs": [{"name": n, "conclusion": "success"} for n in names]}
        with patch.object(gate, "api", side_effect=[runs, jobs]):
            gate.check_ci(self.sha)
        jobs["jobs"][0]["conclusion"] = "skipped"
        with patch.object(gate, "api", side_effect=[runs, jobs]), self.assertRaises(ValueError):
            gate.check_ci(self.sha)

    def test_ci_accepts_complete_pinned_and_historical_matrices_only(self):
        packaging = ["Packaging (darwin/amd64)", "Packaging (darwin/arm64)",
                     "Packaging (linux/amd64)", "Packaging (linux/arm64)"]
        old = ["check (ubuntu-latest)", "check (macos-latest)",
               "Acceptance (ubuntu-latest)", "Acceptance (macos-latest)"] + packaging
        new = ["check (ubuntu-24.04)", "check (xcode-27)",
               "Acceptance (ubuntu-24.04)", "Acceptance (xcode-27)"] + packaging
        mixed = old[:2] + new[2:4] + packaging
        runs = {"workflow_runs": [{"id": 9, "run_attempt": 1, "head_sha": self.sha,
                                    "head_branch": "master", "conclusion": "success"}]}
        for names in (old, new, mixed):
            jobs = {"total_count": 8, "jobs": [{"name": n, "conclusion": "success"} for n in names]}
            with self.subTest(names=names), patch.object(gate, "api", side_effect=[runs, jobs]):
                if names == mixed:
                    with self.assertRaises(ValueError):
                        gate.check_ci(self.sha)
                else:
                    gate.check_ci(self.sha)

    def test_current_workflow_ci_requires_pinned_matrix(self):
        packaging = ["Packaging (darwin/amd64)", "Packaging (darwin/arm64)",
                     "Packaging (linux/amd64)", "Packaging (linux/arm64)"]
        legacy = ["check (ubuntu-latest)", "check (macos-latest)",
                  "Acceptance (ubuntu-latest)", "Acceptance (macos-latest)"] + packaging
        pinned = ["check (ubuntu-24.04)", "check (xcode-27)",
                  "Acceptance (ubuntu-24.04)", "Acceptance (xcode-27)"] + packaging
        runs = {"workflow_runs": [{"id": 9, "run_attempt": 1, "head_sha": self.sha,
                                    "head_branch": "master", "conclusion": "success"}]}
        for names in (legacy, pinned):
            jobs = {"total_count": 8, "jobs": [{"name": n, "conclusion": "success"} for n in names]}
            with self.subTest(names=names), patch.object(gate, "api", side_effect=[runs, jobs]):
                if names == legacy:
                    with self.assertRaises(ValueError):
                        gate.check_ci(self.sha, pinned_only=True)
                else:
                    gate.check_ci(self.sha, pinned_only=True)

    def test_workflow_runner_matrix_and_release_control_checks(self):
        root = Path(__file__).resolve().parents[1]
        ci = (root / ".github/workflows/ci.yml").read_text()
        release = (root / ".github/workflows/release.yml").read_text()
        self.assertEqual(ci.count("os: [ubuntu-24.04, xcode-27]"), 2)
        for workflow in (ci, release):
            for entry in ("{os: macos-15-intel, target: darwin/amd64}",
                          "{os: xcode-27, target: darwin/arm64}",
                          "{os: ubuntu-24.04, target: linux/amd64}",
                          "{os: ubuntu-24.04-arm, target: linux/arm64}"):
                self.assertIn(entry, workflow)
            self.assertIn('test "$VERSION_ID" = 24.04', workflow)
            self.assertIn('case "$version" in 27|27.*)', workflow)
        self.assertNotIn("ubuntu-latest", ci)
        self.assertNotIn("macos-latest", ci)
        for job in ("validate", "package", "publish"):
            section = re.split(r"\n  (?=[A-Za-z][A-Za-z0-9_-]*:)", release.split("\n  " + job + ":\n", 1)[1], maxsplit=1)[0]
            self.assertEqual(section.count("- name: Verify Ubuntu 24.04 runner"), 1)
            self.assertIn('test "$(uname -s):$(uname -m)" = Linux:x86_64', section)

    def test_publish_retains_partial_draft_and_never_publishes_on_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / "a"
            p.write_bytes(b"artifact")
            release = {"id": 3, "draft": True, "tag_name": self.version}
            asset = {"id": 5, "name": "a", "state": "uploaded", "size": 8,
                     "digest": "sha256:" + hashlib.sha256(b"artifact").hexdigest()}
            request = (self.version, self.sha, self.ctx, 8)
            for failure in ("moved", "create", "upload", "download"):
                outputs = ([subprocess.CalledProcessError(1, "create")] if failure == "create" else
                           [json.dumps(release).encode(), OSError("download") if failure == "download" else b"artifact"])
                with self.subTest(failure=failure), patch.object(gate, "verify_request", return_value=request), \
                     patch.object(gate, "artifacts", return_value=[p]), \
                     patch.object(gate, "check_identity", side_effect=ValueError("moved") if failure == "moved" else None), \
                     patch.object(gate, "check_environment"), patch.object(gate, "check_tag_protection"), \
                     patch.object(gate, "check_no_release"), patch.object(gate.policy, "observed_approval"), \
                     patch.object(gate, "api", side_effect=[[], [release], [], [asset]]), \
                     patch.object(gate.subprocess, "check_output", side_effect=outputs), \
                     patch.object(gate.subprocess, "run") as run:
                    if failure == "upload":
                        run.side_effect = subprocess.CalledProcessError(1, "upload")
                    with self.assertRaises((ValueError, OSError, subprocess.CalledProcessError)):
                        gate.publish()
                    self.assertFalse(any("PATCH" in call.args[0] for call in run.call_args_list))

    def test_success_requires_exact_downloaded_assets_and_native_gate(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / "a"
            p.write_bytes(b"artifact")
            release = {"id": 3, "draft": True, "tag_name": self.version}
            asset = {"id": 5, "name": "a", "state": "uploaded", "size": 8,
                     "digest": "sha256:" + hashlib.sha256(b"artifact").hexdigest()}
            request = (self.version, self.sha, self.ctx, 8)
            with patch.object(gate, "verify_request", return_value=request), \
                 patch.object(gate, "artifacts", return_value=[p]), \
                 patch.object(gate, "check_identity"), patch.object(gate, "check_environment"), \
                 patch.object(gate, "check_tag_protection"), patch.object(gate, "check_no_release"), \
                 patch.object(gate.policy, "observed_approval") as approval, \
                 patch.object(gate, "api", side_effect=[[], [release], [], [asset], [release], [asset],
                                                         dict(release, draft=False), [asset]]), \
                 patch.object(gate.subprocess, "check_output", side_effect=[json.dumps(release).encode(), b"artifact"]) as output, \
                 patch.object(gate.subprocess, "run") as run:
                gate.publish()
                approval.assert_called_once()
                self.assertIn("tag_name=v0.1.0", output.call_args_list[0].args[0])
                self.assertIn("target_commitish=" + self.sha, output.call_args_list[0].args[0])
                self.assertEqual(len(run.call_args_list), 2)
                self.assertIn("PATCH", run.call_args_list[-1].args[0])
                self.assertTrue(all("/releases/3" in " ".join(c.args[0]) for c in run.call_args_list))
                self.assertFalse(any("--clobber" in c.args[0] for c in run.call_args_list))

    def test_uncertain_post_publication_is_reported_without_retry(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / "a"
            p.write_bytes(b"artifact")
            release = {"id": 3, "draft": True, "tag_name": self.version}
            asset = {"id": 5, "name": "a", "state": "uploaded", "size": 8,
                     "digest": "sha256:" + hashlib.sha256(b"artifact").hexdigest()}
            request = (self.version, self.sha, self.ctx, 8)
            error = io.StringIO()
            with patch.object(gate, "verify_request", return_value=request), \
                 patch.object(gate, "artifacts", return_value=[p]), \
                 patch.object(gate, "check_identity"), patch.object(gate, "check_environment"), \
                 patch.object(gate, "check_tag_protection"), patch.object(gate, "check_no_release"), \
                 patch.object(gate.policy, "observed_approval"), \
                 patch.object(gate, "api", side_effect=[[], [release], [], [asset], [release], [asset], OSError("read failed")]), \
                 patch.object(gate.subprocess, "check_output", side_effect=[json.dumps(release).encode(), b"artifact"]), \
                 patch.object(gate.subprocess, "run") as run, contextlib.redirect_stderr(error), self.assertRaises(OSError):
                gate.publish()
            self.assertIn("may already have succeeded", error.getvalue())
            self.assertEqual(sum("PATCH" in c.args[0] for c in run.call_args_list), 1)
            self.assertFalse(any("DELETE" in c.args[0] for c in run.call_args_list))

    def test_source_and_workflow_revisions_each_require_ci(self):
        workflow_sha = "b" * 40
        with patch.dict(os.environ, {"RELEASE_VERSION": self.version, "RELEASE_COMMIT": self.sha}), \
             patch.object(gate, "context", return_value={"workflow_sha": workflow_sha}), \
             patch.object(gate, "check_identity"), patch.object(gate, "check_no_release"), \
             patch.object(gate, "check_ci") as ci:
            gate.verify_request("validate", False)
        self.assertEqual([c.args[0] for c in ci.call_args_list], [self.sha, workflow_sha])
        self.assertEqual([c.kwargs for c in ci.call_args_list], [{"pinned_only": False}, {"pinned_only": True}])

    def test_moved_tag_or_changed_assets_after_upload_never_publish(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / "a"
            p.write_bytes(b"artifact")
            release = {"id": 3, "draft": True, "tag_name": self.version}
            asset = {"id": 5, "name": "a", "state": "uploaded", "size": 8,
                     "digest": "sha256:" + hashlib.sha256(b"artifact").hexdigest()}
            request = (self.version, self.sha, self.ctx, 8)
            for failure in ("moved", "deleted", "changed-assets", "changed-release"):
                final_assets = [] if failure == "changed-assets" else [asset]
                final_release = [] if failure == "changed-release" else [release]
                identity = [None, None, None, ValueError(failure)] if failure in ("moved", "deleted") else None
                with self.subTest(failure=failure), patch.object(gate, "verify_request", return_value=request), \
                     patch.object(gate, "artifacts", return_value=[p]), \
                     patch.object(gate, "check_identity", side_effect=identity), \
                     patch.object(gate, "check_environment"), patch.object(gate, "check_tag_protection"), \
                     patch.object(gate, "check_no_release"), patch.object(gate.policy, "observed_approval"), \
                     patch.object(gate, "api", side_effect=[[], [release], [], [asset], final_release, final_assets]), \
                     patch.object(gate.subprocess, "check_output", side_effect=[json.dumps(release).encode(), b"artifact"]), \
                     patch.object(gate.subprocess, "run") as run, self.assertRaises(ValueError):
                    gate.publish()
                self.assertEqual(len(run.call_args_list), 1)
                self.assertFalse(any("PATCH" in c.args[0] for c in run.call_args_list))

    def test_api_access_failure_never_attempts_create(self):
        with patch.object(gate, "verify_request", side_effect=OSError("403 or unavailable")), \
             patch.object(gate.subprocess, "run") as run, \
             patch.object(gate.subprocess, "check_output") as output, self.assertRaises(OSError):
            gate.publish()
        run.assert_not_called()
        output.assert_not_called()

    def test_release_validation_uses_run_and_eligibility(self):
        env = {"GITHUB_REPOSITORY": gate.REPOSITORY, "GITHUB_EVENT_NAME": "workflow_dispatch",
               "GITHUB_REF": "refs/heads/master", "WORKFLOW_REF":
               gate.REPOSITORY + "/.github/workflows/release.yml@refs/heads/master",
               "WORKFLOW_SHA": self.sha, "GITHUB_SHA": self.sha, "GITHUB_RUN_ID": "91",
               "GITHUB_RUN_ATTEMPT": "1", "GITHUB_JOB": "validate",
               "RELEASE_VERSION": self.version, "RELEASE_COMMIT": self.sha}
        names = ["check (ubuntu-24.04)", "check (xcode-27)", "Acceptance (ubuntu-24.04)",
                 "Acceptance (xcode-27)", "Packaging (darwin/amd64)", "Packaging (darwin/arm64)",
                 "Packaging (linux/amd64)", "Packaging (linux/arm64)"]
        responses = {
            "/actions/runs/91": {"id": 91, "run_attempt": 1, "event": "workflow_dispatch", "head_sha": self.sha,
                                 "repository": {"full_name": gate.REPOSITORY}, "workflow_id": 7},
            "/actions/workflows/release.yml": {"id": 7, "path": ".github/workflows/release.yml"},
            "/branches/master": {"commit": {"sha": self.sha}},
            "/compare/" + self.sha + "..." + self.sha: {"merge_base_commit": {"sha": self.sha}},
            "/git/ref/tags/v0.1.0": {"ref": "refs/tags/v0.1.0", "object": {"type": "commit", "sha": self.sha}},
            "/releases?per_page=100": [],
            "/actions/workflows/ci.yml/runs?event=push&status=success&head_sha=" + self.sha + "&per_page=100":
                {"workflow_runs": [{"id": 12, "run_attempt": 1, "head_sha": self.sha,
                                    "head_branch": "master", "conclusion": "success"}]},
            "/actions/runs/12/attempts/1/jobs?per_page=100":
                {"total_count": 8, "jobs": [{"name": n, "conclusion": "success"} for n in names]},
        }
        with patch.dict(os.environ, env, clear=True), patch.object(gate, "shell", return_value=self.sha), \
             patch.object(gate, "api", side_effect=lambda path, **kw: responses[path]):
            self.assertEqual(gate.verify_request("validate", False)[:2], (self.version, self.sha))
        ci_key = "/actions/runs/12/attempts/1/jobs?per_page=100"
        legacy_names = ["check (ubuntu-latest)", "check (macos-latest)",
                        "Acceptance (ubuntu-latest)", "Acceptance (macos-latest)"] + names[4:]
        legacy_responses = dict(responses, **{ci_key: {"total_count": 8, "jobs": [
            {"name": name, "conclusion": "success"} for name in legacy_names]}})
        with patch.dict(os.environ, env, clear=True), patch.object(gate, "shell", return_value=self.sha), \
             patch.object(gate, "api", side_effect=lambda path, **kw: legacy_responses[path]), \
             self.assertRaises(ValueError):
            gate.verify_request("validate", False)
        for key, value in [("GITHUB_REPOSITORY", "wrong/repo"), ("GITHUB_RUN_ATTEMPT", "2"),
                           ("GITHUB_JOB", "publish"), ("WORKFLOW_REF", "wrong"), ("RELEASE_COMMIT", "bad")]:
            changed = dict(env, **{key: value})
            with self.subTest(key=key), patch.dict(os.environ, changed, clear=True), \
                 patch.object(gate, "shell", return_value=self.sha), \
                 patch.object(gate, "api", side_effect=lambda path, **kw: responses[path]), \
                 patch.object(gate.subprocess, "run") as mutation, self.assertRaises((ValueError, KeyError)):
                gate.verify_request("validate", False)
            mutation.assert_not_called()

    def test_api_pagination_and_permission_errors_fail_closed(self):
        class Response(BytesIO):
            def __init__(self, payload, link=None):
                super().__init__(payload)
                self.headers = {"Link": link} if link else {}
            def __enter__(self):
                return self
            def __exit__(self, *_):
                self.close()
        with patch.dict(os.environ, {"GH_TOKEN": "fixture"}), \
             patch.object(gate.urllib.request, "urlopen", return_value=Response(b"[]", '<https://evil.invalid/repos/fernandoaleman/dots/releases?page=2>; rel="next"')), \
             self.assertRaises(ValueError):
            gate.api("/releases")
        first = Response(b'[1]', '<https://api.github.com/repos/fernandoaleman/dots/releases?page=2>; rel="next"')
        second = Response(b'[2]')
        with patch.dict(os.environ, {"GH_TOKEN": "fixture"}), \
             patch.object(gate.urllib.request, "urlopen", side_effect=[first, second]):
            self.assertEqual(gate.api("/releases"), [1, 2])
        for status in (403, 429, 500):
            error = urllib.error.HTTPError("fixture", status, "failure", {}, None)
            with self.subTest(status=status), patch.dict(os.environ, {"GH_TOKEN": "fixture"}), \
                 patch.object(gate.urllib.request, "urlopen", side_effect=error), \
                 self.assertRaises(urllib.error.HTTPError):
                gate.api("/releases", missing=True)

    def test_real_artifact_set_rejects_missing_and_corruption(self):
        with tempfile.TemporaryDirectory() as tmp:
            old = os.getcwd()
            self.addCleanup(os.chdir, old)
            os.chdir(tmp)
            directory = Path("dist/release")
            directory.mkdir(parents=True)
            for target, name in gate.package.names(self.version).items():
                gate.package.write_archive(directory / name, {"dots": binary(target), "LICENSE": b"license", "INSTALL.md": b"install"})
            manifest = directory / "SHA256SUMS"
            manifest.write_text("".join(f"{gate.package.checksum(p)}  {p.name}\n" for p in sorted(directory.glob("*.tar.gz"))))
            expected = hashlib.sha256(manifest.read_bytes()).hexdigest()
            with patch.dict(os.environ, {"EXPECTED_SUMS": expected}):
                self.assertEqual(len(gate.artifacts(self.version)), 5)
                archive = next(directory.glob("*.tar.gz"))
                saved = archive.read_bytes()
                archive.write_bytes(saved[:-1])
                with self.assertRaises(ValueError):
                    gate.artifacts(self.version)
                archive.unlink()
                with self.assertRaises(ValueError):
                    gate.artifacts(self.version)

    def test_corrupt_manifest_refused_before_verifier(self):
        with tempfile.TemporaryDirectory() as tmp:
            old = os.getcwd()
            self.addCleanup(os.chdir, old)
            os.chdir(tmp)
            Path("dist/release").mkdir(parents=True)
            Path("dist/release/SHA256SUMS").write_text("changed")
            with patch.dict(os.environ, {"EXPECTED_SUMS": "0" * 64}), \
                 patch.object(gate.package, "verify") as verify, self.assertRaises(ValueError):
                gate.artifacts(self.version)
            verify.assert_not_called()


if __name__ == "__main__":
    unittest.main()
