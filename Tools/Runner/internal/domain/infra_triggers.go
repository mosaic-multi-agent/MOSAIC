package domain

import (
	"fmt"
	"strings"
)

// Infrastructure trigger names: the closed vocabulary of the infrastructure
// agent concept. Matching is exact and case-sensitive after trimming
// surrounding whitespace. The constants are untyped strings so
// DeclaredInfraTrigger.Trigger keeps its string type.
const (
	InfraTriggerInvocationInterval = "INVOCATION_INTERVAL"
	InfraTriggerIterationEnd       = "ITERATION_END"
	InfraTriggerPhaseEnd           = "PHASE_END"
	InfraTriggerManual             = "MANUAL"
)

// InfraTriggerLegacyStageEnd is the former name of ITERATION_END. It is not
// part of the vocabulary and has no alias; it exists only so refusals can give
// rename guidance.
const InfraTriggerLegacyStageEnd = "STAGE_END"

// InfraTriggerLocation says where a trigger was found, so refusal guidance can
// name a remedy that works there.
type InfraTriggerLocation string

const (
	InfraTriggerInDeclaration InfraTriggerLocation = "declaration" // orchestrator declaration region
	InfraTriggerInOverride    InfraTriggerLocation = "override"    // run artifact infrastructure_overrides
)

// InfraTriggerVocabulary returns the four valid trigger names in the order
// INVOCATION_INTERVAL, ITERATION_END, PHASE_END, MANUAL. Each call returns a
// new slice; callers may modify it.
func InfraTriggerVocabulary() []string {
	return []string{
		InfraTriggerInvocationInterval,
		InfraTriggerIterationEnd,
		InfraTriggerPhaseEnd,
		InfraTriggerManual,
	}
}

// IsValidInfraTrigger reports whether trigger, after strings.TrimSpace, equals
// one of the four vocabulary names exactly (case-sensitive). A blank trigger,
// STAGE_END, case variants and quote-wrapped forms are not valid.
func IsValidInfraTrigger(trigger string) bool {
	t := strings.TrimSpace(trigger)
	for _, v := range InfraTriggerVocabulary() {
		if t == v {
			return true
		}
	}
	return false
}

// InfraTriggerRefusalReason returns the human-readable reason for refusing
// trigger on infrastructure agent agentName found at loc. For a valid trigger
// it returns "".
//
//   - always the agent name;
//   - blank trigger: says the agent names no trigger;
//   - STAGE_END: says it is now ITERATION_END; in a declaration says
//     redeploying the orchestrator regenerates the region, in an override says
//     the entry itself must name ITERATION_END instead;
//   - any other value: names the trigger verbatim and lists the vocabulary.
func InfraTriggerRefusalReason(agentName, trigger string, loc InfraTriggerLocation) string {
	if IsValidInfraTrigger(trigger) {
		return ""
	}
	t := strings.TrimSpace(trigger)
	switch t {
	case "":
		return fmt.Sprintf("infrastructure agent %q names no trigger; each trigger must be one of %s",
			agentName, strings.Join(InfraTriggerVocabulary(), ", "))
	case InfraTriggerLegacyStageEnd:
		base := fmt.Sprintf("infrastructure agent %q names trigger %s, which is now %s",
			agentName, InfraTriggerLegacyStageEnd, InfraTriggerIterationEnd)
		switch loc {
		case InfraTriggerInDeclaration:
			return base + "; redeploy the orchestrator to regenerate its infrastructure declaration region"
		case InfraTriggerInOverride:
			return base + "; the infrastructure_overrides entry itself must name " + InfraTriggerIterationEnd + " instead"
		default:
			return base
		}
	default:
		return fmt.Sprintf("infrastructure agent %q names unknown trigger %s; valid triggers are %s",
			agentName, t, strings.Join(InfraTriggerVocabulary(), ", "))
	}
}
