package transform

import (
	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

// writeToolFields writes the tool-derived frontmatter fields: the fields produced from the
// catalog tools and effective mappings, the run's appended tools, and, on update, the merge
// with the deployed tools field (built-in tools follow the catalog, custom tools stay).
// Every key written is recorded in touchedKeys. The merge describes the built-in change of the
// tools field; it is the zero value when there is no deployed tools field to merge with.
func writeToolFields(req Request, fm *docformat.Frontmatter, desc *domain.HarnessDescriptor, toolResult domain.ToolResult, touchedKeys map[string]bool) ([]FieldChange, ToolsFieldMerge) {
	var changes []FieldChange
	for _, field := range toolResult.Fields {
		touchedKeys[field.Key] = true
		before := ""
		if v, ok := fm.Get(field.Key); ok {
			before = renderValue(v)
		}
		changes = append(changes, FieldChange{
			Key:    field.Key,
			Before: before,
			After:  renderValue(field.Value),
			Reason: "tool mapping",
		})
		fm.Set(field.Key, field.Value)
	}

	changes = append(changes, appendRequestedTools(fm, req.AppendTools, desc)...)
	mergeChanges, merge := mergeDeployedToolsField(req, fm, desc, touchedKeys)
	changes = append(changes, mergeChanges...)
	changes = append(changes, denyRequestedTools(fm, req.DenyTools, desc)...)
	return changes, merge
}

// mergeDeployedToolsField replaces the generated tools field in fm with its merge against the
// deployed file's tools field. It does nothing on create, when the deployed file cannot be
// parsed, or when it has no tools field.
func mergeDeployedToolsField(req Request, fm *docformat.Frontmatter, desc *domain.HarnessDescriptor, touchedKeys map[string]bool) ([]FieldChange, ToolsFieldMerge) {
	key := desc.Frontmatter.ToolsKey
	if req.Deployed == nil || key == "" {
		return nil, ToolsFieldMerge{}
	}
	deployedDoc, err := docformat.Parse(req.Deployed)
	if err != nil {
		return nil, ToolsFieldMerge{}
	}
	deployedValue, present := deployedDoc.Frontmatter().Get(key)
	if !present {
		return nil, ToolsFieldMerge{}
	}
	generated, _ := fm.Get(key)
	merged := MergeToolsField(desc, generated, deployedValue, true)
	touchedKeys[key] = true
	before := renderValue(generated)
	if renderValue(merged.Value) == before {
		return nil, merged
	}
	fm.Set(key, merged.Value)
	return []FieldChange{{
		Key:    key,
		Before: before,
		After:  renderValue(merged.Value),
		Reason: "tool preservation",
	}}, merged
}

// ToolsChange returns the built-in tools the write of req adds to and removes from the deployed
// tools field, computed by the same steps Apply performs. The merge is the zero value when req
// has no deployed file or the deployed file has no tools field.
func ToolsChange(req Request) (ToolsFieldMerge, error) {
	if req.Module == nil {
		return ToolsFieldMerge{}, nil
	}
	desc := req.Module.Descriptor()
	doc, err := docformat.Parse(req.Source)
	if err != nil {
		return ToolsFieldMerge{}, err
	}
	fm := doc.Frontmatter()
	toolResult, err := resolveTools(req, fm, desc)
	if err != nil {
		return ToolsFieldMerge{}, err
	}
	if desc.Frontmatter.ToolsKey != "" {
		fm.Remove(desc.Frontmatter.ToolsKey)
	}
	_, merge := writeToolFields(req, fm, desc, toolResult, map[string]bool{})
	return merge, nil
}
