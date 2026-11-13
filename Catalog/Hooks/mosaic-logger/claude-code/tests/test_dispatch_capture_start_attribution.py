"""End-to-end replay of an Agent dispatch the way the registration fires it.

The synchronous capture-only entry runs before the subagent starts; the async
catch-all entry may run before or after SubagentStart. The subagent must be
attributed to the dispatch's own instance id in every such ordering.

Covers:
  - capture, SubagentStart, then catch-all: own id, no fallback folder, empty queue
  - a second dispatch of the same type is not given the first dispatch's id,
    including when the first catch-all fires late
  - the catch-all alone never supplies the id
"""

import json
import os
import re
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import mosaic_logger
from correlation_support import (
    RUN_ID, CorrelationCase, fallback_pattern, pre_dispatch_payload, start_payload,
)

_TYPE = "test-writer-tdd"
_FIRST = f"{_TYPE}#12"
_SECOND = f"{_TYPE}#13"


class _FlowCase(CorrelationCase):

    def capture(self, instance, use_id):
        payload = pre_dispatch_payload(instance, _TYPE, use_id)
        mosaic_logger.dispatch_capture(json.dumps(payload))

    def catch_all(self, instance, use_id):
        payload = pre_dispatch_payload(instance, _TYPE, use_id)
        mosaic_logger.dispatch(json.dumps(payload))

    def start(self, agent_id):
        mosaic_logger.dispatch(json.dumps(start_payload(agent_id, _TYPE)))

    def fallback_folders(self):
        pattern = re.compile(fallback_pattern(_TYPE))
        return [f for f in self.folders() if pattern.match(f)]


class TestSingleDispatchAttribution(_FlowCase):

    def test_subagent_is_attributed_to_the_dispatch_id(self):
        self.capture(_FIRST, "tu-1")
        self.start("agent-aaa")
        self.catch_all(_FIRST, "tu-1")
        self.assertEqual(_FIRST, self.mapped_instance("agent-aaa"))

    def test_no_fallback_folder_and_no_leftover_queue_entry(self):
        self.capture(_FIRST, "tu-1")
        self.start("agent-aaa")
        self.catch_all(_FIRST, "tu-1")
        self.assertEqual([_FIRST], self.folders())
        self.assertEqual([], self.fallback_folders())
        self.assert_no_unknown_agent_folder()
        self.assertEqual([], self.queued_ids())

    def test_catch_all_before_subagent_start_gives_the_same_result(self):
        self.capture(_FIRST, "tu-1")
        self.catch_all(_FIRST, "tu-1")
        self.start("agent-aaa")
        self.assertEqual(_FIRST, self.mapped_instance("agent-aaa"))
        self.assertEqual([_FIRST], self.folders())
        self.assertEqual([], self.queued_ids())

    def test_catch_all_alone_does_not_supply_the_instance_id(self):
        self.catch_all(_FIRST, "tu-1")
        self.start("agent-aaa")
        self.assertRegex(self.mapped_instance("agent-aaa"), fallback_pattern(_TYPE))
        self.assertNotIn(_FIRST, self.folders())

    def test_dispatch_run_is_adopted_so_the_subagent_logs_under_it(self):
        self.capture(_FIRST, "tu-1")
        self.start("agent-aaa")
        self.assertIn("invocation_start", self.names(_FIRST, RUN_ID))


class TestFollowingDispatchOfTheSameType(_FlowCase):

    def test_second_dispatch_is_not_given_the_first_dispatchs_id(self):
        self.capture(_FIRST, "tu-1")
        self.start("agent-aaa")
        self.catch_all(_FIRST, "tu-1")
        self.capture(_SECOND, "tu-2")
        self.start("agent-bbb")
        self.catch_all(_SECOND, "tu-2")
        self.assertEqual(_FIRST, self.mapped_instance("agent-aaa"))
        self.assertEqual(_SECOND, self.mapped_instance("agent-bbb"))
        self.assertEqual([], self.queued_ids())

    def test_late_catch_all_of_the_first_dispatch_does_not_requeue_it(self):
        self.capture(_FIRST, "tu-1")
        self.start("agent-aaa")
        self.capture(_SECOND, "tu-2")
        self.catch_all(_FIRST, "tu-1")
        self.start("agent-bbb")
        self.catch_all(_SECOND, "tu-2")
        self.assertEqual(_FIRST, self.mapped_instance("agent-aaa"))
        self.assertEqual(_SECOND, self.mapped_instance("agent-bbb"))
        self.assertEqual(sorted([_FIRST, _SECOND]), self.folders())
        self.assertEqual([], self.queued_ids())

    def test_two_parallel_dispatches_start_in_arrival_order(self):
        self.capture(_FIRST, "tu-1")
        self.capture(_SECOND, "tu-2")
        self.catch_all(_FIRST, "tu-1")
        self.catch_all(_SECOND, "tu-2")
        self.start("agent-aaa")
        self.start("agent-bbb")
        self.assertEqual(_FIRST, self.mapped_instance("agent-aaa"))
        self.assertEqual(_SECOND, self.mapped_instance("agent-bbb"))
        self.assertEqual([], self.queued_ids())


if __name__ == "__main__":
    unittest.main()
