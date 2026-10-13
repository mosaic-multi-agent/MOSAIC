package ghcpcli

import (
	"strings"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
)

// logOutcome records an invocation's closing debug-log entry (harness.invoke.ok
// or harness.invoke.error) for agentID. When the raw stdout names the GHCP
// version that ran (copilot self-updates, so it can differ from one invocation
// to the next), the version is added as the ghcp_version field; several
// distinct versions are joined with commas. A stream without a version path
// adds nothing.
func (a *GHCPCLIAdapter) logOutcome(event, message, agentID string, stdout []byte, extra ...domain.DebugField) {
	fields := append([]domain.DebugField{domain.F("agent", agentID)}, extra...)
	if versions := commonharness.GHCPCLIVersions(stdout); len(versions) > 0 {
		fields = append(fields, domain.F("ghcp_version", strings.Join(versions, ",")))
	}
	a.logger.Log(event, message, fields...)
}
