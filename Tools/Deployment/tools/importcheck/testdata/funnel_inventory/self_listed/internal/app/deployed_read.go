package app

// deployed_read.go is a fixture funnel file that wrongly lists itself.
//
// FUNNEL-ROUTED READS:
//   - deployed_read.go (decodeDeployedBytes): the funnel itself is implicitly permitted.
//
// Other sections are outside the inventory.

func decodeDeployedBytes(b []byte) []byte { return b }
