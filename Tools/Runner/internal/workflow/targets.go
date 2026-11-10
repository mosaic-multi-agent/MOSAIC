package workflow

// Strict resolution of the routing cells (On Success, On Findings, Waits For)
// against the Subagent cells (T5, T6, T9, T14), and the placement of fill-ins
// (P9, P11).

import (
	"strings"

	"mosaic-common/mdtable"
	"mosaic-run/internal/domain"
)

const (
	nextEntry     = "NEXT"
	completeEntry = "COMPLETE"
)

// resolveRoutingCells fills the typed On Success, On Findings and Waits For
// entries of rows, resolving every target to the one row holding that Subagent
// cell, and checks that a Waits For column is used by some row (T13).
func resolveRoutingCells(t mdtable.Table, c routingColumns, rows []domain.RoutingRow, workflowID string) error {
	r := cellResolver{workflowID: workflowID, rowOfCell: make(map[string]int, len(rows))}
	for i, row := range rows {
		r.rowOfCell[row.Agent] = i
	}
	waitsForUsed := false
	for i, raw := range t.Rows {
		n := i + 1
		var err error
		if rows[i].Success, err = r.success(raw[c.onSuccess], n); err != nil {
			return err
		}
		if rows[i].Findings, err = r.findings(raw[c.onFindings], n); err != nil {
			return err
		}
		if c.waitsFor >= 0 {
			if rows[i].WaitsFor, err = r.waitsFor(raw[c.waitsFor], n); err != nil {
				return err
			}
			waitsForUsed = waitsForUsed || rows[i].WaitsFor != nil
		}
	}
	if c.waitsFor >= 0 && !waitsForUsed {
		return tableRefusal(workflowID, "T13", 0, waitsForColumn,
			"the Waits For column is present but no row joins; remove the column")
	}
	return nil
}

// cellResolver resolves routing cells of one table.
type cellResolver struct {
	workflowID string
	rowOfCell  map[string]int // whole Subagent cell -> zero-based row
}

func (r cellResolver) refuse(rule string, row int, token, reason string) error {
	return tableRefusal(r.workflowID, rule, row, token, reason)
}

// success decodes an On Success cell: a target, [Name], a fork list, COMPLETE
// or NEXT.
func (r cellResolver) success(cell string, row int) (domain.SuccessEntry, error) {
	entry := domain.SuccessEntry{Raw: cell}
	switch {
	case cell == "-":
		return entry, r.refuse("T5", row, cell, "On Success must hold an entry; - is 1.x notation")
	case cell == nextEntry:
		entry.Kind = domain.SuccessNext
	case cell == completeEntry:
		entry.Kind = domain.SuccessComplete
	case strings.EqualFold(cell, nextEntry) || strings.EqualFold(cell, completeEntry):
		return entry, r.refuse("", row, cell, "1.x notation: write "+nextEntry+" or "+completeEntry+" in uppercase")
	case strings.HasPrefix(cell, "["):
		return r.blockEntry(entry, row)
	case strings.Contains(cell, ","):
		return r.fork(entry, row)
	default:
		ref, err := r.target(cell, row)
		if err != nil {
			return entry, err
		}
		entry.Kind, entry.Target = domain.SuccessTarget, ref
	}
	return entry, nil
}

func (r cellResolver) blockEntry(entry domain.SuccessEntry, row int) (domain.SuccessEntry, error) {
	cell := entry.Raw
	inner := strings.TrimSuffix(strings.TrimPrefix(cell, "["), "]")
	if !strings.HasSuffix(cell, "]") || !validBlockName(inner) {
		return entry, r.refuse("T5", row, cell, "block entry must be [Name] with a letters-only block name")
	}
	if inner == legacyStageNumber {
		return entry, r.refuse("", row, cell, "1.x block reference [StageNumber] is not WorkflowFormat 2.16 notation")
	}
	entry.Kind, entry.Block = domain.SuccessBlockEntry, domain.BlockName(inner)
	return entry, nil
}

// fork decodes a comma-separated list of distinct targets.
func (r cellResolver) fork(entry domain.SuccessEntry, row int) (domain.SuccessEntry, error) {
	targets, err := r.targetList(entry.Raw, row, true)
	if err != nil {
		return entry, err
	}
	entry.Kind, entry.Fork = domain.SuccessFork, targets
	return entry, nil
}

