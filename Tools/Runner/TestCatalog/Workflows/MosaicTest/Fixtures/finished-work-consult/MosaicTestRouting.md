---
mosaictest_routing: 1
---

# MosaicTest Routing Fixture: finished-work-consult

Two rules. Both are used in the Auto run; the Auto-review run uses only the second.

1. `after mosaictest-scripted COMPLETED_NEEDS_ACTION` (Auto only): the checker (row 3) has
   reported findings and the engine cannot route them in Auto. Dispatch the creator (row 1).
2. `after mosaictest-scripted SUCCESS #3`: the creator has succeeded again, and its next step,
   the reviewer (row 2), already has a SUCCESS in the Execution Log. The Runner consults the
   orchestrator instead of dispatching it a second time. At that point the log holds three
   SUCCESS rows of the agent (creator, reviewer, creator again), the last being the creator.
   Dispatch the checker (row 3), which is the step still outstanding.

Any other state matches no rule and the stub stops, so an extra or misplaced consultation is
visible as a failure.

## Pre-Consultation
none

## Rule: after mosaictest-scripted COMPLETED_NEEDS_ACTION

### Action
dispatch

### Agent
mosaictest-scripted

### Row
1

### TaskDescription
~~~
MOSAICTEST-FINISHED-REDISPATCH / orchestrator re-dispatching the creator after the checker's CNA deviation
~~~

### Overrides
none

## Rule: after mosaictest-scripted SUCCESS #3

### Action
dispatch

### Agent
mosaictest-scripted

### Row
3

### TaskDescription
~~~
MOSAICTEST-FINISHED-CONSULT / orchestrator dispatching the checker after the finished-work consultation
~~~

### Overrides
none
