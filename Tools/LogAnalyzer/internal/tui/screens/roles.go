package screens

import (
	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
)

// The mapping from domain facts to the palette's signal and data roles. The
// Common kit only names the roles; deciding which fact is good, a warning or
// bad is LogAnalyzer's call and lives here.

// statusCodeRole maps a returned status code to a signal role.
func statusCodeRole(code string) tuicommon.StyleRole {
	switch code {
	case "SUCCESS":
		return tuicommon.RoleSignalGood
	case "CAPABILITY_EXCEEDED", "BLOCKED":
		return tuicommon.RoleSignalBad
	default:
		// Needs-action codes, unknown codes and no status all ask for a look.
		return tuicommon.RoleSignalWarning
	}
}

// StatusRole maps an invocation's outcome to a signal role.
func StatusRole(inv domain.Invocation) tuicommon.StyleRole {
	if inv.State == domain.InvocationOpen {
		return tuicommon.RoleSignalWarning
	}
	return statusCodeRole(inv.StatusCode)
}

// StatusGroupRole maps a status group to a signal role.
func StatusGroupRole(g domain.StatusGroup) tuicommon.StyleRole {
	if g.Kind != domain.StatusReported {
		return tuicommon.RoleSignalWarning
	}
	return statusCodeRole(g.Code)
}

// MoneyRole maps a money value to a role; complete is false for a partial total.
func MoneyRole(v domain.MoneyValue, complete bool) tuicommon.StyleRole {
	switch v.State {
	case domain.MoneyKnown:
		if !complete {
			return tuicommon.RoleSignalWarning
		}
		return tuicommon.RoleDataCost
	case domain.MoneyUnpriced:
		return tuicommon.RoleSignalWarning
	default:
		return tuicommon.RoleSignalMuted
	}
}

// TokensRole maps a token count to a role.
func TokensRole(t domain.TokenCount) tuicommon.StyleRole {
	if _, ok := t.Value(); !ok {
		return tuicommon.RoleSignalMuted
	}
	return tuicommon.RoleDataTokens
}

// BandRole maps a context band to a signal role.
func BandRole(b domain.ContextBand) tuicommon.StyleRole {
	switch b {
	case domain.BandOptimal:
		return tuicommon.RoleSignalGood
	case domain.BandDegraded:
		return tuicommon.RoleSignalBad
	default:
		return tuicommon.RoleSignalWarning
	}
}

// FindingRole maps a finding kind to a signal role: malformed or unreadable
// data is bad, every other data-quality finding is a warning.
func FindingRole(k domain.FindingKind) tuicommon.StyleRole {
	switch k {
	case domain.FindingMalformedLine, domain.FindingTruncatedLine, domain.FindingUnreadableFile,
		domain.FindingInvalidDigest, domain.FindingUnsupportedDigestVersion:
		return tuicommon.RoleSignalBad
	default:
		return tuicommon.RoleSignalWarning
	}
}

// ChangeRole maps a change direction to a signal role.
func ChangeRole(d domain.ChangeDirection) tuicommon.StyleRole {
	switch d {
	case domain.ChangeIncrease:
		return tuicommon.RoleSignalBad
	case domain.ChangeDecrease:
		return tuicommon.RoleSignalGood
	default:
		return tuicommon.RoleSignalMuted
	}
}

// ToolErrorsRole maps a tool error count to a signal role.
func ToolErrorsRole(errors int) tuicommon.StyleRole {
	if errors > 0 {
		return tuicommon.RoleSignalBad
	}
	return tuicommon.RoleSignalMuted
}
