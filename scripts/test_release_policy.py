"""Nonpublishing release-approval policy tests."""

import copy
import unittest

import release_policy as policy


class PolicyTests(unittest.TestCase):
    def setUp(self):
        self.commit = "a" * 40
        self.env = {"id": 123, "name": "release", "can_admins_bypass": False,
                    "protection_rules": [{"type": "required_reviewers", "prevent_self_review": False,
                                          "reviewers": [{"type": "User", "reviewer": {"id": 42}}]}]}

    def test_exact_identity(self):
        policy.identity("v0.1.0", self.commit, "v0.1.0", self.commit, True)
        for args in [("v0.01.0", self.commit, "v0.01.0", self.commit, True),
                     ("v0.1.0", "a" * 39, "v0.1.0", self.commit, True),
                     ("v0.1.0", self.commit, "v0.1.1", self.commit, True),
                     ("v0.1.0", self.commit, "v0.1.0", "b" * 40, True),
                     ("v0.1.0", self.commit, "v0.1.0", self.commit, False)]:
            with self.subTest(args=args), self.assertRaises(ValueError):
                policy.identity(*args)

    def test_environment_requires_maintainer_and_self_approval(self):
        self.assertEqual(policy.environment(self.env, 42), 123)
        mutations = [lambda e: e.update(name="wrong"), lambda e: e.update(id=True), lambda e: e.pop("can_admins_bypass"),
                     lambda e: e.update(can_admins_bypass=True), lambda e: e.update(protection_rules=[]),
                     lambda e: e["protection_rules"][0].update(prevent_self_review=True),
                     lambda e: e["protection_rules"][0]["reviewers"][0]["reviewer"].update(id=99),
                     lambda e: e["protection_rules"][0]["reviewers"].append({"type": "Team"})]
        for mutate in mutations:
            env = copy.deepcopy(self.env)
            mutate(env)
            with self.assertRaises(ValueError):
                policy.environment(env, 42)

    def test_execution_context_is_fail_closed(self):
        context = {"repository": "fernandoaleman/dots", "event": "workflow_dispatch",
                   "ref": "refs/heads/master", "workflow_ref":
                   "fernandoaleman/dots/.github/workflows/release.yml@refs/heads/master",
                   "run_id": 12, "attempt": 1, "workflow_sha": self.commit}
        policy.execution(context, ".github/workflows/release.yml")
        for key, value in [("repository", "other/repo"), ("event", "push"),
                           ("ref", "refs/heads/other"), ("workflow_ref", "wrong"),
                           ("run_id", True), ("attempt", 2), ("workflow_sha", "short")]:
            changed = dict(context, **{key: value})
            with self.subTest(key=key), self.assertRaises(ValueError):
                policy.execution(changed, ".github/workflows/release.yml")

    def test_approval_rejects_unverifiable_history_and_reruns(self):
        history = [{"state": "approved", "user": {"id": 42}, "environments": [{"id": 123, "name": "release"}]}]
        policy.observed_approval(history, 123, 42, 1)
        for data, env_id, owner, attempt in [(history, 123, 42, 2), ([], 123, 42, 1),
                                              (history, 124, 42, 1), (history, 123, 43, 1),
                                              (history * 2, 123, 42, 1),
                                              (history + [{"state": "rejected"}], 123, 42, 1)]:
            with self.assertRaises(ValueError):
                policy.observed_approval(data, env_id, owner, attempt)


if __name__ == "__main__":
    unittest.main()
