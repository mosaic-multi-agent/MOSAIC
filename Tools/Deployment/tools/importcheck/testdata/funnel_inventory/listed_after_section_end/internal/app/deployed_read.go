package app

// deployed_read.go is a fixture funnel file.
//
// FUNNEL-ROUTED READS:
//   - a.go (decodeDeployedBytes): decodes a deployed agent.
//
// Other sections are outside the inventory.
//   - reader.go is named only after the section end and must not count as listed.

func decodeDeployedBytes(b []byte) []byte { return b }

func readDeployedArtifact(path string) []byte { return nil }
