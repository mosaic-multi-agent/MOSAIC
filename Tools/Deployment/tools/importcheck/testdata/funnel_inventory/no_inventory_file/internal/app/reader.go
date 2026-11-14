package app

// No deployed_read.go exists in this package, so the inventory rule is inactive.
func useRead() []byte { return readDeployedArtifact("x") }

func readDeployedArtifact(path string) []byte { return nil }
