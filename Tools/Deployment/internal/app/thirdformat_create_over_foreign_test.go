package app_test

// thirdformat_create_over_foreign_test.go specifies a create over another agent's file: the
// planned path of an agent holds a file claimed by another deployed agent, so the plan creates
// the agent there. The test format and Markdown must behave alike, and nothing that lives only
// in the other file's raw bytes (the test format's format-native member) may leak into the new
// file, while user-owned frontmatter keys of the other file are carried as for Markdown.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

const (
	twinForeignKey   = "foreign_key"
	twinForeignValue = "foreign-value"
	// twinForeignNative is the sentinel of the test format's native member of the foreign file.
	twinForeignNative = "foreignnativesentinel"
)

// twinForeignFileAtRunnerPath makes the planned path of the runner hold a file of another agent
// that is claimed by that agent: it carries the other agent's id (shared with an indexed file
// elsewhere), a user-owned frontmatter key and, for the test format, a native member.
func twinForeignFileAtRunnerPath(t *testing.T, p twinPair) {
	t.Helper()
	for _, side := range twinSides(p) {
		other := twinSetField(twinIDKey, twinOtherID)(twinHarnessOnlyCanonical())
		writeTwinAgent(t, side, twinHarnessOnlyKey, other)
		editTwinAgent(t, side, owRunner, twinChain(
			twinSetField(twinIDKey, twinOtherID), twinSetField(twinForeignKey, twinForeignValue)))
		if side.Format == twinTestFormat {
			addTwinNativeMember(t, side, owRunner)
		}
	}
}

// addTwinNativeMember adds a native member to the test format file of the agent key.
func addTwinNativeMember(t *testing.T, side twinSide, key string) {
	t.Helper()
	rel := twinAgentRel(t, side, key)
	raw := strings.TrimRight(string(gamRead(t, side.Workspace, rel)), "\n")
	end := strings.LastIndex(raw, "}")
	if end < 0 {
		t.Fatalf("test format file of %s is not a JSON object", key)
	}
	body := strings.TrimRight(raw[:end], " \n")
	gamWrite(t, side.Workspace, rel, body+",\n  \"native\": {\"sentinel\": \""+twinForeignNative+"\"}\n}\n")
}

func twinCreateFlows() map[string]twinStep {
	return map[string]twinStep{"update": twinUpdateStep, "deploy agents": twinDeployAgentsStep}
}

func TestTwinParity_CreateOverAnotherAgentsFile_BehavesAsMarkdown(t *testing.T) {
	for name, step := range twinCreateFlows() {
		t.Run(name, func(t *testing.T) {
			// Arrange
			p := twinDeployed(t)
			twinForeignFileAtRunnerPath(t, p)

			// Act
			test, md := runTwinStep(t, p, step)

			// Assert
			assertTwinParity(t, "create over another agent's file", test, md, twinIgnoreProblemDetail())
			for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
				if got := twinItem(t, obs, owRunner).Action; got != domain.ActionCreate {
					t.Errorf("%s: plan action of %s = %s; want %s (the path is claimed by another agent)", label, owRunner, got, domain.ActionCreate)
				}
				if got := obs.Agents[owRunner].Frontmatter[twinForeignKey]; got != twinForeignValue {
					t.Errorf("%s: user-owned key %s of the file at the path = %q after the create; want %q carried as for markdown",
						label, twinForeignKey, got, twinForeignValue)
				}
			}
		})
	}
}

func TestTwinTestFormat_CreateOverAnotherAgentsFile_DoesNotCopyNativeMember(t *testing.T) {
	for name, step := range twinCreateFlows() {
		t.Run(name, func(t *testing.T) {
			// Arrange
			p := twinDeployed(t)
			twinForeignFileAtRunnerPath(t, p)

			// Act
			runTwinStep(t, p, step)

			// Assert
			written := string(gamRead(t, p.Test.Workspace, twinAgentRel(t, p.Test, owRunner)))
			if strings.Contains(written, twinForeignNative) {
				t.Errorf("the new file holds the native member of the other agent's file:\n%s", written)
			}
		})
	}
}
