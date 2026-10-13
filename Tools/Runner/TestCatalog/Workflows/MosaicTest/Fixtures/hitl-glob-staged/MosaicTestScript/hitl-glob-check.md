---
mosaictest_script: 1
---

# MosaicTest Script: hitl-glob-check

Marker-gated, bound to the single VALIDATION row of hitl-glob-staged. The row declares a
wildcard Output (`Stage-*/HITLGlobStage.md`) next to the concrete paths
`Stage-1/HITLGlobStage.md`, `Stage-2/HITLGlobStage.md` and `MosaicTestMarker.md`. The stub
skips the wildcard and writes the concrete paths.

Dispatch 1 (HITL=false) finds the marker absent: it writes the marker and the per-iteration
files with no approval stamp, and returns SUCCESS.

Dispatch 2 (HITL=true override) finds the marker present. `HitlBehaviour: proceed` stops the
stub from returning BLOCKED/E503 (which the Runner routes without any approval read), and
`HitlApproval: true` stamps `human_approved: true` on every file written. The body differs
from dispatch 1's, so every concrete path changes and the Runner's approval check has files
to read. The status is BLOCKED with E401, which the check still inspects.

## Selector
marker-artifact: MosaicTestMarker.md
marker-content: MOSAICTEST-MARKER-SET

## Outcome: marker-absent

### Status
SUCCESS

### Message
~~~
row 1 / VALIDATION / hitl-glob-staged / dispatch 1 / hitl=false / marker absent, wrote marker and per-iteration files / returning SUCCESS
~~~

### Write
~~~
# MosaicTest HITL Glob Artifact

MOSAICTEST-MARKER-SET

Written by the hitl-glob-check script on its first dispatch (marker absent). The content is
fixture data and means nothing beyond marking the dispatch.
~~~

## Outcome: marker-present

### Status
BLOCKED

### HitlBehaviour
proceed

### HitlApproval
true

### Error
E401
DEPENDENCY_MISSING: fixture-declared blocker for the hitl-glob-staged workflow

### Message
~~~
row 1 / VALIDATION / hitl-glob-staged / dispatch 2 / hitl=true / marker present, wrote approved per-iteration files / returning BLOCKED E401
~~~

### Write
~~~
# MosaicTest HITL Glob Artifact - approved

Written by the hitl-glob-check script on its second dispatch (marker present). The content
differs from the first dispatch's so that every concrete path is modified, and the file
carries `human_approved: true` in its frontmatter. The content is fixture data.
~~~
