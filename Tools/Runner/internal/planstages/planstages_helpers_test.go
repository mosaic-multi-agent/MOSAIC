package planstages_test

// Shared test helpers and constants for the planstages test suite.

import "path/filepath"

const planstagesTestdataDir = "../../testdata/planstages"

// planstagesFixture returns the path to a named fixture file.
func planstagesFixture(name string) string {
	return filepath.Join(planstagesTestdataDir, name)
}
