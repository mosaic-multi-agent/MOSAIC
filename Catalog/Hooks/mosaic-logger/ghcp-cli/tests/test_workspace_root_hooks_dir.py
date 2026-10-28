"""Workspace root resolution when the hook runs from the registration directory.

The registration runs the script with cwd `.github/hooks`. Whether the CLI reports the
session working directory or the registration directory as the payload cwd, logs must land
in `<workspace>/OrchestrationLogs`; the same holds when the payload has no cwd and the
process cwd is the registration directory.
"""

import contextlib
import io
import json
import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger
import mosaic_logger_core as core


class TestResolveWorkspaceRoot(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.ws = pathlib.Path(self._tmp.name)

    def test_hooks_dir_resolves_to_the_workspace(self):
        hooks = self.ws / ".github" / "hooks"
        self.assertEqual(self.ws, core.resolve_workspace_root({"cwd": str(hooks)}))

    def test_workspace_dir_resolves_to_itself(self):
        self.assertEqual(self.ws, core.resolve_workspace_root({"cwd": str(self.ws)}))

    def test_hooks_dir_match_ignores_case(self):
        hooks = self.ws / ".GitHub" / "Hooks"
        self.assertEqual(self.ws, core.resolve_workspace_root({"cwd": str(hooks)}))

    def test_hooks_dir_with_trailing_separator_resolves_to_the_workspace(self):
        hooks = str(self.ws / ".github" / "hooks") + os.sep
        self.assertEqual(self.ws, core.resolve_workspace_root({"cwd": hooks}))

    def test_github_dir_alone_is_not_rewritten(self):
        github = self.ws / ".github"
        self.assertEqual(github, core.resolve_workspace_root({"cwd": str(github)}))

    def test_plain_hooks_dir_is_not_rewritten(self):
        hooks = self.ws / "hooks"
        self.assertEqual(hooks, core.resolve_workspace_root({"cwd": str(hooks)}))

    def test_dir_below_hooks_dir_is_not_rewritten(self):
        below = self.ws / ".github" / "hooks" / "sub"
        self.assertEqual(below, core.resolve_workspace_root({"cwd": str(below)}))

    def test_process_cwd_in_hooks_dir_resolves_to_the_workspace(self):
        hooks = self.ws / ".github" / "hooks"
        hooks.mkdir(parents=True)
        previous = os.getcwd()
        self.addCleanup(os.chdir, previous)
        os.chdir(hooks)
        result = core.resolve_workspace_root({})
        self.assertEqual(self.ws.resolve(), result.resolve())

    def test_log_paths_root_is_orchestration_logs_of_the_workspace(self):
        hooks = self.ws / ".github" / "hooks"
        root = core.resolve_workspace_root({"cwd": str(hooks)})
        self.assertEqual(self.ws / "OrchestrationLogs", core.build_paths(root).root)


class TestLogsLandInWorkspaceRoot(unittest.TestCase):

    _SESSION = "ghcp-hooks-cwd-session"

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.ws = pathlib.Path(self._tmp.name)
        self.hooks = self.ws / ".github" / "hooks"
        self.hooks.mkdir(parents=True)

    def _fire(self, event, cwd, **payload):
        body = {"sessionId": self._SESSION, **payload}
        if cwd is not None:
            body["cwd"] = str(cwd)
        with contextlib.redirect_stdout(io.StringIO()):
            mosaic_logger.dispatch(event, json.dumps(body))

    def _workspace_orchestrator_events(self):
        path = core.build_paths(self.ws).orchestrator_events("unknown-run")
        if not path.exists():
            return []
        return [json.loads(ln) for ln in path.read_text("utf-8").splitlines() if ln.strip()]

    def _orchestration_logs_dirs_under_github(self):
        return list((self.ws / ".github").rglob("OrchestrationLogs"))

    def test_payload_cwd_in_the_hooks_dir_logs_to_the_workspace(self):
        self._fire("sessionStart", self.hooks)
        self.assertTrue(self._workspace_orchestrator_events())
        self.assertEqual([], self._orchestration_logs_dirs_under_github())

    def test_payload_cwd_at_the_workspace_logs_to_the_workspace(self):
        self._fire("sessionStart", self.ws)
        self.assertTrue(self._workspace_orchestrator_events())
        self.assertEqual([], self._orchestration_logs_dirs_under_github())

    def test_both_cwd_forms_in_one_session_share_one_logs_root(self):
        self._fire("sessionStart", self.ws)
        self._fire("userPromptSubmitted", self.hooks, prompt="hello")
        self._fire("sessionEnd", self.hooks, reason="complete")
        names = {e["event"] for e in self._workspace_orchestrator_events()}
        self.assertTrue({"run_start", "session_start", "run_end"} <= names, names)
        self.assertEqual([], self._orchestration_logs_dirs_under_github())

    def test_subagent_events_from_the_hooks_dir_land_in_the_workspace_logs(self):
        run_id = "20261028T100000Z-b7c4"
        prompt = json.dumps({"agent_instance_id": "Research#3", "run_id": run_id,
                             "task_description": "Do it."})
        self._fire("sessionStart", self.hooks)
        self._fire("preToolUse", self.hooks, toolName="task", toolUseId="toolu_X1",
                   toolArgs={"prompt": prompt})
        self._fire("subagentStart", self.hooks, agentId="agent-1", agentType="Research")
        path = core.build_paths(self.ws).invocation_events(run_id, "Research#3")
        self.assertTrue(path.exists(), "invocation stream must be under the workspace logs")
        self.assertEqual([], self._orchestration_logs_dirs_under_github())

    def test_payload_without_cwd_uses_the_process_cwd_workspace_root(self):
        previous = os.getcwd()
        self.addCleanup(os.chdir, previous)
        os.chdir(self.hooks)
        self._fire("sessionStart", None)
        os.chdir(previous)
        self.assertTrue(self._workspace_orchestrator_events())
        self.assertEqual([], self._orchestration_logs_dirs_under_github())


if __name__ == "__main__":
    unittest.main()
