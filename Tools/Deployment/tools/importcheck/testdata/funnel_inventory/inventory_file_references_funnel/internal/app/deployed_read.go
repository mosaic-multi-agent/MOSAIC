package app

// deployed_read.go is a fixture funnel file that itself references funnel identifiers in code.
// The inventory file is implicitly permitted and must not be listed.
//
// FUNNEL-ROUTED READS:
//
// Other sections are outside the inventory.

func decodeDeployedBytes(b []byte) []byte { return b }

func readDeployedArtifact(path string) []byte { return decodeDeployedBytes(nil) }

func selfUse() func(string) []byte { return readDeployedArtifact }
