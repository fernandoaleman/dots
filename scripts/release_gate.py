#!/usr/bin/env python3
"""Release authorization and publication. Called only by the release workflow."""

import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request

import package
import release_policy as policy

REPOSITORY = "fernandoaleman/dots"
MAINTAINER_ID = 523634
API = "https://api.github.com/repos/" + REPOSITORY


def require(value, message):
    if not value:
        raise ValueError(message)


def api(path, *, missing=False):
    url = API + path
    pages = []
    seen = set()
    while True:
        require(url.startswith(API + "/") and url not in seen and len(seen) < 100,
                "unsafe or excessive API pagination")
        seen.add(url)
        request = urllib.request.Request(url, headers={
            "Accept": "application/vnd.github+json", "Authorization": "Bearer " + os.environ["GH_TOKEN"],
            "X-GitHub-Api-Version": "2022-11-28", "User-Agent": "dots-release-validation"})
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                pages.append(json.load(response))
                link = response.headers.get("Link", "")
        except urllib.error.HTTPError as error:
            if missing and error.code == 404 and not pages:
                return None
            raise
        next_links = re.findall(r'<([^>]+)>;\s*rel="next"', link)
        require(len(next_links) <= 1, "ambiguous API pagination")
        if not next_links:
            break
        url = next_links[0]
    if len(pages) == 1:
        return pages[0]
    if all(isinstance(page, list) for page in pages):
        return [item for page in pages for item in page]
    require(all(isinstance(page, dict) for page in pages), "inconsistent API pagination")
    keys = [key for key in ("workflow_runs", "jobs", "branch_policies") if key in pages[0]]
    require(len(keys) == 1 and all(isinstance(page.get(keys[0]), list) for page in pages),
            "unsupported API pagination")
    result = dict(pages[0])
    result[keys[0]] = [item for page in pages for item in page[keys[0]]]
    return result


def shell(args, **kwargs):
    return subprocess.check_output(args, **kwargs).decode().strip()


def context(job):
    env = os.environ
    ctx = {"repository": env.get("GITHUB_REPOSITORY"), "event": env.get("GITHUB_EVENT_NAME"),
           "ref": env.get("GITHUB_REF"), "workflow_ref": env.get("WORKFLOW_REF"),
           "run_id": int(env.get("GITHUB_RUN_ID", "0")), "attempt": int(env.get("GITHUB_RUN_ATTEMPT", "0")),
           "workflow_sha": env.get("WORKFLOW_SHA")}
    policy.execution(ctx, ".github/workflows/release.yml")
    require(env.get("GITHUB_JOB") == job, "unexpected release job identity")
    require(ctx["workflow_sha"] == env.get("GITHUB_SHA") == shell(["git", "rev-parse", "HEAD"]),
            "workflow revision and checkout differ")
    run = api("/actions/runs/" + str(ctx["run_id"]))
    require(run["id"] == ctx["run_id"] and run["run_attempt"] == ctx["attempt"] and
            run["event"] == "workflow_dispatch" and run["head_sha"] == ctx["workflow_sha"] and
            run["repository"]["full_name"] == REPOSITORY, "workflow run identity changed")
    workflow = api("/actions/workflows/release.yml")
    require(run["workflow_id"] == workflow["id"] and workflow["path"] == ".github/workflows/release.yml",
            "unexpected workflow identity")
    return ctx


def tag_commit(version):
    ref = api("/git/ref/tags/" + urllib.parse.quote(version, safe=""), missing=True)
    require(ref is not None and ref.get("ref") == "refs/tags/" + version, "release tag is missing or unexpected")
    obj = ref.get("object", {})
    for _ in range(8):
        if obj.get("type") == "commit":
            return obj.get("sha")
        require(obj.get("type") == "tag", "unexpected tag object type")
        tag = api("/git/tags/" + obj["sha"])
        require(tag.get("sha") == obj["sha"], "tag object identity mismatch")
        obj = tag.get("object", {})
    raise ValueError("tag nesting exceeds safety limit")


def check_identity(version, commit, ctx):
    master = api("/branches/master")
    head = master["commit"]["sha"]
    require(head == ctx["workflow_sha"], "master moved after the release workflow started")
    compare = api("/compare/" + commit + "..." + head)
    in_history = compare.get("merge_base_commit", {}).get("sha") == commit
    policy.identity(version, commit, version, tag_commit(version), in_history)


