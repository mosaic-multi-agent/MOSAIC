package descriptor

import "mosaic-deploy/internal/domain"

// appendCustomPermissionPairs adds one "<name>: allow" pair for every custom tool name that is
// not already a universe tool, in the order given, each name once. Names equal to a universe
// tool are skipped here because the universe pair is already set to allow by the caller.
func appendCustomPermissionPairs(pairs []domain.FieldPair, universe []domain.HarnessTool, customNames []string) []domain.FieldPair {
	seen := make(map[string]bool, len(universe)+len(customNames))
	for _, t := range universe {
		seen[t.Name] = true
	}
	for _, name := range customNames {
		if seen[name] {
			continue
		}
		seen[name] = true
		pairs = append(pairs, domain.FieldPair{
			Key:   name,
			Value: domain.FieldValue{Kind: domain.KindScalar, Scalar: string(domain.Allow)},
		})
	}
	return pairs
}
