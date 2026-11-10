---
mosaictest_routing: 1
---

# MosaicTest Routing Fixture: deviation-ambiguous

One rule. In Auto-review mode, a COMPLETED_NEEDS_ACTION from a row whose On Findings is `-`
has no route-back target, so the engine cannot auto-route it and deviates instead. This rule
resolves the deviation by dispatching back to the same row (the workflow's only row, row 1,
outside every block, so the reply carries no iteration). The marker file already exists from
the first invocation, so the second invocation returns SUCCESS.

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
MOSAICTEST-AMBIGUOUS-REROUTE / deviation resolved by orchestrator / no On Findings target forced consultation
~~~

### Overrides
none
