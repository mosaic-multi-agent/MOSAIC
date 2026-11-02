package pricing

import (
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"mosaic-log-analyzer/internal/domain"
)

// loadedFile is what one level's pricing file contributes.
type loadedFile struct {
	entries  []domain.ModelPricing
	findings []domain.Finding
	// currency is the currency the file states: "" when the key is absent, USD
	// when its value is invalid (reported as a finding).
	currency domain.CurrencyCode
}

// loadFile reads one level's pricing file. A missing file yields no entries and
// no error. Invalid entries become findings; each valid entry is tagged with the
// level. An unreadable file or unparseable document is an error naming the file.
func loadFile(path string, level domain.ConfigLevel) (loadedFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return loadedFile{}, nil
		}
		return loadedFile{}, fmt.Errorf("read pricing file %s: %w", path, err)
	}

	var raw pricingFileYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return loadedFile{}, fmt.Errorf("parse pricing file %s: %w", path, err)
	}

	out := loadedFile{}
	out.currency, out.findings = statedCurrency(raw.Currency, path)

	load := func(provider string, models map[string]modelEntryYAML) {
		for modelID, wireEntry := range models {
			entry, err := wireEntry.toDomain(provider, modelID)
			if err == nil {
				err = entry.Validate()
			}
			if err != nil {
				out.findings = append(out.findings, domain.Finding{
					Kind:     domain.FindingInvalidPricingEntry,
					Severity: domain.SeverityWarning,
					Detail:   fmt.Sprintf("invalid pricing entry for %s: %v", entryLabel(provider, modelID), err),
				})
				continue
			}
			entry.Source = level
			out.entries = append(out.entries, entry)
		}
	}

	load("", raw.Models)
	for providerID, section := range raw.Providers {
		if strings.TrimSpace(providerID) == "" {
			// An empty provider key would silently become a top-level entry.
			for modelID := range section.Models {
				out.findings = append(out.findings, domain.Finding{
					Kind:     domain.FindingInvalidPricingEntry,
					Severity: domain.SeverityWarning,
					Detail:   fmt.Sprintf("invalid pricing entry for %s: empty provider ID", modelID),
				})
			}
			continue
		}
		load(providerID, section.Models)
	}
	return out, nil
}

// statedCurrency interprets a file's currency key: "" when absent; the parsed
// code when valid; USD plus a finding naming the file when invalid.
func statedCurrency(raw *string, path string) (domain.CurrencyCode, []domain.Finding) {
	if raw == nil {
		return "", nil
	}
	if cur, ok := domain.ParseCurrency(*raw); ok {
		return cur, nil
	}
	return domain.DefaultCurrency, []domain.Finding{{
		Kind:     domain.FindingInvalidPricingEntry,
		Severity: domain.SeverityWarning,
		Path:     path,
		Detail: fmt.Sprintf("invalid currency %q in %s: expected USD or EUR; the file is read as USD",
			*raw, path),
	}}
}

// fileCurrency reports the currency a file states for display: "" when the file
// is absent, unreadable, unparseable or has no key; USD for an invalid value.
func fileCurrency(path string) domain.CurrencyCode {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var raw pricingFileYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ""
	}
	cur, _ := statedCurrency(raw.Currency, path)
	return cur
}
