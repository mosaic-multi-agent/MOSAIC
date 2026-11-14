package app_test

// owned_content_harness_only_refresh_test.go specifies that a harness-only agent refreshed by
// Update gets a manifest record equal to the record of the canonical form of the file the
// refresh wrote, for a Markdown harness and for Codex. A Codex file is TOML on disk, so a
// record taken over its raw bytes differs from the record of its decoded form; for Markdown
// the two coincide, so the Codex row carries the basis evidence.

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

const (
	hoKey      = "my-harness-agent"
	hoStaleMsg = "OLD PROTOCOL CONTENT"
)

// hoAgentBytes is an eligible harness-only agent (a transform stamp and a canonical Identity
// section) whose CommunicationProtocol region is stale, with a project region and no catalog
// counterpart.
func hoAgentBytes(h owHarness) []byte {
	if h.formatID == "" {
		return []byte("---\ntransform_version: \"1.0.0\"\n---\n" +
			"<CommunicationProtocol type=\"managed\">\n" + hoStaleMsg + "\n</CommunicationProtocol>\n" +
			"<Identity type=\"core\">\n# Harness-only agent\n" +
			"<IdentityExtension type=\"project\">\nTeam rules.\n</IdentityExtension>\n</Identity>\n")
	}
	return []byte("# mosaic_transform_version: 1.0.0\n# mosaic_version: 2.1.0\n\n" +
		"name = \"" + hoKey + "\"\nsandbox_mode = \"read-only\"\n" +
		"developer_instructions = '''\n" +
		"<CommunicationProtocol type=\"managed\">\n" + hoStaleMsg + "\n</CommunicationProtocol>\n" +
		"<Identity type=\"core\">\n# Harness-only agent\n" +
		"<IdentityExtension type=\"project\">\nTeam rules.\n</IdentityExtension>\n</Identity>\n'''\n")
}

// hoScopeAnswers scripts the refresh-scope answer for the agent under both path spellings.
func hoScopeAnswers(h owHarness) *interactiontest.Stub {
	b := interactiontest.NewBuilder()
	for _, subject := range []string{h.rel(hoKey), filepath.Join(filepath.FromSlash(h.dir), hoKey+h.ext)} {
		b.AnswerSelectOne(domain.QHarnessOnlyRefreshScope, subject, string(app.RefreshProtocolOnly))
	}
	return b.Build()
}

func TestWriteBasis_HarnessOnlyRefresh_RecordEqualsProbeOfWrittenFile(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			ws := owDeploy(t, h)
			gamWrite(t, ws, h.rel(hoKey), string(hoAgentBytes(h)))
			spy := &owSpy{}
			svc := owService(t, spy, func(d *app.Deps) { d.Interaction = hoScopeAnswers(h) })

			_, err := owFlowNamed(t, "Update").run(t, svc, h, ws, owFlowOpts{})

			if err != nil {
				t.Fatalf("Update with a harness-only agent: %v", err)
			}
			// The refresh item is appended to the plan after the planner ran, so it is read
			// from the plan the executor received.
			item := owPlanItem(t, spy.lastExec(t).req.Plan, hoKey)
			if item.SourcePath != "" || item.Action != domain.ActionUpdate {
				t.Fatalf("plan item = %+v; want a harness-only update (empty SourcePath) so the refresh write path is exercised", item)
			}
			if written := string(gamRead(t, ws, h.rel(hoKey))); strings.Contains(written, hoStaleMsg) {
				t.Fatalf("the stale protocol text survived; the refresh did not write the file")
			}
			owAssertChannel(t, spy)
			owAssertRecords(t, h, ws, []string{hoKey}, false)
		})
	}
}