def check_environment(expected_id=None):
    env = api("/environments/release", missing=True)
    env_id = policy.environment(env, MAINTAINER_ID)
    require(expected_id is None or env_id == expected_id, "release environment was replaced")
    branch_policy = env.get("deployment_branch_policy")
    require(isinstance(branch_policy, dict) and branch_policy.get("custom_branch_policies") is True,
            "release environment must restrict deployment to master")
    rules = api("/environments/release/deployment-branch-policies?per_page=100")
    require(rules.get("total_count") == 1 and len(rules.get("branch_policies", [])) == 1 and
            rules["branch_policies"][0].get("name") == "master" and
            rules["branch_policies"][0].get("type", "branch") == "branch", "release environment must allow only master")
    return env_id


def check_tag_protection():
    rulesets = api("/rulesets?targets=tag&includes_parents=true&per_page=100")
    require(isinstance(rulesets, list), "cannot verify tag rulesets")
    for entry in rulesets:
        if entry.get("target") != "tag" or entry.get("enforcement") != "active":
            continue
        ruleset = api("/rulesets/" + str(entry["id"]))
        refs = ruleset.get("conditions", {}).get("ref_name", {})
        types = {rule.get("type") for rule in ruleset.get("rules", [])}
        # GitHub withholds bypass actors from the read-only token. The maintainer
        # must verify in settings that only their account may bypass this rule.
        if (refs.get("include") == ["refs/tags/v*"] and refs.get("exclude") == [] and
                {"creation", "update", "deletion"} <= types):
            bypass = ruleset.get("bypass_actors")
            if bypass is not None:
                require(isinstance(bypass, list) and len(bypass) == 1 and
                        bypass[0].get("actor_type") == "User" and bypass[0].get("actor_id") == MAINTAINER_ID and
                        bypass[0].get("bypass_mode") == "always", "unexpected tag-ruleset bypass actors")
            return
    raise ValueError("active release-tag creation/update/deletion protection is required")


def check_no_release(version):
    releases = api("/releases?per_page=100")
    require(isinstance(releases, list), "cannot verify existing releases")
    require(not any(r.get("tag_name") == version for r in releases), "release or draft already exists")


def check_ci(commit):
    runs = api("/actions/workflows/ci.yml/runs?event=push&status=success&head_sha=" + commit + "&per_page=100")
    expected = {"check (ubuntu-latest)", "check (macos-latest)", "Acceptance (ubuntu-latest)",
                "Acceptance (macos-latest)", "Packaging (darwin/amd64)", "Packaging (darwin/arm64)",
                "Packaging (linux/amd64)", "Packaging (linux/arm64)"}
    for run in runs.get("workflow_runs", []):
        if run.get("head_sha") != commit or run.get("head_branch") != "master" or run.get("conclusion") != "success":
            continue
        jobs = api("/actions/runs/" + str(run["id"]) + "/attempts/" + str(run["run_attempt"]) + "/jobs?per_page=100")
        rows = jobs.get("jobs", [])
        if jobs.get("total_count") == len(rows) and {r.get("name") for r in rows} == expected and len(rows) == len(expected) and all(r.get("conclusion") == "success" for r in rows):
            return
    raise ValueError("no complete successful master CI run for the authorized commit")


def verify_request(job, publishing):
    version, commit = os.environ["RELEASE_VERSION"], os.environ["RELEASE_COMMIT"]
    package.metadata(version, commit)
    ctx = context(job)
    check_identity(version, commit, ctx)
    check_no_release(version)
    check_ci(commit)
    if ctx["workflow_sha"] != commit:
        check_ci(ctx["workflow_sha"])
    if publishing:
        env_id = check_environment(int(os.environ["ENVIRONMENT_ID"]) if job == "publish" else None)
        check_tag_protection()
        return version, commit, ctx, env_id
    return version, commit, ctx, None


def artifacts(version):
    directory = Path("dist/release")
    expected = os.environ["EXPECTED_SUMS"]
    require(len(expected) == 64 and hashlib.sha256((directory / "SHA256SUMS").read_bytes()).hexdigest() == expected,
            "artifact manifest identity changed")
    package.verify(directory, version)
    return sorted(directory.iterdir())


