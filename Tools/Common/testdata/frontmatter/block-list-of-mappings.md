---
name: test-agent
triggers:
  - trigger: ITERATION_END
    trigger_param: null
  - trigger: MANUAL
    trigger_param: "run-all"
tools:
  - alpha
  - beta
permissions: [read, write]
---

Agent body.
