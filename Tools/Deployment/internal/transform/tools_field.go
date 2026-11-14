package transform

import (
	"strings"

	"mosaic-deploy/internal/domain"
)

// ToolsFieldMerge is the tools-field value written for a deployed agent and its change.
type ToolsFieldMerge struct {
	Value          domain.FieldValue
	AddedBuiltIn   []string // built-in names granted in Value but not in the deployed value
	RemovedBuiltIn []string // built-in names granted in the deployed value but not in Value
}

// toolEntry is one named tool of a tools-field value; value is set for the permission shape.
type toolEntry struct {
	name  string
	value domain.FieldValue
}

// MergeToolsField computes the tools field of a deployed agent's write.
//
// generated is the value produced from the agent's catalog tools, the effective mappings and
// this run's custom answers and tool_destinations names. deployed and deployedPresent describe
// the deployed file's value under the harness tools key. Built-in means exactly the names in
// d.Tools.Universe; custom means every other name.
//
// The result holds every built-in entry of generated (generated order and disposition), then
// every custom entry of generated whose name is absent from deployed, then every custom entry of
// deployed in deployed order exactly as deployed. Built-in entries of deployed are not carried.
// deployedPresent false returns generated unchanged. "Granted" means present for the list shape
// and disposition allow for the permission shape.
func MergeToolsField(d *domain.HarnessDescriptor, generated, deployed domain.FieldValue, deployedPresent bool) ToolsFieldMerge {
	if !deployedPresent {
		return ToolsFieldMerge{Value: generated}
	}
	permission := d.Tools.Shape == domain.ShapePermission
	universe := make(map[string]bool, len(d.Tools.Universe))
	for _, t := range d.Tools.Universe {
		universe[t.Name] = true
	}

	genEntries := toolEntries(generated)
	depEntries := toolEntries(deployed)
	depNames := make(map[string]bool, len(depEntries))
	depGranted := map[string]bool{}
	for _, e := range depEntries {
		depNames[e.name] = true
		if entryGranted(e, permission) {
			depGranted[e.name] = true
		}
	}

	var result []toolEntry
	for _, e := range genEntries {
		if universe[e.name] {
			result = append(result, e)
		}
	}
	for _, e := range genEntries {
		if !universe[e.name] && !depNames[e.name] {
			result = append(result, e)
		}
	}
	for _, e := range depEntries {
		if !universe[e.name] {
			result = append(result, e)
		}
	}

	var out ToolsFieldMerge
	resGranted := map[string]bool{}
	for _, e := range result {
		if !universe[e.name] || !entryGranted(e, permission) {
			continue
		}
		resGranted[e.name] = true
		if !depGranted[e.name] {
			out.AddedBuiltIn = append(out.AddedBuiltIn, e.name)
		}
	}
	for _, e := range depEntries {
		if universe[e.name] && depGranted[e.name] && !resGranted[e.name] {
			out.RemovedBuiltIn = append(out.RemovedBuiltIn, e.name)
		}
	}
	out.Value = buildToolsValue(result, generated, permission)
	return out
}

// toolEntries reads the named entries of a tools-field value in document order, first seen wins.
func toolEntries(v domain.FieldValue) []toolEntry {
	var out []toolEntry
	seen := map[string]bool{}
	add := func(name string, val domain.FieldValue) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, toolEntry{name: name, value: val})
	}
	switch v.Kind {
	case domain.KindMapping:
		for _, p := range v.Pairs {
			add(p.Key, p.Value)
		}
	case domain.KindList:
		for _, it := range v.Items {
			if it.Kind == domain.KindScalar {
				add(it.Scalar, it)
			}
		}
	case domain.KindScalar:
		for _, part := range strings.Split(v.Scalar, ",") {
			add(strings.TrimSpace(part), domain.FieldValue{})
		}
	}
	return out
}

// entryGranted reports whether an entry grants its tool: present for lists, an allow
// disposition (scalar, or any allowed sub-pattern) for permissions.
func entryGranted(e toolEntry, permission bool) bool {
	if !permission {
		return true
	}
	switch e.value.Kind {
	case domain.KindScalar:
		return e.value.Scalar == string(domain.Allow)
	case domain.KindMapping:
		for _, p := range e.value.Pairs {
			if p.Value.Kind == domain.KindScalar && p.Value.Scalar == string(domain.Allow) {
				return true
			}
		}
	}
	return false
}

// buildToolsValue renders entries in the shape of the generated value (a comma-separated scalar
// stays a scalar, a list keeps its style).
func buildToolsValue(entries []toolEntry, generated domain.FieldValue, permission bool) domain.FieldValue {
	if permission {
		pairs := make([]domain.FieldPair, len(entries))
		for i, e := range entries {
			pairs[i] = domain.FieldPair{Key: e.name, Value: e.value}
		}
		return domain.MappingValue(pairs)
	}
	if generated.Kind == domain.KindScalar {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.name
		}
		return domain.ScalarValue(strings.Join(names, ", "), generated.Quote)
	}
	style := domain.ListFlow
	if generated.Kind == domain.KindList {
		style = generated.List
	}
	items := make([]domain.FieldValue, len(entries))
	for i, e := range entries {
		if e.value.Kind == domain.KindScalar {
			items[i] = e.value
		} else {
			items[i] = domain.ScalarValue(e.name, domain.QuotePlain)
		}
	}
	return domain.ListValue(items, style)
}
