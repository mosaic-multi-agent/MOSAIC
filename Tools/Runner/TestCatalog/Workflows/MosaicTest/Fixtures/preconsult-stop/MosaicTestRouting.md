---
mosaictest_routing: 1
---

# MosaicTest Routing Fixture: preconsult-stop

The pre-consultation declares a stop. The run must end before any workflow dispatch, with
exit code 6, and the reason below must reach stderr.

There are no rules: nothing is dispatched, so the orchestrator is never consulted for routing.

## Pre-Consultation
stop
reason:
~~~
PRECONSULT-STOP-MARKER: pre-consultation stop declared by the routing fixture
~~~
