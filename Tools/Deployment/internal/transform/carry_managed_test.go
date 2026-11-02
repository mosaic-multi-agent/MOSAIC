package transform_test

// carry_managed_test.go covers the PreserveDeployedManaged option of transform.Request:
// protocol-sourced and bundle-sourced managed regions are taken from the deployed file
// (content, version attribute and nested user regions) instead of being regenerated from
// Request.Protocol and Request.Bundle. The option is off by default and a no-op without a
// deployed file.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// carrySource is a generic subagent source declaring one protocol region and one bundle
// region, both empty as source files require.
const carrySource = `---
id: 77
version: 3.1.0
description: Carry test agent
role: subagent
tools: [generic_a]
---

<Identity type="core">
## Identity

Carry test agent.

<AuthorityHierarchy type="managed">
</AuthorityHierarchy>

</Identity>

<CommunicationProtocol type="managed">
</CommunicationProtocol>
`

// carryDeployedTemplate is a deployed form of carrySource whose managed regions hold content
// and versions that differ from anything a request supplies. The frontmatter line for the
// bundle stamp is substituted by carryDeployed.
const carryDeployedTemplate = `---
mosaic_id: 77
mosaic_version: 3.1.0
mosaic_harness_version: 1.0.0
BUNDLE_STAMP_LINE
mosaic_role: subagent
tools: [alpha]
---

<Identity type="core">
## Identity

Carry test agent.

<AuthorityHierarchy type="managed">
DEPLOYED-AUTHORITY-TEXT
</AuthorityHierarchy>

</Identity>

<CommunicationProtocol type="managed" version="0.9">
DEPLOYED-PROTOCOL-TEXT

<ProtocolLocalRule type="custom">
NESTED-CUSTOM-TEXT
</ProtocolLocalRule>

</CommunicationProtocol>
`

// carryDeployed returns the deployed file with the given bundle stamp line ("" omits it).
func carryDeployed(bundleStampLine string) []byte {
	return []byte(strings.Replace(carryDeployedTemplate, "BUNDLE_STAMP_LINE\n", lineOrNothing(bundleStampLine), 1))
}

func lineOrNothing(line string) string {
	if line == "" {
		return ""
	}
	return line + "\n"
}

// carryRequest builds a request that supplies no Protocol and no Bundle, as the rebuild does.
func carryRequest(t *testing.T, deployed []byte, preserve bool) transform.Request {
	t.Helper()
	return transform.Request{
		Source:                  []byte(carrySource),
		Deployed:                deployed,
		Kind:                    domain.ArtifactAgent,
		Key:                     "carry-agent",
		Module:                  newDescriptorModule(t, preservationListDescriptorYAML, "inline:carry"),
		Model:                   domain.ModelSelection{Origin: domain.OriginUnresolved},
		Scope:                   domain.ScopeProject,
		Role:                    domain.RoleSubagent,
		PreserveDeployedManaged: preserve,
	}
}

func applyCarry(t *testing.T, req transform.Request) string {
	t.Helper()
	res, err := transform.Apply(req)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return string(res.Output)
}

func TestPreserveDeployedManaged_ProtocolRegion_TakesContentAndVersionFromDeployed(t *testing.T) {
	out := applyCarry(t, carryRequest(t, carryDeployed("mosaic_bundle_version: 1.5.0"), true))

	if !strings.Contains(out, "DEPLOYED-PROTOCOL-TEXT") {
		t.Errorf("protocol content of the deployed file missing from output:\n%s", out)
	}
	if !strings.Contains(out, `<CommunicationProtocol type="managed" version="0.9">`) {
		t.Errorf("protocol region must keep the deployed version attribute 0.9:\n%s", out)
	}
}

