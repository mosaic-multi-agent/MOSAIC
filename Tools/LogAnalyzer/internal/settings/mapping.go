package settings

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"mosaic-log-analyzer/internal/domain"
)

// Top-level and nested keys of the settings file.
const (
	keyWriteTools           = "write_tools"
	keyUserInteractionTools = "user_interaction_tools"
	keyDetectedCompaction   = "detected_compaction"
	keyDropPercent          = "drop_percent"
)

// parseDocument decodes the file into an order-preserving generic mapping, so a
// rewrite keeps unknown keys and entry order. An empty or comments-only document
// yields an empty mapping.
func parseDocument(data []byte) (yaml.MapSlice, error) {
	var doc yaml.MapSlice
	if err := yaml.UnmarshalWithOptions(data, &doc, yaml.UseOrderedMap()); err != nil {
		return nil, err
	}
	return doc, nil
}

// applyDocument layers the generic document onto settings: each setting the
// document sets replaces the current value (a harness list replaces that
// harness's list), every absent or invalid entry leaves it unchanged, and one
// finding is returned per invalid entry.
func applyDocument(settings *domain.AnalysisSettings, doc yaml.MapSlice, path string) []domain.Finding {
	var findings []domain.Finding
	invalid := func(format string, args ...any) {
		findings = append(findings, domain.Finding{
			Kind:     domain.FindingInvalidSettingsEntry,
			Severity: domain.SeverityWarning,
			Path:     path,
			Detail:   fmt.Sprintf(format, args...),
		})
	}

	for _, item := range doc {
		key := fmt.Sprint(item.Key)
		switch key {
		case keyWriteTools:
			applyHarnessLists(settings.WriteTools, key, item.Value, invalid)
		case keyUserInteractionTools:
			applyHarnessLists(settings.UserInteractionTools, key, item.Value, invalid)
		case keyDefaultCacheWriteTier:
			applyCacheWriteTiers(settings, item.Value, invalid)
		case keyDetectedCompaction:
			applyDetectedCompaction(settings, item.Value, invalid)
		default:
			invalid("unknown settings key %q ignored", key)
		}
	}
	return findings
}

// applyHarnessLists replaces the default list of each harness the section names.
// An invalid harness entry is reported and leaves that harness's default.
func applyHarnessLists(dst map[domain.Harness][]string, section string, value any, invalid func(string, ...any)) {
	if value == nil {
		return
	}
	entries, ok := value.(yaml.MapSlice)
	if !ok {
		invalid("%s must map harness names to lists of tool names; defaults apply", section)
		return
	}
	for _, entry := range entries {
		harness := fmt.Sprint(entry.Key)
		if harness == "" {
			invalid("%s has an empty harness key; entry ignored", section)
			continue
		}
		names, err := toolNames(entry.Value)
		if err != nil {
			invalid("%s.%s: %v; default list applies", section, harness, err)
			continue
		}
		dst[domain.Harness(harness)] = names
	}
}

// toolNames converts a YAML sequence of strings to a validated name list.
func toolNames(value any) ([]string, error) {
	seq, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("not a list of tool names")
	}
	names := make([]string, 0, len(seq))
	for _, el := range seq {
		name, ok := el.(string)
		if !ok {
			return nil, fmt.Errorf("list element %v is not a string", el)
		}
		if err := domain.ValidateToolName(name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// applyDetectedCompaction reads the detected_compaction section.
func applyDetectedCompaction(settings *domain.AnalysisSettings, value any, invalid func(string, ...any)) {
	if value == nil {
		return
	}
	entries, ok := value.(yaml.MapSlice)
	if !ok {
		invalid("%s must be a mapping; defaults apply", keyDetectedCompaction)
		return
	}
	for _, entry := range entries {
		key := fmt.Sprint(entry.Key)
		if key != keyDropPercent {
			invalid("unknown settings key %q under %s ignored", key, keyDetectedCompaction)
			continue
		}
		percent, ok := wholeNumber(entry.Value)
		if !ok || percent < 1 || percent > 99 {
			invalid("%s.%s must be an integer from 1 to 99, got %v; default %d applies",
				keyDetectedCompaction, keyDropPercent, entry.Value, domain.DefaultCompactionDropPercent)
			continue
		}
		settings.CompactionDropPercent = percent
	}
}

// wholeNumber returns v as an int when the YAML value is an integer.
func wholeNumber(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case uint64:
		if n > 1<<31 {
			return 0, false
		}
		return int(n), true
	}
	return 0, false
}
