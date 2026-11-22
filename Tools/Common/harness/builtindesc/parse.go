package builtindesc

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"mosaic-common/agentformat/formatid"
)

// wireDescriptor mirrors only the descriptor keys Common reads; every other
// key is ignored (strictness on unknown keys is Deploy's job).
type wireDescriptor struct {
	ID            string `yaml:"id"`
	DisplayName   string `yaml:"display_name"`
	AgentFormatID string `yaml:"agent_format_id"`
	Paths         struct {
		Agents wirePath `yaml:"agents"`
		Skills wirePath `yaml:"skills"`
		Hooks  wirePath `yaml:"hooks"`
	} `yaml:"paths"`
	Extensions struct {
		Agent string `yaml:"agent"`
	} `yaml:"extensions"`
	Models *struct {
		IDs        []string `yaml:"ids"`
		FormatHint string   `yaml:"format_hint"`
	} `yaml:"models"`
	Experimental *struct {
		Reason string `yaml:"reason"`
	} `yaml:"experimental"`
	CLI struct {
		Label            string `yaml:"label"`
		LoadingMechanism string `yaml:"loading_mechanism"`
	} `yaml:"cli"`
}

type wirePath struct {
	Supported bool   `yaml:"supported"`
	Project   string `yaml:"project"`
}

// Parse reads the fields Common needs from descriptor YAML and validates them.
// Keys Common does not read are ignored. origin names the source in every
// returned error. On failure the Descriptor is the zero value.
func Parse(src []byte, origin string) (Descriptor, error) {
	var top map[string]any
	if err := yaml.Unmarshal(src, &top); err != nil {
		return Descriptor{}, fmt.Errorf("%s: %w", origin, err)
	}
	var w wireDescriptor
	cliValue, hasCLI := top["cli"]
	if hasCLI {
		if _, isMap := cliValue.(map[string]any); !isMap {
			return Descriptor{}, &ValidationError{Origin: origin, Field: "cli", Message: "must be a mapping with \"label\" and \"loading_mechanism\""}
		}
	}
	if err := yaml.Unmarshal(src, &w); err != nil {
		return Descriptor{}, fmt.Errorf("%s: %w", origin, err)
	}

	fail := func(field, msg string) (Descriptor, error) {
		return Descriptor{}, &ValidationError{Origin: origin, Field: field, Message: msg}
	}

	if strings.TrimSpace(w.ID) == "" {
		return fail("id", "required field is empty")
	}
	if strings.TrimSpace(w.DisplayName) == "" {
		return fail("display_name", "required field is empty")
	}
	if !w.Paths.Agents.Supported || strings.TrimSpace(w.Paths.Agents.Project) == "" {
		return fail("paths.agents.project", "required field is empty")
	}
	skills, hooks := "", ""
	if w.Paths.Skills.Supported {
		if strings.TrimSpace(w.Paths.Skills.Project) == "" {
			return fail("paths.skills.project", "required when skills are supported")
		}
		skills = w.Paths.Skills.Project
	}
	if w.Paths.Hooks.Supported {
		if strings.TrimSpace(w.Paths.Hooks.Project) == "" {
			return fail("paths.hooks.project", "required when hooks are supported")
		}
		hooks = w.Paths.Hooks.Project
	}
	if !strings.HasPrefix(w.Extensions.Agent, ".") || len(w.Extensions.Agent) < 2 {
		return fail("extensions.agent", fmt.Sprintf("must start with a dot, got %q", w.Extensions.Agent))
	}
	format := w.AgentFormatID
	if format == "" {
		format = string(formatid.Markdown)
	}
	if !formatid.IsKnown(formatid.ID(format)) {
		return fail("agent_format_id", fmt.Sprintf("unknown agent format id %q", format))
	}

	d := Descriptor{
		ID:             w.ID,
		DisplayName:    w.DisplayName,
		AgentsDir:      w.Paths.Agents.Project,
		SkillsDir:      skills,
		HooksDir:       hooks,
		AgentExtension: w.Extensions.Agent,
		AgentFormatID:  format,
	}

	if w.Experimental != nil {
		d.ExperimentalReason = w.Experimental.Reason
	}

	if w.Models != nil {
		if len(w.Models.IDs) == 0 {
			return fail("models.ids", "must list at least one model id")
		}
		seen := make(map[string]bool, len(w.Models.IDs))
		for _, id := range w.Models.IDs {
			if strings.TrimSpace(id) == "" {
				return fail("models.ids", "contains a blank model id")
			}
			if seen[id] {
				return fail("models.ids", fmt.Sprintf("duplicate model id %q", id))
			}
			seen[id] = true
		}
		if strings.TrimSpace(w.Models.FormatHint) == "" {
			return fail("models.format_hint", "required field is empty")
		}
		d.Models = &Models{IDs: append([]string(nil), w.Models.IDs...), FormatHint: w.Models.FormatHint}
	}

	if hasCLI {
		if strings.TrimSpace(w.CLI.Label) == "" {
			return fail("cli.label", "required field is empty")
		}
		switch w.CLI.LoadingMechanism {
		case LoadingMechanismName, LoadingMechanismRunnerLoaded:
		default:
			return fail("cli.loading_mechanism", fmt.Sprintf("must be %q or %q, got %q",
				LoadingMechanismName, LoadingMechanismRunnerLoaded, w.CLI.LoadingMechanism))
		}
		if d.Models == nil {
			return fail("models", "required for a CLI-backed descriptor")
		}
		d.CLI = &CLI{Label: w.CLI.Label, LoadingMechanism: w.CLI.LoadingMechanism}
	}
	return d, nil
}

// Builtins parses and validates every embedded descriptor, in IDs() order,
// with origin "builtin:<id>". It returns the first error found, with a nil slice.
func Builtins() ([]Descriptor, error) {
	out := make([]Descriptor, 0, len(ids))
	for _, id := range ids {
		src, ok := Raw(id)
		if !ok {
			return nil, fmt.Errorf("builtin:%s: embedded descriptor missing", id)
		}
		d, err := parseEmbedded(id, src)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// parseEmbedded parses a descriptor embedded under id and requires its id
// field to equal that id.
func parseEmbedded(id string, src []byte) (Descriptor, error) {
	origin := "builtin:" + id
	d, err := Parse(src, origin)
	if err != nil {
		return Descriptor{}, err
	}
	if d.ID != id {
		return Descriptor{}, &ValidationError{Origin: origin, Field: "id",
			Message: fmt.Sprintf("id %q does not match the id it is embedded under (%q)", d.ID, id)}
	}
	return d, nil
}