func TestPreserveDeployedManaged_ProtocolRegion_DeployedWinsOverSuppliedProtocol(t *testing.T) {
	req := carryRequest(t, carryDeployed("mosaic_bundle_version: 1.5.0"), true)
	req.Protocol = fixtureProtocol("9.9")

	out := applyCarry(t, req)

	if !strings.Contains(out, "DEPLOYED-PROTOCOL-TEXT") {
		t.Errorf("deployed protocol content must win over the supplied protocol:\n%s", out)
	}
	if strings.Contains(out, subagentBlockContent) {
		t.Errorf("supplied protocol block must not be written when the deployed region is carried:\n%s", out)
	}
	if strings.Contains(out, `version="9.9"`) {
		t.Errorf("supplied protocol version must not replace the deployed version:\n%s", out)
	}
}

func TestPreserveDeployedManaged_NestedCustomRegion_AppearsExactlyOnce(t *testing.T) {
	out := applyCarry(t, carryRequest(t, carryDeployed("mosaic_bundle_version: 1.5.0"), true))

	if n := strings.Count(out, "NESTED-CUSTOM-TEXT"); n != 1 {
		t.Errorf("nested custom region content must appear exactly once, got %d:\n%s", n, out)
	}
	if n := strings.Count(out, "DEPLOYED-PROTOCOL-TEXT"); n != 1 {
		t.Errorf("protocol content must appear exactly once, got %d:\n%s", n, out)
	}
}

func TestPreserveDeployedManaged_BundleRegion_TakesContentFromDeployedWithoutBundle(t *testing.T) {
	out := applyCarry(t, carryRequest(t, carryDeployed("mosaic_bundle_version: 1.5.0"), true))

	if !strings.Contains(out, "DEPLOYED-AUTHORITY-TEXT") {
		t.Errorf("bundle region content of the deployed file missing from output:\n%s", out)
	}
}

func TestPreserveDeployedManaged_BundleStamp_CarriesDeployedValue(t *testing.T) {
	out := applyCarry(t, carryRequest(t, carryDeployed("mosaic_bundle_version: 1.5.0"), true))

	assertFrontmatterScalar(t, out, "mosaic_bundle_version", "1.5.0")
}

func TestPreserveDeployedManaged_BundleStamp_NotInventedWhenDeployedLacksIt(t *testing.T) {
	req := carryRequest(t, carryDeployed(""), true)
	req.Bundle = domain.BundleContent{Version: "8.8.8"}

	out := applyCarry(t, req)

	if _, ok := frontmatterOf(t, out).Get("mosaic_bundle_version"); ok {
		t.Errorf("mosaic_bundle_version must be absent when the deployed file carries none:\n%s", out)
	}
}

func TestPreserveDeployedManaged_Off_ProtocolContentMissingStillRaised(t *testing.T) {
	req := carryRequest(t, carryDeployed("mosaic_bundle_version: 1.5.0"), false)

	_, err := transform.Apply(req)

	if !errors.Is(err, transform.ErrProtocolContentMissing) {
		t.Errorf("without the option and without a protocol, want ErrProtocolContentMissing, got %v", err)
	}
}

func TestPreserveDeployedManaged_WithoutDeployed_IsIgnored(t *testing.T) {
	req := carryRequest(t, nil, true)

	_, err := transform.Apply(req)

	if !errors.Is(err, transform.ErrProtocolContentMissing) {
		t.Errorf("the option is ignored without a deployed file; want ErrProtocolContentMissing, got %v", err)
	}
}

// frontmatterOf parses the output and returns its frontmatter.
func frontmatterOf(t *testing.T, out string) *docformat.Frontmatter {
	t.Helper()
	doc, err := docformat.Parse([]byte(out))
	if err != nil {
		t.Fatalf("parse output: %v\n%s", err, out)
	}
	return doc.Frontmatter()
}

// assertFrontmatterScalar asserts a frontmatter key holds exactly the given scalar.
func assertFrontmatterScalar(t *testing.T, out, key, want string) {
	t.Helper()
	v, ok := frontmatterOf(t, out).Get(key)
	if !ok {
		t.Errorf("frontmatter key %q missing:\n%s", key, out)
		return
	}
	if v.Scalar != want {
		t.Errorf("frontmatter %s: want %q, got %q", key, want, v.Scalar)
	}
}
