package app

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"mosaic-deploy/internal/domain"
)

// promoteToolQuestion builds the question that asks for the generic tool name of one harness
// tool with no reverse mapping. Single-file and batch promote share it, so both enforce the
// same single-name rule and show the same wording.
func promoteToolQuestion(harnessName string) domain.TextQuestion {
	return domain.TextQuestion{
		Question: domain.Question{
			ID:      domain.QPromoteCustomTool,
			Subject: harnessName,
			Title:   "Generic tool name for harness tool " + harnessName,
			Prompt:  "Enter one generic tool name.",
			Detail: "Several harness tools can be given the same name. " +
				"Leave it empty to keep the harness name.",
			AllowSkip:    true,
			AllowSkipAll: true,
		},
		Validate: validatePromoteToolName,
	}
}

// validatePromoteToolName refuses an answer that, once trimmed, holds a comma or inner
// whitespace. A blank answer is accepted; callers treat it as empty.
func validatePromoteToolName(text string) error {
	name := strings.TrimSpace(text)
	if strings.ContainsRune(name, ',') {
		return fmt.Errorf("%s: enter a single generic tool name, not a list", strconv.Quote(name))
	}
	if strings.IndexFunc(name, unicode.IsSpace) >= 0 {
		return fmt.Errorf("%s: a generic tool name cannot contain whitespace", strconv.Quote(name))
	}
	return nil
}
