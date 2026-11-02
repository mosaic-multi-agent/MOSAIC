package transform

import (
	"strings"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/agentfields"
	"mosaic-deploy/internal/domain"
)

// preserveDeployedManagedRegion fills a protocol-sourced or bundle-sourced managed region from
// the same-named region of the deployed document: its canonical content (nested user regions
// excised, they are re-emitted by the caller's capture sequence) and its version attribute.
// It reports false, leaving node untouched, when the deployed document has no such region.
func preserveDeployedManagedRegion(node *docformat.Node, name string, class domain.InjectionClass, action RegionAction, req Request, depDoc *docformat.Document) (RegionOutcome, bool) {
	if depDoc == nil {
		return RegionOutcome{}, false
	}
	region, ok := depDoc.Body().Deployed(name)
	if !ok {
		return RegionOutcome{}, false
	}
	content := ConvertLineEndings(CanonicalContent(region), sourceLineEnding(req))
	if len(content) > 0 {
		content = append(content, []byte(sourceLineEnding(req))...)
	}
	node.SetContent(content) //nolint:errcheck // Node.SetContent always returns nil; forward-compatible error return.
	if v := region.Version(); v != "" {
		node.SetVersion(v) //nolint:errcheck // a version read from a parsed tag is valid.
	}
	return RegionOutcome{
		Name:   name,
		Marker: node.Kind(),
		Class:  class,
		Action: action,
		Bytes:  len(content),
	}, true
}

// carryDeployedBundleStamp copies the bundle version stamp of the deployed frontmatter into fm
// when the deployed file carries one. It never invents a stamp.
func carryDeployedBundleStamp(fm *docformat.Frontmatter, deployed []byte) *FieldChange {
	doc, err := docformat.Parse(deployed)
	if err != nil {
		return nil
	}
	bvField, _ := agentfields.ByDeployedName("bundle_version")
	v, ok := doc.Frontmatter().Get(bvField.Deployed)
	if !ok {
		return nil
	}
	fm.Set(bvField.Deployed, v)
	return &FieldChange{
		Key:    bvField.Deployed,
		After:  renderValue(v),
		Reason: "bundle version stamp preserved from deployed file",
	}
}

// presentToolNames lists the tool names a tools field value holds, regardless of shape.
func presentToolNames(v domain.FieldValue) []string {
	var names []string
	switch v.Kind {
	case domain.KindList:
		for _, item := range v.Items {
			names = append(names, item.Scalar)
		}
	case domain.KindScalar:
		for _, part := range strings.Split(v.Scalar, ",") {
			if p := strings.TrimSpace(part); p != "" {
				names = append(names, p)
			}
		}
	case domain.KindMapping:
		for _, pair := range v.Pairs {
			names = append(names, pair.Key)
		}
	}
	return names
}

// appendRequestedTools adds req.AppendTools to the main tools field, in order, skipping names
// already present. Permission shape gets "<name>: allow"; an existing pair is set to allow.
func appendRequestedTools(fm *docformat.Frontmatter, names []string, desc *domain.HarnessDescriptor) []FieldChange {
	key := desc.Frontmatter.ToolsKey
	if len(names) == 0 || key == "" {
		return nil
	}
	current, _ := fm.Get(key)
	before := renderValue(current)

	var newValue domain.FieldValue
	if desc.Tools.Shape == domain.ShapePermission {
		newValue = appendPermissionPairs(current, names)
	} else {
		var toAppend []string
		seen := map[string]bool{}
		for _, n := range presentToolNames(current) {
			seen[n] = true
		}
		for _, n := range names {
			if n != "" && !seen[n] {
				seen[n] = true
				toAppend = append(toAppend, n)
			}
		}
		if len(toAppend) == 0 {
			return nil
		}
		newValue = appendListOrScalar(current, toAppend)
	}
	if renderValue(newValue) == before {
		return nil
	}
	fm.Set(key, newValue)
	return []FieldChange{{Key: key, Before: before, After: renderValue(newValue), Reason: "appended tools"}}
}

func appendPermissionPairs(current domain.FieldValue, names []string) domain.FieldValue {
	allow := domain.ScalarValue(string(domain.Allow), domain.QuotePlain)
	var pairs []domain.FieldPair
	if current.Kind == domain.KindMapping {
		pairs = append(pairs, current.Pairs...)
	}
	for _, name := range names {
		found := false
		for i := range pairs {
			if pairs[i].Key == name {
				pairs[i].Value = allow
				found = true
				break
			}
		}
		if !found {
			pairs = append(pairs, domain.FieldPair{Key: name, Value: allow})
		}
	}
	return domain.MappingValue(pairs)
}

func appendListOrScalar(current domain.FieldValue, toAppend []string) domain.FieldValue {
	switch current.Kind {
	case domain.KindScalar:
		existing := strings.TrimSpace(current.Scalar)
		added := strings.Join(toAppend, ", ")
		if existing != "" {
			added = existing + ", " + added
		}
		return domain.ScalarValue(added, current.Quote)
	case domain.KindList:
		items := make([]domain.FieldValue, len(current.Items), len(current.Items)+len(toAppend))
		copy(items, current.Items)
		for _, n := range toAppend {
			items = append(items, domain.ScalarValue(n, domain.QuotePlain))
		}
		return domain.ListValue(items, current.List)
	}
	items := make([]domain.FieldValue, len(toAppend))
	for i, n := range toAppend {
		items[i] = domain.ScalarValue(n, domain.QuotePlain)
	}
	return domain.ListValue(items, domain.ListFlow)
}
