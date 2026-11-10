---
mosaictest_script: 1
---

# MosaicTest Script: preconsult-stop-unreached

Never read in a passing run: the pre-consultation stops the run before any dispatch. It exists
so the row has a bound script, and so a run that wrongly dispatches still ends legibly.

## Selector
none

## Outcome: always

### Status
SUCCESS

### Message
~~~
row 1 / RESEARCH / preconsult-stop / dispatched although the pre-consultation stopped the run / returning SUCCESS
~~~

### Write
none
