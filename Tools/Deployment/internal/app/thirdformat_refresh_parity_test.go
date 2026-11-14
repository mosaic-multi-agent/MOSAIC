package app_test

// thirdformat_refresh_parity_test.go specifies that the harness-only refresh of Update behaves
// identically for the test-only agent format and for Markdown: an eligible agent with no catalog
// counterpart gets its stale managed region refreshed with the user-owned frontmatter key and the
// project region intact, and a file the format cannot read as an agent is never offered for a
// refresh and stays byte-identical.

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

const (
	twinHarnessOnlyKey = "my-harness-agent"
	twinStaleProtocol  = "OLD PROTOCOL CONTENT"
)

// twinHarnessOnlyCanonical is an eligible harness-only agent in canonical form: a transform
// stamp, a user-owned key, a stale CommunicationProtocol region and a project region.
func twinHarnessOnlyCanonical() []byte {
	return []byte("---\ntransform_version: \"1.0.0\"\n" + twinUserKey + ": " + twinUserValue + "\n---\n" +
		"<CommunicationProtocol type=\"managed\">\n" + twinStaleProtocol + "\n</CommunicationProtocol>\n" +
		"<Identity type=\"core\">\n# Harness-only agent\n" +
		"<IdentityExtension type=\"project\">\n" + twinProjectNote + "\n</IdentityExtension>\n</Identity>\n")
}

// twinScopeAnswers is the per-side option that answers the refresh-scope question for the
// harness-only agent (under both path spellings) with the protocol-only scope.
func twinScopeAnswers(t *testing.T) func(f twinFormat, ws string) func(*app.Deps) {
	dir := rbModule(t, hOpen).Descriptor().Paths.Agents.Project
	return func(f twinFormat, _ string) func(*app.Deps) {
		file := twinHarnessOnlyKey + twinExtOf(f)
		b := interactiontest.NewBuilder()
		for _, subject := range []string{strings.TrimSuffix(filepath.ToSlash(dir), "/") + "/" + file,
			filepath.Join(filepath.FromSlash(dir), file)} {
			b.AnswerSelectOne(domain.QHarnessOnlyRefreshScope, subject, string(app.RefreshProtocolOnly))
		}
		stub := b.Build()
		return func(d *app.Deps) { d.Interaction = stub }
	}
}

func TestTwinParity_HarnessOnlyRefresh_RefreshesManagedRegionAndKeepsUserContent(t *testing.T) {
	// Arrange
	p := newTestTwinPairWith(t, nil, twinScopeAnswers(t))
	runTwinStep(t, p, twinDeployNewStep)
	for _, side := range twinSides(p) {
		writeTwinAgent(t, side, twinHarnessOnlyKey, twinHarnessOnlyCanonical())
	}

	// Act
	test, md := runTwinStep(t, p, twinUpdateStep)

	// Assert
	assertTwinParity(t, "harness-only refresh", test, md)
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		a, ok := obs.Agents[twinHarnessOnlyKey]
		if !ok {
			t.Errorf("%s: the harness-only agent is not a decodable agent after the refresh", label)
			continue
		}
		if strings.Contains(a.Body, twinStaleProtocol) {
			t.Errorf("%s: the stale protocol text survived; the refresh did not rewrite the managed region", label)
		}
		if !strings.Contains(a.Body, twinProjectNote) {
			t.Errorf("%s: the project region of the harness-only agent was lost", label)
		}
		if got := a.Frontmatter[twinUserKey]; got != twinUserValue {
			t.Errorf("%s: user-owned key %s = %q after the refresh; want %q", label, twinUserKey, got, twinUserValue)
		}
		if got := twinTaken(t, obs, twinHarnessOnlyKey); got != domain.TakenUpdated {
			t.Errorf("%s: action taken for the harness-only agent = %s; want %s", label, got, domain.TakenUpdated)
		}
	}
}

func TestTwinParity_HarnessOnlyRefresh_RerunAfterRefresh_KeepsTheRefreshedContent(t *testing.T) {
	// Arrange: the harness-only agent has already been refreshed once
	p := newTestTwinPairWith(t, nil, twinScopeAnswers(t))
	runTwinStep(t, p, twinDeployNewStep)
	for _, side := range twinSides(p) {
		writeTwinAgent(t, side, twinHarnessOnlyKey, twinHarnessOnlyCanonical())
	}
	runTwinStep(t, p, twinUpdateStep)

	// Act
	test, md := runTwinStep(t, p, twinUpdateStep)

	// Assert
	assertTwinParity(t, "update after a harness-only refresh", test, md)
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		if strings.Contains(obs.Agents[twinHarnessOnlyKey].Body, twinStaleProtocol) {
			t.Errorf("%s: the stale protocol text is back after the second Update", label)
		}
	}
}

func TestTwinParity_HarnessOnlyRefresh_UnreadableFile_IsNotOfferedAndStaysUntouched(t *testing.T) {
	// Arrange: no scope answer is scripted, so asking the question would fail the run
	p := newTestTwinPair(t)
	runTwinStep(t, p, twinDeployNewStep)
	before := map[twinFormat][]byte{}
	for _, side := range twinSides(p) {
		writeTwinAgent(t, side, twinHarnessOnlyKey, twinHarnessOnlyCanonical())
		corruptTwinAgent(t, side, twinHarnessOnlyKey)
		before[side.Format] = gamRead(t, side.Workspace, twinAgentRel(t, side, twinHarnessOnlyKey))
	}

	// Act
	test, md := runTwinStep(t, p, twinUpdateStep)

	// Assert
	assertTwinParity(t, "update over an unreadable harness-only file", test, md)
	for _, side := range twinSides(p) {
		after := gamRead(t, side.Workspace, twinAgentRel(t, side, twinHarnessOnlyKey))
		if !bytes.Equal(before[side.Format], after) {
			t.Errorf("%s twin: the unreadable file was modified by Update", side.name())
		}
	}
}
