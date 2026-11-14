package app

// deployed_read.go is a fixture funnel file with several inventory entries.
//
// FUNNEL-ROUTED READS:
//   - a.go (decodeDeployedBytes): decodes a deployed agent.
//     Continuation prose is ignored by the guard.
//   - b.go
//   - c.go: a bare entry with a colon but no parentheses.
//
// Other sections are outside the inventory.
//   - d.go is a dash line after the section end and must not count as an entry.

func decodeDeployedBytes(b []byte) []byte { return b }

func readDeployedArtifact(path string) []byte { return nil }
