package app

// deployed_read.go is a fixture funnel file.
//
// FUNNEL-ROUTED READS:
//   - reader.go (decodeDeployedBytes): decodes a deployed agent.
//     Continuation prose is ignored by the guard.
//
// Other sections are outside the inventory.

func decodeDeployedBytes(b []byte) []byte { return b }

func readDeployedArtifact(path string) []byte { return nil }
