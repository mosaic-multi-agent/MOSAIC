package transform

import (
	"bytes"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

// sourceManagedRegionIsEmpty reports whether a source managed region holds nothing that would
// be discarded on regeneration. Project regions that are direct children are disregarded
// whatever their content (their content is a deploy-once default, carried separately), and so
// are whitespace-only custom regions. Prose, or a custom region with content, makes the region
// populated.
//
// This is deliberately local to the transform package: docformat.SourceDeployedRegionIsEmpty
// keeps its stricter meaning for the validation rule that shares it.
func sourceManagedRegionIsEmpty(node *docformat.Node) bool {
	content := node.Content()
	for _, child := range node.Children() {
		switch child.Kind() {
		case docformat.NodeInjection:
		case docformat.NodeCustom:
			if !child.IsEmpty() {
				return false
			}
		default:
			return false
		}
		content = bytes.Replace(content, child.Bytes(), nil, 1)
	}
	return len(bytes.TrimSpace(content)) == 0
}

// attachNestedSourceDefaults records, on every captured source-declared project region, the
// content its source declaration carries. It must run before the generator replaces the managed
// region's content, while the source children are still attached.
func attachNestedSourceDefaults(node *docformat.Node, capture []nestedRegionRecord) {
	sourceContent := make(map[string][]byte)
	for _, child := range node.Children() {
		if child.Kind() == docformat.NodeInjection {
			sourceContent[child.Name()] = child.Content()
		}
	}
	for i := range capture {
		if capture[i].Kind == docformat.NodeInjection {
			capture[i].SourceContent = sourceContent[capture[i].Name]
		}
	}
}

// nestedDefaultGaps returns a default-content gap for each re-emitted nested project region
// that received its source default.
func nestedDefaultGaps(outcomes []RegionOutcome, owner string) []domain.Gap {
	var gaps []domain.Gap
	for _, out := range outcomes {
		if out.Action == RegionDefaulted {
			gaps = append(gaps, domain.Gap{
				Kind:    domain.GapDefaultContentDeployed,
				Subject: out.Name,
				Owner:   owner,
			})
		}
	}
	return gaps
}

// hasNestedDefault reports whether the record carries a non-whitespace source default.
func (r nestedRegionRecord) hasNestedDefault() bool {
	return r.Kind == docformat.NodeInjection && len(bytes.TrimSpace(r.SourceContent)) > 0
}
