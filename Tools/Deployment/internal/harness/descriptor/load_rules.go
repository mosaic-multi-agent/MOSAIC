package descriptor

import (
	"fmt"
	"regexp"
	"strings"

	"mosaic-deploy/internal/domain"
)

// wireRuleFrame is the wire form of the fixed rules of a rules-shape descriptor.
type wireRuleFrame struct {
	Leading  []wireRule `yaml:"leading"`
	Trailing []wireRule `yaml:"trailing"`
}

// wireRule is the wire form of one fixed permission rule.
type wireRule struct {
	Action   string `yaml:"action"`
	Resource string `yaml:"resource"`
	Effect   string `yaml:"effect"`
}

func mapWireRuleFrame(w wireRuleFrame) domain.RuleFrame {
	return domain.RuleFrame{
		Leading:  mapWireRules(w.Leading),
		Trailing: mapWireRules(w.Trailing),
	}
}

func mapWireRules(ws []wireRule) []domain.PermissionRule {
	if len(ws) == 0 {
		return nil
	}
	rules := make([]domain.PermissionRule, len(ws))
	for i, r := range ws {
		rules[i] = domain.PermissionRule{Action: r.Action, Resource: r.Resource, Effect: domain.RuleEffect(r.Effect)}
	}
	return rules
}

// driveLetterPath matches a Windows drive-qualified path such as C:\code or c:/code.
var driveLetterPath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// isAbsolutePathResource reports whether a rule resource is written as an absolute path, which
// the permission matcher never matches.
func isAbsolutePathResource(resource string) bool {
	return strings.HasPrefix(resource, "/") || strings.HasPrefix(resource, `\`) ||
		strings.HasPrefix(resource, "~") || strings.HasPrefix(resource, "$") ||
		driveLetterPath.MatchString(resource)
}

// validateRulesShape checks the rules-shape parts of a descriptor: the fixed rules, the
// per-tool permission action and the tools key the rule list is written under. Rules and
// actions are only legal for the rules shape.
func validateRulesShape(d *domain.HarnessDescriptor) []ValidationError {
	var errs []ValidationError
	isRules := d.Tools.Shape == domain.ShapeRules
	frame := d.Tools.Rules

	if !isRules {
		if len(frame.Leading)+len(frame.Trailing) > 0 {
			errs = append(errs, ValidationError{
				Field:   "tools.rules",
				Message: fmt.Sprintf("fixed rules are only valid for shape %q", domain.ShapeRules),
			})
		}
		for i, t := range d.Tools.Universe {
			if t.Action != "" {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("tools.universe[%d].action", i),
					Message: fmt.Sprintf("action is only valid for shape %q", domain.ShapeRules),
				})
			}
		}
		return errs
	}

	if d.Frontmatter.ToolsKey == "" {
		errs = append(errs, ValidationError{
			Field:   "frontmatter.tools_key",
			Message: fmt.Sprintf("shape %q requires a tools_key to write the rule list under", domain.ShapeRules),
		})
	}
	errs = append(errs, validateFixedRules(frame.Leading, "tools.rules.leading")...)
	errs = append(errs, validateFixedRules(frame.Trailing, "tools.rules.trailing")...)
	for i, t := range d.Tools.Universe {
		if t.Action == domain.WholeResource {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("tools.universe[%d].action", i),
				Message: `action "*" would govern every tool; name the specific action`,
			})
		}
	}
	return errs
}

func validateFixedRules(rules []domain.PermissionRule, fieldPrefix string) []ValidationError {
	var errs []ValidationError
	for i, r := range rules {
		if r.Action == "" {
			errs = append(errs, ValidationError{
				Field: fmt.Sprintf("%s[%d].action", fieldPrefix, i), Message: "required field is empty",
			})
		}
		if r.Resource == "" {
			errs = append(errs, ValidationError{
				Field: fmt.Sprintf("%s[%d].resource", fieldPrefix, i), Message: "required field is empty",
			})
		} else if isAbsolutePathResource(r.Resource) {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("%s[%d].resource", fieldPrefix, i),
				Message: fmt.Sprintf("resource %q is an absolute path, which never matches; use a relative path or a wildcard", r.Resource),
			})
		}
		switch r.Effect {
		case domain.EffectAllow, domain.EffectAsk, domain.EffectDeny:
		default:
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("%s[%d].effect", fieldPrefix, i),
				Message: fmt.Sprintf("unknown effect %q; valid values: allow, ask, deny", r.Effect),
			})
		}
	}
	return errs
}
