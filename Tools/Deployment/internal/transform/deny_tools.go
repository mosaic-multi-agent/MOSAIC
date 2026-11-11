package transform

import (
	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

// denyRequestedTools writes req.DenyTools as "<name>: deny" pairs in a permission-shaped main
// tools field, after tool resolution and appended tools. An existing pair of the same name is
// set to deny, so a deny always wins over an allow; a name is written once. A list-shaped
// field has no deny, so the names are ignored there and never become a grant.
func denyRequestedTools(fm *docformat.Frontmatter, names []string, desc *domain.HarnessDescriptor) []FieldChange {
	key := desc.Frontmatter.ToolsKey
	if len(names) == 0 || key == "" || desc.Tools.Shape != domain.ShapePermission {
		return nil
	}
	current, _ := fm.Get(key)
	before := renderValue(current)

	deny := domain.ScalarValue(string(domain.Deny), domain.QuotePlain)
	var pairs []domain.FieldPair
	if current.Kind == domain.KindMapping {
		pairs = append(pairs, current.Pairs...)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		found := false
		for i := range pairs {
			if pairs[i].Key == name {
				pairs[i].Value = deny
				found = true
				break
			}
		}
		if !found {
			pairs = append(pairs, domain.FieldPair{Key: name, Value: deny})
		}
	}
	newValue := domain.MappingValue(pairs)
	if renderValue(newValue) == before {
		return nil
	}
	fm.Set(key, newValue)
	return []FieldChange{{Key: key, Before: before, After: renderValue(newValue), Reason: "denied tools"}}
}
