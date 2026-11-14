package descriptor

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

// wireExperimental captures the optional "experimental" key with presence preserved, so a
// bare or null "experimental:" is distinguishable from an absent key. The raw value is
// validated by experimentalReason rather than decoded into a typed struct, so every
// malformed form yields a ValidationError naming the field.
type wireExperimental struct {
	present bool
	raw     interface{}
}

// UnmarshalYAML records that the key was present and keeps its raw decoded value.
func (e *wireExperimental) UnmarshalYAML(unmarshal func(interface{}) error) error {
	e.present = true
	return unmarshal(&e.raw)
}

// experimentalReason validates the experimental declaration and returns the trimmed reason.
// An absent key yields an empty reason and no errors; every present-but-incomplete form
// yields a ValidationError.
func experimentalReason(e wireExperimental) (string, []ValidationError) {
	if !e.present {
		return "", nil
	}
	const bareMsg = "must be a mapping with a non-blank \"reason\" (e.g. experimental:\\n  reason: \"...\")"
	var entries yaml.MapSlice
	switch v := e.raw.(type) {
	case nil:
		return "", []ValidationError{{Field: "experimental", Message: bareMsg}}
	case yaml.MapSlice:
		entries = v
	case map[string]interface{}:
		for k, val := range v {
			entries = append(entries, yaml.MapItem{Key: k, Value: val})
		}
	default:
		return "", []ValidationError{{Field: "experimental", Message: bareMsg}}
	}

	var reason interface{}
	hasReason := false
	for _, item := range entries {
		key := fmt.Sprint(item.Key)
		if key != "reason" {
			return "", []ValidationError{{
				Field:   "experimental",
				Message: fmt.Sprintf("unknown key %q: only \"reason\" is allowed", key),
			}}
		}
		reason, hasReason = item.Value, true
	}
	if !hasReason {
		return "", []ValidationError{{Field: "experimental.reason", Message: "is required when experimental is declared"}}
	}
	text, ok := reason.(string)
	if !ok {
		return "", []ValidationError{{Field: "experimental.reason", Message: "must be a string"}}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", []ValidationError{{Field: "experimental.reason", Message: "must not be blank"}}
	}
	return text, nil
}

// hasExperimentalKey reports whether the top-level "experimental" key appears in src, even
// with a null value. The YAML decoder skips custom unmarshalers for null values, so a bare
// "experimental:" would otherwise look identical to an absent key.
func hasExperimentalKey(src []byte) bool {
	var top map[string]interface{}
	if err := yaml.Unmarshal(src, &top); err != nil {
		return false
	}
	_, ok := top["experimental"]
	return ok
}
