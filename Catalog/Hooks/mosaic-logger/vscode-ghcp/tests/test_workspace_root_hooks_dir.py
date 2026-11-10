"""Workspace root resolution when the hook runs from the registration directory.

The hook registration runs the script with cwd `.github/hooks`. Logs must still land in
`<workspace>/OrchestrationLogs`, whether the payload cwd, or (when the payload has none)
the process cwd, points at the workspace itself or at `<workspace>/.github/hooks`.
"""

import json
import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import vscode_flow as flow
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

    def setUp(self):
        self.ws = flow.Workspace()
        self.addCleanup(self.ws.cleanup)
        self.hooks = self.ws.path / ".github" / "hooks"
        self.hooks.mkdir(parents=True)

    def _orchestration_logs_dirs_under_github(self):
        return list((self.ws.path / ".github").rglob("OrchestrationLogs"))

    def test_events_fired_from_the_hooks_dir_land_in_the_workspace_logs(self):
        for kind in ("session_start", "pre_dispatch", "start"):
            self.ws.send(kind, cwd=str(self.hooks))
        self.assertTrue(self.ws.invocation_events(),
                        "invocation events must be under <workspace>/OrchestrationLogs")
        self.assertTrue(self.ws.orchestrator_events())

    def test_nothing_is_written_under_the_github_dir(self):
        for kind in ("session_start", "pre_dispatch", "start", "stop", "post_dispatch"):
            self.ws.send(kind, cwd=str(self.hooks))
        self.assertEqual([], self._orchestration_logs_dirs_under_github())

    def test_events_with_workspace_cwd_land_in_the_same_logs_root(self):
        self.ws.send("session_start", cwd=str(self.hooks))
        self.ws.send("pre_dispatch", cwd=str(self.ws.path))
        self.ws.send("start", cwd=str(self.hooks))
        self.assertTrue(self.ws.invocation_events())
        self.assertEqual([], self._orchestration_logs_dirs_under_github())

    def test_payload_without_cwd_uses_the_process_cwd_workspace_root(self):
        previous = os.getcwd()
        self.addCleanup(os.chdir, previous)
        os.chdir(self.hooks)
        data = flow.payload("session_start", self.ws.path)
        data.pop("cwd")
        mosaic_logger.dispatch(json.dumps(data))
        os.chdir(previous)
        self.assertTrue(self.ws.orchestrator_events())
        self.assertEqual([], self._orchestration_logs_dirs_under_github())


if __name__ == "__main__":
    unittest.main()
