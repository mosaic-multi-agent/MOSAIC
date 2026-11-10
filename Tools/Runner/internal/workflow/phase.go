package workflow

import (
	"strings"

	"mosaic-run/internal/domain"
)

// legacyStageNumber is the only block reference and fill-in name 1.x uses.
const legacyStageNumber = "StageNumber"

// cellDefect is a refused cell: the sec. 10 rule it breaks ("" for a 1.x
// token without a 2.16 rule) and a human-readable reason.
type cellDefect struct {
	rule   string
	reason string
}

// parsePhaseCell parses a WorkflowFormat 2.16 Phase cell: a phase name, then
// zero or more block references "[Name]" outermost first, then at most one
// group, all separated by ".". The result carries the bare phase name, the
// block references and the group. IsStaged is true when the row is in a block.
func parsePhaseCell(cell string) (domain.PhaseParsed, *cellDefect) {
	segs := strings.Split(cell, ".")
	name := segs[0]
	if !validPhaseName(name) {
		return domain.PhaseParsed{}, &cellDefect{"P12",
			"phase name must use only uppercase letters, digits and _, start with a letter and not be INIT or COMPLETED"}
	}
	parsed := domain.PhaseParsed{Name: name}
	rest := segs[1:]
	for i, seg := range rest {
		if strings.ContainsAny(seg, "[]") {
			block, defect := blockReference(seg, parsed.Blocks)
			if defect != nil {
				return domain.PhaseParsed{}, defect
			}
			parsed.Blocks = append(parsed.Blocks, block)
			continue
		}
		// A segment that is not a block reference is the group.
		if len(parsed.Blocks) == 0 {
			return domain.PhaseParsed{}, &cellDefect{"P1", "a group must follow a block reference"}
		}
		if i < len(rest)-1 {
			if hasBracket(rest[i+1:]) {
				return domain.PhaseParsed{}, &cellDefect{"P10", "block references must directly follow the phase name"}
			}
			return domain.PhaseParsed{}, &cellDefect{"P1", "at most one segment may follow the last block reference"}
		}
		if !validGroupToken(seg) {
			return domain.PhaseParsed{}, &cellDefect{"P2",
				"a group token may contain only letters, digits and - and must start with a letter or digit"}
		}
		parsed.Group = domain.GroupName(seg)
	}
	parsed.IsStaged = len(parsed.Blocks) > 0
	return parsed, nil
}

// blockReference decodes one "[Name]" segment, given the blocks already
// referenced in the row.
func blockReference(seg string, seen []domain.BlockName) (domain.BlockName, *cellDefect) {
	inner, ok := bracketName(seg)
	if !ok || !validBlockName(inner) {
		return "", &cellDefect{"P10", "malformed block reference " + seg + ": expected [Name] with a letters-only name"}
	}
	if inner == legacyStageNumber {
		return "", &cellDefect{"", "1.x block reference [StageNumber] is not WorkflowFormat 2.16 notation"}
	}
	block := domain.BlockName(inner)
	for _, s := range seen {
		if s == block {
			return "", &cellDefect{"P10", "block " + inner + " appears twice in one row"}
		}
	}
	return block, nil
}

func hasBracket(segs []string) bool {
	for _, s := range segs {
		if strings.ContainsAny(s, "[]") {
			return true
		}
	}
	return false
}

func validPhaseName(s string) bool {
	if s == "" || s == "INIT" || s == "COMPLETED" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		upper := c >= 'A' && c <= 'Z'
		if i == 0 && !upper {
			return false
		}
		if !upper && !isDigit(c) && c != '_' {
			return false
		}
	}
	return true
}

// validGroupToken reports whether s is a group or approach token: letters,
// digits and "-", starting with a letter or digit.
func validGroupToken(s string) bool {
	if s == "" || s[0] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isLetter(s[i]) && !isDigit(s[i]) && s[i] != '-' {
			return false
		}
	}
	return true
}

func validBlockName(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isLetter(s[i]) {
			return false
		}
	}
	return true
}

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
