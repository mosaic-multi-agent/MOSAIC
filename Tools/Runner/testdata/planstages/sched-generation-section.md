# Generation Plan

## Stages

| Stage | Depends On | HITL |
|-------|------------|------|
| 1 | - | FALSE |

## Generation

| Generation | Name | Depends On | HITL |
|------------|------|------------|------|
| A | First | - | FALSE |
| b-2 | Second | A | TRUE |
| Acme | Third | A, b-2 | FALSE |