// findings decodes an On Findings cell: exactly one target, or - for none.
func (r cellResolver) findings(cell string, row int) (domain.FindingsEntry, error) {
	if cell == "-" {
		return domain.FindingsEntry{Kind: domain.FindingsNone, Raw: cell}, nil
	}
	ref, err := r.target(cell, row)
	if err != nil {
		return domain.FindingsEntry{Raw: cell}, err
	}
	return domain.FindingsEntry{Kind: domain.FindingsTarget, Target: ref, Raw: cell}, nil
}

// waitsFor decodes a Waits For cell: - for none, else a list of distinct
// targets.
func (r cellResolver) waitsFor(cell string, row int) ([]domain.RowRef, error) {
	if cell == "-" {
		return nil, nil
	}
	return r.targetList(cell, row, false)
}

// targetList decodes a comma-separated list of distinct targets. inFork selects
// the refusal for an entry that is a reserved entry or a block entry.
func (r cellResolver) targetList(cell string, row int, inFork bool) ([]domain.RowRef, error) {
	parts := strings.Split(cell, ",")
	refs := make([]domain.RowRef, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			return nil, r.refuse("T6", row, cell, "list has an empty entry")
		}
		if inFork && (entry == nextEntry || entry == completeEntry || strings.HasPrefix(entry, "[")) {
			return nil, r.refuse("T5", row, entry, "a fork lists only targets, not "+entry)
		}
		ref, err := r.target(entry, row)
		if err != nil {
			return nil, err
		}
		if seen[entry] {
			return nil, r.refuse("T14", row, entry, "target appears twice in one cell")
		}
		seen[entry] = true
		refs = append(refs, ref)
	}
	return refs, nil
}

// target resolves one entry to the row holding that Subagent cell, compared
// whole (T6 for the shape, T9 for the match).
func (r cellResolver) target(entry string, row int) (domain.RowRef, error) {
	if _, _, ok := splitSubagentCell(entry); !ok {
		return domain.RowRef{}, r.refuse("T6", row, entry,
			"target must be a kebab-case agent id, optionally followed directly by a kebab-case qualifier in parentheses")
	}
	index, ok := r.rowOfCell[entry]
	if !ok {
		return domain.RowRef{}, r.refuse("T9", row, entry, "target matches no Subagent cell")
	}
	return domain.RowRef{Cell: entry, Index: index}, nil
}

// checkFillIns checks where fill-ins ({Name}) stand: none in the Subagent, On
// Success, On Findings or Waits For cells (P11), and in Input and Output only
// those of a block the row is in (P9). The 1.x fill-in {StageNumber} is
// refused wherever it stands in Input and Output.
func checkFillIns(raw []string, c routingColumns, parsed domain.PhaseParsed, row int, workflowID string) error {
	for _, cell := range []string{raw[c.input], raw[c.output]} {
		if err := refuseLegacyFillIn(row, cell, workflowID); err != nil {
			return err
		}
	}
	targetCols := []int{c.subagent, c.onSuccess, c.onFindings}
	if c.waitsFor >= 0 {
		targetCols = append(targetCols, c.waitsFor)
	}
	for _, col := range targetCols {
		if strings.ContainsAny(raw[col], "{}") {
			return tableRefusal(workflowID, "P11", row, raw[col],
				"the "+c.header[col]+" cell must not contain a fill-in")
		}
	}
	for _, cell := range []string{raw[c.input], raw[c.output]} {
		for _, name := range fillInNames(cell) {
			if !rowIsIn(parsed, name) {
				return tableRefusal(workflowID, "P9", row, "{"+name+"}",
					"fill-in {"+name+"} names a block the row is not in")
			}
		}
	}
	return nil
}

// fillInNames returns the names of the {Name} fill-ins in an artifact cell.
func fillInNames(cell string) []string {
	var names []string
	for {
		open := strings.IndexByte(cell, '{')
		if open < 0 {
			return names
		}
		end := strings.IndexByte(cell[open:], '}')
		if end < 0 {
			return names
		}
		names = append(names, cell[open+1:open+end])
		cell = cell[open+end+1:]
	}
}

func rowIsIn(parsed domain.PhaseParsed, name string) bool {
	for _, b := range parsed.Blocks {
		if string(b) == name {
			return true
		}
	}
	return false
}
