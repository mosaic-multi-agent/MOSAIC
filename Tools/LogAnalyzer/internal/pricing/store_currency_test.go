package pricing_test

// Tests for the optional top-level currency key of pricing.yaml: it is read,
// validated, reported per file, kept when a price is saved, and documented in
// the template. A file without the key stays valid.

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestLoadCurrency_ProjectFileStatesEUR_TableCarriesEUR(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.projFile, withCurrency("EUR", modelYAML("model-p", "4.00")))

	table, findings := loadLayered(t, l)

	requireCurrency(t, table, domain.CurrencyEUR)
	mustLookup(t, table, "", "model-p")
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestLoadCurrency_FileWithoutKey_LoadsAndReadsAsUSD(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.projFile, modelYAML("model-p", "4.00"))

	table, findings := loadLayered(t, l)

	requireCurrency(t, table, domain.CurrencyUSD)
	mustLookup(t, table, "", "model-p")
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestLoadCurrency_CurrencyKeyOnlyFile_IsValid(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.projFile, "currency: EUR\n")

	table, findings := loadLayered(t, l)

	requireCurrency(t, table, domain.CurrencyEUR)
	if table.Len() != 0 || len(findings) != 0 {
		t.Errorf("len = %d, findings = %v, want an empty table and no findings", table.Len(), findings)
	}
}

func TestLoadCurrency_InvalidValue_OneFindingNamingTheFileAndReadAsUSD(t *testing.T) {
	for _, bad := range []string{"GBP", "usd", "eur", "Euro"} {
		t.Run(bad, func(t *testing.T) {
			l := newLayered(t)
			writeFileAt(t, l.projFile, withCurrency(bad, modelYAML("model-p", "4.00")))

			table, findings := loadLayered(t, l)

			cf := currencyFindings(findings)
			if len(cf) != 1 {
				t.Fatalf("currency findings = %v, want exactly one", findings)
			}
			if cf[0].Kind != domain.FindingInvalidPricingEntry {
				t.Errorf("finding kind = %q, want invalid_pricing_entry", cf[0].Kind)
			}
			if !namesFile(cf[0], l.projFile) {
				t.Errorf("finding %+v must name the file %s", cf[0], l.projFile)
			}
			requireCurrency(t, table, domain.CurrencyUSD)
			mustLookup(t, table, "", "model-p")
		})
	}
}

func TestPathsCurrency_ReportsTheCurrencyEachFileStates(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.globalFile, withCurrency("EUR", modelYAML("g", "3.00")))
	writeFileAt(t, l.projFile, withCurrency("USD", modelYAML("p", "3.00")))

	paths := l.store.Paths()

	if paths.Global.Currency != domain.CurrencyEUR {
		t.Errorf("global currency = %q, want EUR", paths.Global.Currency)
	}
	if paths.Project.Currency != domain.CurrencyUSD {
		t.Errorf("project currency = %q, want USD", paths.Project.Currency)
	}
}

func TestPathsCurrency_StatedEURInProjectOnly_GlobalStaysEmpty(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.projFile, withCurrency("EUR", modelYAML("p", "3.00")))

	paths := l.store.Paths()

	if paths.Project.Currency != domain.CurrencyEUR {
		t.Errorf("project currency = %q, want EUR", paths.Project.Currency)
	}
	if paths.Global.Currency != "" {
		t.Errorf("absent global file currency = %q, want empty", paths.Global.Currency)
	}
}

func TestPathsCurrency_InvalidValue_ReportsUSD(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.projFile, withCurrency("GBP", modelYAML("p", "3.00")))

	if got := l.store.Paths().Project.Currency; got != domain.CurrencyUSD {
		t.Errorf("project currency = %q, want USD for an invalid value", got)
	}
}

func TestPathsCurrency_UnparseableFile_ReportsEmpty(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.projFile, "currency: [EUR\nmodels: {{{\n")

	if got := l.store.Paths().Project.Currency; got != "" {
		t.Errorf("unparseable project file currency = %q, want empty", got)
	}
}

func TestPutCurrency_SavingAPriceKeepsTheKeyAndOtherEntries(t *testing.T) {
	for name, level := range map[string]domain.ConfigLevel{"project": domain.LevelProject, "global": domain.LevelGlobal} {
		t.Run(name, func(t *testing.T) {
			l := newLayered(t)
			file := l.projFile
			if level == domain.LevelGlobal {
				file = l.globalFile
			}
			writeFileAt(t, file, withCurrency("EUR", modelYAML("old-model", "3.00")))

			if _, err := l.store.Put(context.Background(), level, newMinimalEntry(t, "saved-model")); err != nil {
				t.Fatalf("Put: %v", err)
			}

			raw := readFile(t, file)
			if !regexp.MustCompile(`(?m)^currency:\s*"?EUR"?\s*$`).MatchString(raw) {
				t.Errorf("the currency key was dropped by the save:\n%s", raw)
			}
			table, _ := loadLayered(t, l)
			requireCurrency(t, table, domain.CurrencyEUR)
			mustLookup(t, table, "", "old-model")
			mustLookup(t, table, "", "saved-model")
		})
	}
}

func TestPutCurrency_OverwritingAnEntryKeepsTheKey(t *testing.T) {
	l := newLayered(t)
	writeFileAt(t, l.projFile, withCurrency("EUR", modelYAML("model-x", "3.00")))

	if _, err := l.store.Put(context.Background(), domain.LevelProject, newMinimalEntry(t, "model-x")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if got := l.store.Paths().Project.Currency; got != domain.CurrencyEUR {
		t.Errorf("project currency after save = %q, want EUR", got)
	}
}

func TestTemplateCurrency_DocumentsTheKeyAndBothValuesAsComments(t *testing.T) {
	_, raw := createdByPut(t, newMinimalEntry(t, "first-model"))
	template := leadingCommentBlock(raw)

	if !regexp.MustCompile(`(?m)^#.*\bcurrency\b`).MatchString(template) {
		t.Error("template must document the currency key in a comment")
	}
	for _, code := range []string{"USD", "EUR"} {
		if !strings.Contains(template, code) {
			t.Errorf("template must name the allowed value %s", code)
		}
	}
	for _, line := range activeLines(template) {
		if strings.Contains(line, "currency") {
			t.Errorf("the currency key must stay commented out in the template, found active line %q", line)
		}
	}
}

func TestTemplateCurrency_RateWordingNoLongerClaimsAFixedCurrency(t *testing.T) {
	_, raw := createdByPut(t, newMinimalEntry(t, "first-model"))
	template := leadingCommentBlock(raw)

	if regexp.MustCompile(`(?i)USD per (1M|million)`).MatchString(template) {
		t.Error("template must not claim that rates are fixed to USD per million tokens")
	}
	if !regexp.MustCompile(`(?i)per million tokens`).MatchString(template) {
		t.Error("template must still say rates are per million tokens")
	}
}

func TestTemplateCurrency_FreshFileStatesNoCurrency(t *testing.T) {
	store, _ := createdByPut(t, newMinimalEntry(t, "first-model"))

	if got := store.Paths().Project.Currency; got != "" {
		t.Errorf("a file created from the template states currency %q, want none", got)
	}
}
