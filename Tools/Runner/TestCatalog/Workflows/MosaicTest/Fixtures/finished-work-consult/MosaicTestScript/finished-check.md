---
mosaictest_script: 1
---

# MosaicTest Script: finished-check

Marker-gated. Bound to row 3 (the checker) in finished-work-consult. First invocation writes
the marker and returns COMPLETED_NEEDS_ACTION, which sends the run back to the creator on row 1.
Second invocation finds the marker and returns SUCCESS.

## Selector
marker-artifact: MosaicTestMarker.md
marker-content: MOSAICTEST-MARKER-SET

## Outcome: marker-absent

### Status
COMPLETED_NEEDS_ACTION

### Message
~~~
row 3 / REVIEW / checker / marker absent / wrote MosaicTestMarker.md / returning COMPLETED_NEEDS_ACTION
~~~

### Write
~~~
MOSAICTEST-MARKER-SET
~~~

## Outcome: marker-present

### Status
SUCCESS

### Message
~~~
row 3 / REVIEW / checker / marker present / returning SUCCESS
~~~

### Write
none
