---
mosaictest_script: 1
---

# MosaicTest Script: ambiguous-produce

Marker-gated. First invocation (no marker): returns COMPLETED_NEEDS_ACTION and writes the
report, setting the marker. Second invocation (marker present): returns SUCCESS and echoes
the task description.

The difference from the findings-loop scripts is in the workflow: this workflow's row has
`-` in On Findings, so there is no route-back target and even Auto-review mode deviates
rather than auto-routing.

## Selector
marker-artifact: MosaicTestAmbiguousReport.md
marker-content: MOSAICTEST-AMBIGUOUS-WRITTEN

## Outcome: marker-absent

### Status
COMPLETED_NEEDS_ACTION

### Message
~~~
row 1 / REVIEW / findings produced / wrote MosaicTestAmbiguousReport.md / returning COMPLETED_NEEDS_ACTION
~~~

### Write
~~~
MOSAICTEST-AMBIGUOUS-WRITTEN

Ambiguous findings report placeholder. Sets the marker for the second invocation.
~~~

## Outcome: marker-present

### Status
SUCCESS

### Message
~~~
row 1 / REVIEW / marker present / received task_description: {task_description} / returning SUCCESS
~~~

### Write
none
