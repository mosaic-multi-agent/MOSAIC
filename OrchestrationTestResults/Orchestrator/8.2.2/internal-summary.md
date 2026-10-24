# Internal Summary: 8.2.2

<!-- generated:internal-overview -->
## Overview

- **Version:** 8.2.2
- **Reports:** 9
- **Suites:** execution-groups, hitl-gate, infrastructure-triggers, route-back, status-routing, wildcard-expansion
- **Models:** claude-haiku-5-5, claude-sonnet-5-5
- **Harnesses:** claude-code

<!-- /generated:internal-overview -->

<!-- generated:problem-areas -->
## Problem Areas

| Suite | ID | Test | Best Rate | Best Combo | Worst Rate | Worst Combo | Spread |
|-------|----|------|-----------|------------|------------|-------------|--------|
| hitl-gate | 57 | hitl-redispatch-unapproved | 94% (100) | claude-sonnet-5-5/claude-code | 87% (100) | claude-haiku-5-5/claude-code | 7% |
| status-routing | 70 | blocked-e503-hitl-retry | 100% (100) | claude-sonnet-5-5/claude-code | 1% (100) | claude-haiku-5-5/claude-code | 99% |
| status-routing | 72 | creator-fix-rereview | 100% (100) | claude-sonnet-5-5/claude-code | 99% (100) | claude-haiku-5-5/claude-code | 1% |
| status-routing | 73 | findings-route-back | 100% (100) | claude-sonnet-5-5/claude-code | 83% (100) | claude-haiku-5-5/claude-code | 17% |
| status-routing | 74 | needs-clarification-no-advance | 100% (100) | claude-sonnet-5-5/claude-code | 99% (100) | claude-haiku-5-5/claude-code | 1% |
| wildcard-expansion | 76 | wildcard-after-routeback | 100% (100) | claude-sonnet-5-5/claude-code | 99% (100) | claude-haiku-5-5/claude-code | 1% |
| wildcard-expansion | 78 | wildcard-input-expansion | 100% (100) | claude-sonnet-5-5/claude-code | 99% (100) | claude-haiku-5-5/claude-code | 1% |

<!-- /generated:problem-areas -->

<!-- generated:infrastructure-failures -->
## Infrastructure Failures

| Suite | ID | Test | Best Rate | Best Combo | Worst Rate | Worst Combo | Spread |
|-------|----|------|-----------|------------|------------|-------------|--------|
| hitl-gate | 55 | hitl-plan-stage-all-agents | 97% (100/102) | claude-haiku-5-5/claude-code | 97% (100/102) | claude-haiku-5-5/claude-code | 0% |

<!-- /generated:infrastructure-failures -->

<!-- generated:exclusions-detail -->
## Exclusions Detail

| Suite | Test | Reason | Termination | Detail |
|-------|------|--------|-------------|--------|
| hitl-gate | hitl-plan-stage-all-agents | echo_mismatch | early_exit | invocation 1: echo mismatch |
| hitl-gate | hitl-plan-stage-all-agents | echo_mismatch | early_exit | invocation 1: echo mismatch |
| infrastructure-triggers | gated-checkpoint-disabled | echo_mismatch | early_exit | invocation 2: echo mismatch |

<!-- /generated:exclusions-detail -->

<!-- analysis:internal-analysis -->
<!-- /analysis:internal-analysis -->
