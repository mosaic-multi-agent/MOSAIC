---
mosaictest_routing: 1
---

# MosaicTest Routing Fixture: hitl-glob-staged

Three rules. Two dispatches of the workflow's single VALIDATION row, then a stop.

Dispatch 1 uses table defaults (HITL=false from the row). The agent finds the marker absent,
writes the marker and the per-iteration files, and returns SUCCESS.

Dispatch 2 overrides HITL to true. The script's `HitlBehaviour: proceed` lets the agent accept
it: it writes the per-iteration files stamped `human_approved: true` and returns BLOCKED with
E401. The HITL check resolves the declared `Stage-*/HITLGlobStage.md` output to the concrete
`Stage-1/` and `Stage-2/` files the agent wrote, reads approval from them, and accepts. No HITL
redispatch occurs.

An unmatched state after the BLOCKED would mean the Runner consulted more than expected.

## Pre-Consultation
none

## Rule: run-start

### Action
dispatch

### Agent
mosaictest-scripted

### Row
1

### TaskDescription
~~~
MOSAICTEST-HITL-GLOB-DISPATCH-ONE / VALIDATION row / hitl=false / returning SUCCESS / marker and per-iteration files are written by this step
~~~

### Overrides
none

## Rule: after mosaictest-scripted SUCCESS #1

### Action
dispatch

### Agent
mosaictest-scripted

### Row
1

### TaskDescription
~~~
MOSAICTEST-HITL-GLOB-DISPATCH-TWO / VALIDATION row / hitl=true override / human_in_the_loop=true / expecting BLOCKED E401 with approved per-iteration files / the approval check reads the concrete files matching the Stage-* output
~~~

### Overrides
hitl: true

## Rule: after mosaictest-scripted BLOCKED

### Action
stop

### Reason
~~~
MOSAICTEST-ROUTING-COMPLETE / two dispatches done as scripted / BLOCKED E401 from HITL dispatch confirmed no false redispatch / ending the run on fixture instruction
~~~
