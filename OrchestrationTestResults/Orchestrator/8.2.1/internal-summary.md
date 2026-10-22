# Internal Summary: 8.2.1

<!-- generated:internal-overview -->
## Overview

- **Version:** 8.2.1
- **Reports:** 15
- **Suites:** execution-groups, hitl-gate, infrastructure-triggers, route-back, status-routing, wildcard-expansion
- **Models:** claude-fable-5-1, claude-haiku-5-5, claude-opus-5-5, claude-sonnet-5-5
- **Harnesses:** claude-code

<!-- /generated:internal-overview -->

<!-- generated:problem-areas -->
## Problem Areas

| Suite | ID | Test | Best Rate | Best Combo | Worst Rate | Worst Combo | Spread |
|-------|----|------|-----------|------------|------------|-------------|--------|
| infrastructure-triggers | 61 | multiple-triggers-same-boundary | 100% (100) | claude-haiku-5-5/claude-code | 99% (100) | claude-sonnet-5-5/claude-code | 1% |
| route-back | 66 | contracts-routeback-quality-gate | 74% (100) | claude-sonnet-5-5/claude-code | 61% (100) | claude-haiku-5-5/claude-code | 13% |
| status-routing | 68 | blocked-e101-retry | 59% (100) | claude-opus-5-5/claude-code | 48% (100) | claude-haiku-5-5/claude-code | 11% |

<!-- /generated:problem-areas -->

<!-- generated:infrastructure-failures -->
## Infrastructure Failures

| Suite | ID | Test | Best Rate | Best Combo | Worst Rate | Worst Combo | Spread |
|-------|----|------|-----------|------------|------------|-------------|--------|
| hitl-gate | 56 | hitl-plan-stage-override | 100% (100/102) | claude-haiku-5-5/claude-code | 100% (100/102) | claude-haiku-5-5/claude-code | 0% |

<!-- /generated:infrastructure-failures -->

<!-- generated:exclusions-detail -->
## Exclusions Detail

| Suite | Test | Reason | Termination | Detail |
|-------|------|--------|-------------|--------|
| hitl-gate | hitl-plan-stage-override | echo_mismatch | early_exit | invocation 1: echo mismatch |
| hitl-gate | hitl-plan-stage-override | echo_mismatch | early_exit | invocation 1: echo mismatch |
| infrastructure-triggers | interval-precise-boundary | echo_mismatch | early_exit | invocation 1: echo mismatch |
| infrastructure-triggers | restore-class-exclusion | echo_mismatch | early_exit | invocation 2: echo mismatch |
| route-back | planner-routeback-quality-gate | echo_mismatch | early_exit | invocation 1: echo mismatch; assertion failures also present |
| status-routing | needs-clarification-no-advance | echo_mismatch | early_exit | invocation 1: echo mismatch |
| wildcard-expansion | wildcard-input-expansion | echo_mismatch | early_exit | invocation 1: echo mismatch |

<!-- /generated:exclusions-detail -->

<!-- analysis:internal-analysis -->
<!-- /analysis:internal-analysis -->