def publish():
    version, commit, ctx, env_id = verify_request("publish", True)
    paths = artifacts(version)
    # The native environment gate authorizes this job. Run-level history
    # is only an additional refusal check, never proof of job-level approval.
    history = api("/actions/runs/" + str(ctx["run_id"]) + "/approvals")
    policy.observed_approval(history, env_id, MAINTAINER_ID, ctx["attempt"])
    check_identity(version, commit, ctx)
    check_environment(env_id)
    check_tag_protection()
    check_no_release(version)
    check_identity(version, commit, ctx)
    # The REST response supplies the new immutable release ID. Identity and the
    # native tag rules were just rechecked. GitHub may create a missing tag;
    # enforced tag rules and the manual bypass audit guard that race.
    raw = subprocess.check_output(["gh", "api", "--method", "POST", "repos/" + REPOSITORY + "/releases",
                                   "-f", "tag_name=" + version, "-f", "target_commitish=" + commit, "-f", "name=" + version,
                                   "-f", "body=Release artifacts and SHA-256 checksums.", "-F", "draft=true"])
    release = json.loads(raw)
    require(release.get("draft") is True and release.get("tag_name") == version and
            type(release.get("id")) is int and release["id"] > 0, "draft release identity changed")
    release_id = release["id"]
    print("Created draft release ID:", release_id, flush=True)
    # Any failure from this point leaves the draft for explicit operator review.
    # Use the returned ID, never tag-name lookup, for uploads and publication.
    check_identity(version, commit, ctx)
    releases = api("/releases?per_page=100")
    matches = [r for r in releases if r.get("tag_name") == version]
    require(len(matches) == 1 and matches[0].get("id") == release_id and matches[0].get("draft") is True,
            "draft release is missing, replaced or ambiguous")
    require(api("/releases/" + str(release_id) + "/assets?per_page=100") == [],
            "new draft unexpectedly contains assets")
    for p in paths:
        upload = ("https://uploads.github.com/repos/" + REPOSITORY + "/releases/" + str(release_id) +
                  "/assets?name=" + urllib.parse.quote(p.name, safe=""))
        subprocess.run(["gh", "api", "--method", "POST", "-H", "Content-Type: application/octet-stream",
                        "--input", str(p), upload], check=True, stdout=subprocess.DEVNULL)
    assets = api("/releases/" + str(release_id) + "/assets?per_page=100")
    require(isinstance(assets, list) and len(assets) == len(paths), "incomplete or unexpected uploaded assets")
    by_name = {a.get("name"): a for a in assets}
    require(len(by_name) == len(paths) and set(by_name) == {p.name for p in paths}, "uploaded asset inventory mismatch")
    for p in paths:
        asset = by_name[p.name]
        require(asset.get("state") == "uploaded" and asset.get("size") == p.stat().st_size and
                asset.get("digest") == "sha256:" + package.checksum(p), "uploaded asset digest mismatch")
        downloaded = subprocess.check_output(["gh", "api", "-H", "Accept: application/octet-stream",
                                              "repos/" + REPOSITORY + "/releases/assets/" + str(asset["id"])])
        require(hashlib.sha256(downloaded).hexdigest() == package.checksum(p), "downloaded asset bytes changed")
    check_identity(version, commit, ctx)
    check_environment(env_id)
    check_tag_protection()
    artifacts(version)
    final = api("/releases?per_page=100")
    matches = [r for r in final if r.get("tag_name") == version]
    require(len(matches) == 1 and matches[0].get("id") == release_id and matches[0].get("draft") is True,
            "draft release changed before publication")
    final_assets = api("/releases/" + str(release_id) + "/assets?per_page=100")
    fields = ("id", "name", "state", "size", "digest")
    fingerprint = lambda rows: sorted(tuple(row.get(field) for field in fields) for row in rows)
    require(fingerprint(final_assets) == fingerprint(assets), "uploaded assets changed before publication")
    check_identity(version, commit, ctx)
    # GH API checks above cannot atomically lock repository settings or refs.
    try:
        subprocess.run(["gh", "api", "--method", "PATCH", "repos/" + REPOSITORY + "/releases/" +
                        str(release_id), "-F", "draft=false"], check=True, stdout=subprocess.DEVNULL)
        public = api("/releases/" + str(release_id))
        require(public.get("id") == release_id and public.get("draft") is False and
                public.get("tag_name") == version, "published release identity changed")
        public_assets = api("/releases/" + str(release_id) + "/assets?per_page=100")
        require(fingerprint(public_assets) == fingerprint(assets), "published assets changed")
        check_identity(version, commit, ctx)
    except (KeyError, ValueError, OSError, subprocess.CalledProcessError):
        print("Publication may already have succeeded; inspect release ID", release_id,
              "before any retry or cleanup.", file=sys.stderr)
        raise


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in ("validate", "publish"):
        raise ValueError("expected validate or publish")
    if sys.argv[1] == "validate":
        publishing = os.environ.get("RELEASE_PUBLISH") == "true"
        _, _, _, env_id = verify_request("validate", publishing)
        if env_id is not None:
            with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
                output.write("environment_id=" + str(env_id) + "\n")
    else:
        publish()


if __name__ == "__main__":
    try:
        main()
    except (KeyError, ValueError, OSError, subprocess.CalledProcessError) as error:
        print("release refused:", error, file=sys.stderr)
        sys.exit(1)
