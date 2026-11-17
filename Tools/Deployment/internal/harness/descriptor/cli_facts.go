package descriptor

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"mosaic-deploy/internal/domain"
)

// Accepted values of cli.loading_mechanism.
const (
	loadingMechanismName         = "name"
	loadingMechanismRunnerLoaded = "runner-loaded"
)

// wireCLI captures the optional "cli" section with presence preserved, so a bare or null
// "cli:" is distinguishable from an absent key. The fields are decoded strictly: an unknown
// key inside the section is a decode error.
type wireCLI struct {
	keyPresent bool // the key appears in the source, even with a null value
	decoded    bool // the section was decoded from a non-null value
	fields     wireCLIFields
}

type wireCLIFields struct {
	Label            string `yaml:"label"`
	LoadingMechanism string `yaml:"loading_mechanism"`
}

// UnmarshalYAML records that the key was present and decodes the section's fields.
func (c *wireCLI) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.decoded = true
	return unmarshal(&c.fields)
}

// hasCLIKey reports whether the top-level "cli" key appears in src, even with a null value.
// The YAML decoder skips custom unmarshalers for null values.
func hasCLIKey(src []byte) bool {
	var top map[string]interface{}
	if err := yaml.Unmarshal(src, &top); err != nil {
		return false
	}
	_, ok := top["cli"]
	return ok
}

// mapWireCLI maps the cli section to its domain form: nil when the section is absent.
// A present section that decoded to nothing (a null value) is a validation error.
func mapWireCLI(w wireCLI) (*domain.CLIFacts, []ValidationError) {
	if !w.keyPresent {
		return nil, nil
	}
	if !w.decoded {
		return nil, []ValidationError{{
			Field:   "cli",
			Message: "must be a mapping with \"label\" and \"loading_mechanism\"",
		}}
	}
	return &domain.CLIFacts{
		Label:            w.fields.Label,
		LoadingMechanism: w.fields.LoadingMechanism,
	}, nil
}

// validateCLIFacts checks the values of a present cli section.
func validateCLIFacts(c *domain.CLIFacts) []ValidationError {
	if c == nil {
		return nil
	}
	var errs []ValidationError
	if strings.TrimSpace(c.Label) == "" {
		errs = append(errs, ValidationError{
			Field:   "cli.label",
			Message: "required field is empty",
		})
	}
	switch c.LoadingMechanism {
	case loadingMechanismName, loadingMechanismRunnerLoaded:
	default:
		errs = append(errs, ValidationError{
			Field: "cli.loading_mechanism",
			Message: fmt.Sprintf("must be %q or %q, got %q",
				loadingMechanismName, loadingMechanismRunnerLoaded, c.LoadingMechanism),
		})
	}
	return errs
}
