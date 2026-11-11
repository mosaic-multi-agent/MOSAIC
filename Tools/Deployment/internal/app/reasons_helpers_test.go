package app_test

import (
	"strings"

	"mosaic-deploy/internal/domain"
)

// reasonText joins every reason of an item into one searchable text. Tests that only care that
// a cause is mentioned search it; tests about the reason structure read item.Reasons.
func reasonText(item domain.PlanItem) string {
	return strings.Join(item.Reasons, "; ")
}
