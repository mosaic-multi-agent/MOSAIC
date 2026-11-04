package app_test

// transform_ghcp_pair_backup_test.go specifies the real-run end state of a transform between the
// two GHCP targets for agents: every removed or replaced agent file is recoverable byte for byte
// from the backup path its result entry reports, and the project and custom regions of the
// agent's deployed file are kept.

import (
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
)

// trAssertBackedUp fails the test unless the change reports a backup file holding want.
func trAssertBackedUp(t *testing.T, c app.TransformFileChange, want string) {
	t.Helper()
	if c.BackupPath == "" {
		t.Errorf("%s: BackupPath is empty after a real run, want the backup file", c.Path)
		return
	}
	got, err := os.ReadFile(c.BackupPath)
	if err != nil {
		t.Errorf("%s: backup %s unreadable: %v", c.Path, c.BackupPath, err)
		return
	}
	if string(got) != want {
		t.Errorf("%s: backup holds %q, want the original bytes %q", c.Path, got, want)
	}
}

func TestTransformGhcpPair_RealRunBacksUpTheRemovedLegacyAgentAndTheReplacedOne(t *testing.T) {
	const existing = "an X.agent.md the manifest does not record\n"
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r, legacyRel := trPairLegacyResultRun(t, pair[0], pair[1], nil)
			trWrite(t, r.ws, trPairInPlaceAgent, existing)
			legacyBytes := r.read(legacyRel)

			res := r.mustRun(r.request())

			trAssertBackedUp(t, trChangeAt(t, res.Removals, legacyRel), legacyBytes)
			trAssertBackedUp(t, trChangeAt(t, res.Replacements, trPairInPlaceAgent), existing)
			if r.exists(legacyRel) {
				t.Errorf("%s still exists after the migration", legacyRel)
			}
		})
	}
}

func TestTransformGhcpPair_RealRunBacksUpTheInPlaceAgent(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r, agentRel := trPairResultRun(t, pair[0], pair[1], nil)
			original := r.read(agentRel)

			res := r.mustRun(r.request())

			trAssertBackedUp(t, trChangeAt(t, res.Replacements, agentRel), original)
		})
	}
}

// trWithRegions appends a project region and a custom region to an agent file.
func trWithRegions(content string) string {
	return content + "\n<CodebaseContext type=\"project\">\nPAIR-PROJECT-REGION-TEXT\n</CodebaseContext>\n" +
		"\n<TeamNotes type=\"custom\">\nPAIR-CUSTOM-REGION-TEXT\n</TeamNotes>\n"
}

func TestTransformGhcpPair_RealRunKeepsProjectAndCustomRegionsOfTheInPlaceAndMigratedAgent(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, src, tgt string) (*trRun, string)
	}{
		{"in place", func(t *testing.T, src, tgt string) (*trRun, string) { return trPairResultRun(t, src, tgt, nil) }},
		{"migrated", func(t *testing.T, src, tgt string) (*trRun, string) { return trPairLegacyResultRun(t, src, tgt, nil) }},
	}
	for _, c := range cases {
		for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
			t.Run(c.name+" "+pair[0]+"->"+pair[1], func(t *testing.T) {
				r, rel := c.run(t, pair[0], pair[1])
				trWrite(t, r.ws, rel, trWithRegions(r.read(rel)))

				r.mustRun(r.request())

				got := r.read(r.dest("worker"))
				for _, marker := range []string{"PAIR-PROJECT-REGION-TEXT", "PAIR-CUSTOM-REGION-TEXT"} {
					if n := strings.Count(got, marker); n != 1 {
						t.Errorf("%s appears %d times in %s, want once", marker, n, r.dest("worker"))
					}
				}
			})
		}
	}
}

func TestTransformGhcpPair_RealRunLeavesASingleAgentFileForAMigratedAgent(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r, legacyRel := trPairLegacyResultRun(t, pair[0], pair[1], nil)

			r.mustRun(r.request())

			if r.exists(legacyRel) {
				t.Errorf("%s still exists, want only the X.agent.md file", legacyRel)
			}
			if !r.exists(r.dest("worker")) {
				t.Errorf("%s missing after the migration", r.dest("worker"))
			}
		})
	}
}
