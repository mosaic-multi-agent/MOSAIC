package descriptor

import "mosaic-deploy/internal/domain"

// ExtractToolGrants is ExtractToolEntries with dispositions: permission shape returns allow and
// deny pairs, list shape returns every item as Allow; by-convention universe names excluded;
// document order, first occurrence wins.
func ExtractToolGrants(d *domain.HarnessDescriptor, toolsValue domain.FieldValue) []domain.ToolGrant {
	if d.Tools.Shape != domain.ShapePermission {
		entries := ExtractToolEntries(d, toolsValue)
		grants := make([]domain.ToolGrant, len(entries))
		for i, name := range entries {
			grants[i] = domain.ToolGrant{Name: name, Disposition: domain.Allow}
		}
		return grants
	}
	if d.Frontmatter.ToolsKey == "" || toolsValue.Kind != domain.KindMapping {
		return nil
	}
	byConvention := map[string]bool{}
	for _, t := range d.Tools.Universe {
		if t.ByConvention {
			byConvention[t.Name] = true
		}
	}
	var grants []domain.ToolGrant
	seen := map[string]bool{}
	for _, pair := range toolsValue.Pairs {
		if pair.Key == "" || byConvention[pair.Key] || seen[pair.Key] || pair.Value.Kind != domain.KindScalar {
			continue
		}
		disposition := domain.Disposition(pair.Value.Scalar)
		if disposition != domain.Allow && disposition != domain.Deny {
			continue
		}
		seen[pair.Key] = true
		grants = append(grants, domain.ToolGrant{Name: pair.Key, Disposition: disposition})
	}
	return grants
}
