package transform

import (
	"strings"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

// undeliveredExcerptLimit caps the excerpt of undelivered orchestrator content quoted in the
// missing-region warning.
const undeliveredExcerptLimit = 80

// harnessRegionVersion returns the version tag for one harness region, chosen by region
// name and never by role: the orchestrator region takes the orchestrator injections version,
// every other harness region takes the shared injections version. An empty result means the
// region carries no version attribute.
func harnessRegionVersion(name string, req Request) string {
	if name == docformat.OrchestratorHarnessRegionName {
		return req.OrchestratorInjectionsVersion
	}
	return req.InjectionsVersion
}

// applyHarnessRegion fills a tool-managed harness region from the module's declared content.
// Harness content is refreshed on every transform; the deployed file's content is never
// consulted for tool-managed regions. The region is tagged with its own source file's version
// whether it is filled or left empty.
func applyHarnessRegion(node *docformat.Node, name string, class domain.InjectionClass, req Request) RegionOutcome {
	version := harnessRegionVersion(name, req)

	content, ok := req.Module.Injection(domain.InjectionRequest{Name: name, AgentKey: req.Key, Role: req.Role})
	if ok && content != "" {
		// Ensure the content ends with a newline so the closing tag appears on its own line.
		// Harness content is LF-normalised by the loader; adapt it to the source document's
		// prevailing line ending so the injected lines match the surrounding document.
		lineEnding := sourceLineEnding(req)
		contentBytes := ConvertLineEndings([]byte(content), lineEnding)
		if len(contentBytes) == 0 || contentBytes[len(contentBytes)-1] != '\n' {
			contentBytes = append(contentBytes, []byte(lineEnding)...)
		}
		node.SetContent(contentBytes) //nolint:errcheck // Node.SetContent always returns nil; forward-compatible error return.
		if version != "" {
			node.SetVersion(version) //nolint:errcheck // Node.SetVersion always returns nil; forward-compatible error return.
		}
		return RegionOutcome{
			Name:   name,
			Marker: node.Kind(),
			Class:  class,
			Action: RegionFilled,
			Bytes:  len(contentBytes),
		}
	}
	// No content for this region: clear it so any nested source markers are removed and the
	// capture-regenerate-reemit sequence can re-add them cleanly. Clear strips any existing
	// version attribute, so the tag is written after it.
	node.Clear() //nolint:errcheck // Node.Clear always returns nil; forward-compatible error return.
	if version != "" {
		node.SetVersion(version) //nolint:errcheck // Node.SetVersion always returns nil; forward-compatible error return.
	}
	return RegionOutcome{
		Name:   name,
		Marker: node.Kind(),
		Class:  class,
		Action: RegionEmptied,
		Bytes:  0,
	}
}

// undeliveredOrchestratorGaps returns the missing-region warning for an orchestrator source
// that has no region for the harness's orchestrator content. It returns nil for any other
// role, when the source carries the region, or when the harness has no orchestrator content.
func undeliveredOrchestratorGaps(sourceRegions []*docformat.Node, req Request) []domain.Gap {
	if req.Role != domain.RoleOrchestrator {
		return nil
	}
	for _, node := range sourceRegions {
		if node.Kind() == docformat.NodeDeployed && node.Name() == docformat.OrchestratorHarnessRegionName {
			return nil
		}
	}
	content, ok := req.Module.Injection(domain.InjectionRequest{
		Name:     docformat.OrchestratorHarnessRegionName,
		AgentKey: req.Key,
		Role:     domain.RoleOrchestrator,
	})
	if !ok || strings.TrimSpace(content) == "" {
		return nil
	}
	return []domain.Gap{{
		Kind:    domain.GapUndeliveredOrchestratorInjection,
		Subject: docformat.OrchestratorHarnessRegionName,
		Owner:   req.Key,
		Detail: "agent " + req.Key + " has no " + docformat.OrchestratorHarnessRegionName +
			" region, so the content of HarnessInjectionsOrchestrator.md was not deployed (the source has no marker); undelivered content: " +
			undeliveredExcerpt(content),
	}}
}

// undeliveredExcerpt returns a single-line excerpt of the content that had no region.
func undeliveredExcerpt(content string) string {
	flat := strings.Join(strings.Fields(content), " ")
	if len(flat) > undeliveredExcerptLimit {
		return flat[:undeliveredExcerptLimit] + "..."
	}
	return flat
}
