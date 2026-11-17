# Roadmap

_Last updated: 2026-11-14_

Informal, hand-maintained list of what's next for this project. Order within each section does not indicate priority. Items can be moved between sections at any time.

## Now (ASAP)

## Next (Near Future)

- Update External modules at Deploy tool, its functionality is very likely lagging behind current Deploy tool capabilities.
- Update Runner to support parallelization in workflows.

## Later (Distant Future)

- Additional basic test set to check subagent basic compliance with `managed` instructon sections (like BLOCKED status on missing artifacts, skills, response format, etc).
- Add more harnesses. (Cursor, AWS Kiro, M365, etc)
- Look into orchestrator/subagents instructions, try to reduce number of tokens somehow.
- Attempt to filter out potential secrets from MOSAIC logs.
- Record the harness version consistently across all logs (Runner debug log, MOSAIC log in native and Runner mode) for every harness. Today only the GHCP CLI adapter records it (`ghcp_version` in the Runner debug log, because the CLI self-updates). How each harness exposes its version differs and may not be trivial.

## Someday (Dreams)


- Extra orchestrations test coverage for orchestrator-script (Runner tool helper). Unlikely to ever have budget for that, but at least one baseline comparison to regular orchestrator would be nice.
- MCP tool for cross-harnesses orchestration.


