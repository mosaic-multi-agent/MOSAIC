package screens

import "mosaic-deploy/internal/domain"

// experimentalWarningLines returns one warning line per experimental harness the plan
// involves, in plan order. It returns nil when the plan involves none.
func experimentalWarningLines(p domain.Plan) []reviewLine {
	var lines []reviewLine
	for _, e := range p.ExperimentalHarnesses {
		lines = append(lines, reviewLine{text: e.Message(), style: "warning"})
	}
	return lines
}
