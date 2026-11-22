---
version: "1.0.0"
harness: opencode-v2
---

# Orchestrator Injections — OpenCode V2

---

## Design Rationale

- **No injected content:** No V1 OpenCode content is carried over. V2-specific constraints are added here only once they are observed in use; the OpenCode V2 spike found no V2 quirk that needs an orchestrator injection.
- **Watch list:** The orchestrator re-calling `subagent` with a running child's `sessionID`. Not observed as a problem so far; add a constraint here if it shows up in use.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0.0 | 2026-11-17 | MOSAIC | Initial empty orchestrator injections file for OpenCode V2 |
