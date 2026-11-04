package runflow

import (
	"fmt"
	"strings"
)

// isFailedInfraRow reports whether row is a finished infrastructure step whose
// status is not SUCCESS.
func isFailedInfraRow(row ProgressRow) bool {
	return row.Infrastructure && row.Status != "SUCCESS" && row.Status != "running" && row.Status != ""
}

// infraFailureLabel is the text of a failed infrastructure row:
// "{agent}  {STATUS}[ {ErrorCode}]  infrastructure failed: {Reason}", on one line.
func infraFailureLabel(row ProgressRow) string {
	status := row.Status
	if row.ErrorCode != "" {
		status += " " + row.ErrorCode
	}
	reason := strings.Join(strings.Fields(row.Reason), " ")
	return fmt.Sprintf("%s  %s  infrastructure failed: %s", row.AgentInstance, status, reason)
}

// renderInfraFailureRow renders a failed infrastructure row in the Error
// style, truncated to one screen line.
func (s *ProgressScreen) renderInfraFailureRow(row ProgressRow) string {
	style := s.styles.Error
	if w := s.width - 2; w > 0 {
		style = style.MaxWidth(w)
	}
	return "  " + style.Render(infraFailureLabel(row))
}
