<Workflow type="core" name="staged" version="1.0">
## Block Followed By A Run-Scope Row In The Same Phase

The EXECUTION phase holds a block (Stage) and, after it, a row outside every
block. The block's last row of its last iteration is not the end of the phase:
the run continues at wrap-up, in the same phase. Requires Plan.md in RunFolder.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| EXECUTION.[Stage] | implementation-tdd | FALSE | implementation-review | - | - | - |
| EXECUTION.[Stage] | implementation-review | FALSE | NEXT | implementation-tdd | - | Stage-{Stage}/implementation-review.md |
| EXECUTION | wrap-up | FALSE | COMPLETE | - | - | wrap-up.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |
</Workflow>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="commit-manager-git" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | STAGE_END | - | halt | Git commit manager (fires at iteration end) |
</InfrastructureAgent>
<InfrastructureAgent type="core" name="checkpoint-manager-git" version="1.0.0">
| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| checkpoint | PHASE_END | - | halt | Checkpoint manager that fires at the end of a phase |
</InfrastructureAgent>
</InfrastructureAgents>
