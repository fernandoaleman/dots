"""Pure, fail-closed checks for an explicitly authorized release."""

import re


def require(ok, message):
    if not ok:
        raise ValueError(message)


def identity(version, commit, tag, tag_commit, master_contains_commit):
    require(re.fullmatch(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", version) is not None,
            "invalid stable release version")
    require(re.fullmatch(r"[0-9a-f]{40}", commit) is not None, "expected a full lowercase commit SHA")
    require(tag == version and tag_commit == commit, "tag does not identify the authorized commit")
    require(master_contains_commit is True, "commit is not in the reviewed master history")


def environment(data, owner_id):
    require(type(owner_id) is int and owner_id > 0, "invalid maintainer identity")
    require(isinstance(data, dict) and data.get("name") == "release" and type(data.get("id")) is int and data["id"] > 0,
            "release environment is missing or invalid")
    require(data.get("can_admins_bypass") is False, "release environment must disallow administrator bypass")
    rules = data.get("protection_rules")
    require(isinstance(rules, list), "release environment protection rules cannot be verified")
    reviewers = [r for r in rules if isinstance(r, dict) and r.get("type") == "required_reviewers"]
    require(len(reviewers) == 1, "release environment must require an explicit reviewer")
    rule = reviewers[0]
    require(rule.get("prevent_self_review") is False, "release environment must permit maintainer self-review")
    entries = rule.get("reviewers")
    require(isinstance(entries, list) and len(entries) == 1, "release environment must select only the maintainer")
    reviewer = entries[0]
    require(isinstance(reviewer, dict) and reviewer.get("type") == "User" and
            isinstance(reviewer.get("reviewer"), dict) and reviewer["reviewer"].get("id") == owner_id,
            "release environment reviewer does not match the maintainer")
    return data["id"]


def observed_approval(history, environment_id, owner_id, attempt):
    """Check supplemental history; only the native environment gate authorizes a job."""
    require(attempt == 1, "reruns require a new release workflow and fresh approval")
    require(isinstance(history, list) and len(history) == 1,
            "deployment approval history is missing or ambiguous")
    valid = [r for r in history if isinstance(r, dict) and r.get("state") == "approved" and
             isinstance(r.get("user"), dict) and r["user"].get("id") == owner_id and
             isinstance(r.get("environments"), list) and
             any(isinstance(e, dict) and e.get("id") == environment_id and e.get("name") == "release"
                 for e in r["environments"])]
    require(len(valid) == 1, "explicit maintainer approval for this release environment is required")


def execution(context, workflow_path):
    """Validate trusted runner context, without granting deployment approval."""
    require(isinstance(context, dict), "missing trusted workflow context")
    require(context.get("repository") == "fernandoaleman/dots", "wrong repository")
    require(context.get("event") == "workflow_dispatch", "release requires manual dispatch")
    require(context.get("ref") == "refs/heads/master", "release workflow must run on master")
    require(context.get("workflow_ref") ==
            "fernandoaleman/dots/" + workflow_path + "@refs/heads/master", "wrong workflow identity")
    require(type(context.get("run_id")) is int and context["run_id"] > 0, "invalid run identity")
    require(type(context.get("attempt")) is int and context["attempt"] == 1,
            "reruns require a fresh workflow dispatch")
    require(isinstance(context.get("workflow_sha"), str) and
            re.fullmatch(r"[0-9a-f]{40}", context["workflow_sha"]) is not None,
            "invalid workflow revision")
