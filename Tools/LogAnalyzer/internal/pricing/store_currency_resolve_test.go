package pricing_test

// Tests for the effective currency across the global and project levels: the
// project file's currency, else the global file's, else USD. Different stated
// currencies ignore the global entries and raise one mismatch warning.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestResolveCurrency_NeitherFileStatesOne_IsUSD(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, modelYAML("g", "3.00"))
	writeFileAt(t, l.projFile, modelYAML("p", "3.00"))

	table, findings := loadLayered(t, l)

	requireCurrency(t, table, domain.CurrencyUSD)
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestResolveCurrency_NoFiles_IsUSD(t *testing.T) {
	table, _ := loadLayered(t, newLayered(t))

	requireCurrency(t, table, domain.CurrencyUSD)
}

func TestResolveCurrency_OnlyGlobalStatesOne_GlobalCurrencyApplies(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, withCurrency("EUR", modelYAML("g", "3.00")))
	writeFileAt(t, l.projFile, modelYAML("p", "4.00"))

	table, findings := loadLayered(t, l)

	requireCurrency(t, table, domain.CurrencyEUR)
	mustLookup(t, table, "", "g")
	mustLookup(t, table, "", "p")
	if len(findings) != 0 {
		t.Errorf("a keyless project file never mismatches; findings = %v", findings)
	}
}

func TestResolveCurrency_OnlyGlobalFileExistsAndStatesEUR_IsEUR(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, withCurrency("EUR", modelYAML("g", "3.00")))

	table, _ := loadLayered(t, l)

	requireCurrency(t, table, domain.CurrencyEUR)
	mustLookup(t, table, "", "g")
}

func TestResolveCurrency_OnlyProjectStatesOne_KeylessGlobalEntriesStay(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, modelYAML("g", "3.00"))
	writeFileAt(t, l.projFile, withCurrency("EUR", modelYAML("p", "4.00")))

	table, findings := loadLayered(t, l)

	requireCurrency(t, table, domain.CurrencyEUR)
	mustLookup(t, table, "", "g")
	if len(findings) != 0 {
		t.Errorf("a keyless global file never mismatches; findings = %v", findings)
	}
}

func TestResolveCurrency_BothStateTheSame_AllEntriesStay(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, withCurrency("EUR", modelYAML("g", "3.00")))
	writeFileAt(t, l.projFile, withCurrency("EUR", modelYAML("p", "4.00")))

	table, findings := loadLayered(t, l)

	requireCurrency(t, table, domain.CurrencyEUR)
	mustLookup(t, table, "", "g")
	mustLookup(t, table, "", "p")
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestResolveCurrency_Mismatch_GlobalEntriesAreIgnoredAndProjectCurrencyApplies(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, withCurrency("USD", modelYAML("g-only", "3.00")+modelEntry("g-also", "1.00", 2)))
	writeFileAt(t, l.projFile, withCurrency("EUR", modelYAML("p-only", "4.00")))

	table, _ := loadLayered(t, l)

	requireCurrency(t, table, domain.CurrencyEUR)
	requireMissing(t, table, "", "g-only")
	requireMissing(t, table, "", "g-also")
	mustLookup(t, table, "", "p-only")
	for _, model := range []string{"g-only", "g-also"} {
		ex, ok := table.Excluded(domain.PricingKey{Model: domain.ModelID(model)})
		if !ok {
			t.Errorf("%s must be recorded as excluded", model)
			continue
		}
		if ex.Level != domain.LevelGlobal || ex.Path != l.globalFile || ex.Currency != domain.CurrencyUSD {
			t.Errorf("excluded %s = %+v, want global level, path %s, currency USD", model, ex, l.globalFile)
		}
	}
	if _, ok := table.Excluded(domain.PricingKey{Model: "p-only"}); ok {
		t.Error("a project entry must not be recorded as excluded")
	}
}

func TestResolveCurrency_Mismatch_ProviderScopedGlobalEntriesAreExcludedByFullKey(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, withCurrency("USD", providerModelYAML("acme", "m1", "3.00")))
	writeFileAt(t, l.projFile, withCurrency("EUR", modelYAML("m1", "4.00")))

	table, _ := loadLayered(t, l)

	requireMissing(t, table, "acme", "m1")
	mustLookup(t, table, "", "m1")
	if _, ok := table.Excluded(domain.PricingKey{Provider: "acme", Model: "m1"}); !ok {
		t.Error("acme/m1 must be recorded as excluded")
	}
}

func TestResolveCurrency_Mismatch_RaisesOneWarningNamingBothFilesCurrenciesAndKeys(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, withCurrency("USD", modelYAML("g-only", "3.00")))
	writeFileAt(t, l.projFile, withCurrency("EUR", modelYAML("p-only", "4.00")))

	_, findings := loadLayered(t, l)

	mismatch := mismatchFindings(findings)
	if len(mismatch) != 1 {
		t.Fatalf("mismatch findings = %v, want exactly one", findings)
	}
	f := mismatch[0]
	if f.Severity != domain.SeverityWarning {
		t.Errorf("severity = %v, want warning", f.Severity)
	}
	text := f.Path + " " + f.Detail
	for _, want := range []string{l.globalFile, l.projFile, "USD", "EUR", "g-only"} {
		if !strings.Contains(text, want) {
			t.Errorf("mismatch finding %+v must mention %q", f, want)
		}
	}
}

func TestResolveCurrency_Mismatch_ProjectEntryForTheSameKeyStillPrices(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, withCurrency("USD", modelYAML("shared", "3.00")))
	writeFileAt(t, l.projFile, withCurrency("EUR", modelYAML("shared", "9.00")))

	table, _ := loadLayered(t, l)

	e := mustLookup(t, table, "", "shared")
	if e.Source != domain.LevelProject || e.Standard.Input != mustRate(t, "9.00") {
		t.Errorf("entry = source %v input %v, want the project entry at 9.00", e.Source, e.Standard.Input)
	}
}

func TestResolveCurrency_InvalidProjectValueReadsAsUSD_MismatchesAnEURGlobalFile(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, withCurrency("EUR", modelYAML("g-only", "3.00")))
	writeFileAt(t, l.projFile, withCurrency("GBP", modelYAML("p-only", "4.00")))

	table, findings := loadLayered(t, l)

	requireCurrency(t, table, domain.CurrencyUSD)
	requireMissing(t, table, "", "g-only")
	if len(mismatchFindings(findings)) != 1 || len(currencyFindings(findings)) != 2 {
		t.Errorf("findings = %v, want one invalid-value finding and one mismatch warning", findings)
	}
}

func TestResolveCurrency_InvalidGlobalValueReadsAsUSD_AgreesWithAUSDProject(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, withCurrency("GBP", modelYAML("g", "3.00")))
	writeFileAt(t, l.projFile, withCurrency("USD", modelYAML("p", "4.00")))

	table, findings := loadLayered(t, l)

	requireCurrency(t, table, domain.CurrencyUSD)
	mustLookup(t, table, "", "g")
	cf := currencyFindings(findings)
	if len(cf) != 1 || !namesFile(cf[0], l.globalFile) {
		t.Errorf("currency findings = %v, want one naming %s", cf, l.globalFile)
	}
}
