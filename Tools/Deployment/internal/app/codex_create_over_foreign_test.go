package app_test

// codex_create_over_foreign_test.go pins the outcome of a Codex create over another agent's
// file: the planned path of an agent holds a Codex file claimed by another deployed agent, so
// the plan creates the agent there. The run succeeds and writes this agent's create encoding.
// Content the other file holds as decoded values (a user-owned scalar key) is carried as for
// Markdown. Content that lives only in the other file's raw bytes (its header comment, and a
// marker-carried key with its span) is not copied.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

const codexForeignOtherID = "9998"

// codexForeignFile is the file of another agent: its id stamp, a header comment, a value-carried
// user key and a marker-carried user key (a nested table).
const codexForeignFile = "# mosaic_id: " + codexForeignOtherID + "\n" +
	"# mosaic_version: 2.0\n" +
	"\n" +
	"# foreignheadercomment\n" +
	"sandbox_mode = \"read-only\"\n" +
	"foreign_key = \"foreign-value\"\n" +
	"developer_instructions = \"Other agent body.\\n\"\n" +
	"\n" +
	"[mcp_servers.foreignserver]\n" +
	"command = \"foreigncmd\"\n"

// codexOtherAgentFile is the indexed file under which the other agent's id is deployed.
const codexOtherAgentFile = "# mosaic_id: " + codexForeignOtherID + "\n" +
	"# mosaic_version: 2.0\n" +
	"\n" +
	"sandbox_mode = \"read-only\"\n" +
	"developer_instructions = \"Deployed agent.\\n\"\n"

func TestCodexCreateOverAnotherAgentsFile_Succeeds_AndCopiesOnlyDecodedValues(t *testing.T) {
	h := owHarnesses[1]
	for _, flowName := range []string{"Update", "DeployAgents"} {
		t.Run(flowName, func(t *testing.T) {
			// Arrange
			ws := owDeploy(t, h)
			gamWrite(t, ws, h.rel("foreign-other-agent"), codexOtherAgentFile)
			gamWrite(t, ws, h.rel(owRunner), codexForeignFile)
			spy := &owSpy{}

			// Act
			_, err := owFlowNamed(t, flowName).run(t, owService(t, spy), h, ws, owFlowOpts{})

			// Assert
			if err != nil {
				t.Fatalf("%s over a file claimed by another agent: %v", flowName, err)
			}
			if got := owPlanItem(t, spy.lastPlan(t), owRunner).Action; got != domain.ActionCreate {
				t.Fatalf("plan action of %s = %s; want %s (the path is claimed by another agent)", owRunner, got, domain.ActionCreate)
			}
			raw := gamRead(t, ws, h.rel(owRunner))
			owCanonicalRecord(t, h, raw) // the written file decodes through the Codex translator
			written := string(raw)
			if !strings.Contains(written, "foreign_key") {
				t.Errorf("the value-carried user key of the other file was not carried:\n%s", written)
			}
			for _, absent := range []string{"foreignheadercomment", "mcp_servers", "foreignserver"} {
				if strings.Contains(written, absent) {
					t.Errorf("the new file holds %q from the other file's raw bytes:\n%s", absent, written)
				}
			}
		})
	}
}
