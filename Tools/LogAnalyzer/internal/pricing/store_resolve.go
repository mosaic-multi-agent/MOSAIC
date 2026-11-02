package pricing

import (
	"fmt"
	"sort"
	"strings"

	"mosaic-log-analyzer/internal/domain"
)

// resolveLevels merges the global and project files into one table. The effective
// currency is the project file's, else the global file's, else USD. When both
// files state different currencies the global entries are ignored (except keys the
// project file also prices) and a single warning names the ignored models.
func resolveLevels(global, project loadedFile, globalPath, projectPath string) (domain.PricingTable, []domain.Finding) {
	findings := append(append([]domain.Finding(nil), global.findings...), project.findings...)

	cur := domain.DefaultCurrency
	switch {
	case project.currency != "":
		cur = project.currency
	case global.currency != "":
		cur = global.currency
	}

	globalEntries := global.entries
	var excluded []domain.ExcludedPrice
	if global.currency != "" && project.currency != "" && global.currency != project.currency {
		globalEntries, excluded = nil, excludedByMismatch(global, project, globalPath)
		if len(excluded) > 0 || len(global.entries) > 0 {
			findings = append(findings, mismatchFinding(global.currency, project.currency, globalPath, projectPath, excluded))
		}
	}

	merged := append(append([]domain.ModelPricing(nil), globalEntries...), project.entries...)
	// NewPricingTableIn keeps the last entry per key, so project entries win.
	return domain.NewPricingTableIn(cur, merged, excluded), findings
}

// excludedByMismatch lists the global entries that only the global file prices;
// keys the project file also prices are overridden by it and are not excluded.
func excludedByMismatch(global, project loadedFile, globalPath string) []domain.ExcludedPrice {
	inProject := make(map[domain.PricingKey]bool, len(project.entries))
	for _, e := range project.entries {
		inProject[domain.PricingKey{Provider: e.Provider, Model: e.Model}] = true
	}
	var excluded []domain.ExcludedPrice
	for _, e := range global.entries {
		key := domain.PricingKey{Provider: e.Provider, Model: e.Model}
		if inProject[key] {
			continue
		}
		excluded = append(excluded, domain.ExcludedPrice{
			Key: key, Level: domain.LevelGlobal, Path: globalPath, Currency: global.currency,
		})
	}
	sort.Slice(excluded, func(i, j int) bool {
		a, b := excluded[i].Key, excluded[j].Key
		if a.Provider != b.Provider {
			return a.Provider < b.Provider
		}
		return a.Model < b.Model
	})
	return excluded
}

func mismatchFinding(globalCur, projectCur domain.CurrencyCode, globalPath, projectPath string, excluded []domain.ExcludedPrice) domain.Finding {
	labels := make([]string, 0, len(excluded))
	for _, e := range excluded {
		labels = append(labels, entryLabel(string(e.Key.Provider), string(e.Key.Model)))
	}
	return domain.Finding{
		Kind:     domain.FindingPricingCurrencyMismatch,
		Severity: domain.SeverityWarning,
		Path:     globalPath,
		Detail: fmt.Sprintf("global pricing file %s states %s but project pricing file %s states %s; global entries are ignored: %s",
			globalPath, globalCur, projectPath, projectCur, strings.Join(labels, ", ")),
	}
}
