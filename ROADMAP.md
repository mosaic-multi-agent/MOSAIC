# Roadmap

_Last updated: 2026-11-22_

Informal, hand-maintained list of what's next for this project. Order within each section does not indicate priority. Items can be moved between sections at any time.

## Now (ASAP)

## Next (Near Future)

- Update External modules at Deploy tool, its functionality is very likely lagging behind current Deploy tool capabilities.
- Update Runner to support parallelization in workflows.
- Deploy tool: rebuild the manifest from the agents, skills and hooks already on disk (for a workspace that never had a manifest or lost it). Files that match an item of the catalog stack are recorded against that catalog, using the version from their stamp. Harness-only agents stay out of the manifest, as they do today.
- Deploy tool: Update can put the wrong content into harness-only agents (agents with no generic source in the catalog stack). It finds them by two signals only (a MOSAIC version stamp in the frontmatter, valid MOSAIC section tags in the body), asks for a refresh scope (protocol only, or all managed regions), and refills those regions from the protocol and section bundle of the current stack. The agent's origin is never checked: an agent deployed from another catalog, with its own protocol or bundle, gets this stack's content silently. Decide how Update should treat a harness-only agent whose origin is unknown (skip it, warn, or require the origin to be known).

## Later (Distant Future)

- Additional basic test set to check subagent basic compliance with `managed` instructon sections (like BLOCKED status on missing artifacts, skills, response format, etc).
- Add more harnesses. (Cursor, AWS Kiro, M365, etc)
- Look into orchestrator/subagents instructions, try to reduce number of tokens somehow.
- Attempt to filter out potential secrets from MOSAIC logs.
- Deploy tool: make tool mapping changes revoke old grants. Today an update keeps every non-built-in name in a deployed agent's tools field as "added by you", because Deploy does not record which custom names it wrote itself. So when a `tool_destinations` mapping or a custom tool answer changes or is removed, the old MCP names stay granted next to the new ones (a renamed server is granted twice, a revoked one stays). Deploy could record the custom names it writes per agent and replace only those, keeping names the user added by hand.
- Deploy tool: per-agent tool mapping stamp. `mosaic_tool_mappings_version` is one hash over all mappings, so changing one tool's mapping lists every agent as updated in the plan (only the stamp line changes in agents that don't use the tool). A stamp over just the tools each agent declares would keep the plan to the agents actually affected.
- Record the harness version consistently across all logs (Runner debug log, MOSAIC log in native and Runner mode) for every harness. Today only the GHCP CLI adapter records it (`ghcp_version` in the Runner debug log, because the CLI self-updates). How each harness exposes its version differs and may not be trivial.

## Someday (Dreams)


- Extra orchestrations test coverage for orchestrator-script (Runner tool helper). Unlikely to ever have budget for that, but at least one baseline comparison to regular orchestrator would be nice.
- MCP tool for cross-harnesses orchestration.


