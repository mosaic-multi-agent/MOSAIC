package app_test

// Tests that the rate prompts and the save notice of the pricing flows name the
// currency of the file being written: that file's stated currency, else the
// effective currency of the loaded pricing table.

import (
	"context"
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// currencySetup configures the pricing store: the effective currency of its table
// and the currency each file states ("" = no currency key).
type currencySetup struct {
	effective domain.CurrencyCode
	global    domain.CurrencyCode
	project   domain.CurrencyCode
}

func (c currencySetup) apply(s *fakePricingStore) {
	s.table = domain.NewPricingTableIn(c.effective, nil, nil)
	s.globalCurrency = c.global
	s.projectCurrency = c.project
}

// gapWithCurrency runs the pricing-gap flow, saving at option ("global" or "project").
func gapWithCurrency(t *testing.T, c currencySetup, option string) *scriptedInteraction {
	t.Helper()
	ia := enteringInteraction(func(string) bool { return true })
	ia.levelOption = option
	svc, result, _ := providerGapAnalysisWith(t, ia, []domain.ProviderID{""}, c.apply)
	if _, err := svc.ResolvePricingGaps(context.Background(), result.Session, result.Report); err != nil {
		t.Fatalf("ResolvePricingGaps: %v", err)
	}
	return ia
}

// editWithCurrency runs the pricing-edit flow, saving at option.
func editWithCurrency(t *testing.T, c currencySetup, option string) *scriptedInteraction {
	t.Helper()
	ia := choosingInteraction(string(providerGapModel))
	ia.levelOption = option
	svc, result, _ := providerGapAnalysisWith(t, ia, []domain.ProviderID{""}, c.apply)
	if _, err := svc.EditPricing(context.Background(), result.Session, result.Report); err != nil {
		t.Fatalf("EditPricing: %v", err)
	}
	return ia
}

func ratePrompts(ia *scriptedInteraction) []string {
	var out []string
	for _, q := range ia.askTextCalls {
		if q.ID == app.QuestionPricingRate {
			out = append(out, q.Prompt)
		}
	}
	return out
}

func savedNotices(ia *scriptedInteraction) []interaction.Notice {
	var out []interaction.Notice
	for _, n := range ia.notifyCalls {
		if n.Title == "Saved" {
			out = append(out, n)
		}
	}
	return out
}

func assertAllPromptsStartWith(t *testing.T, prompts []string, prefix string) {
	t.Helper()
	if len(prompts) == 0 {
		t.Fatal("no rate question was asked")
	}
	for _, p := range prompts {
		if !strings.HasPrefix(p, prefix) {
			t.Errorf("rate prompt %q must start with %q", p, prefix)
		}
	}
}

func TestResolvePricingGaps_RatePrompt_NamesTheEffectiveCurrencyWhenTheFileStatesNone(t *testing.T) {
	ia := gapWithCurrency(t, currencySetup{effective: domain.CurrencyEUR}, "project")

	assertAllPromptsStartWith(t, ratePrompts(ia), "EUR per million tokens for shared-model ")
}

func TestResolvePricingGaps_RatePrompt_NamesTheCurrencyOfTheChosenFile(t *testing.T) {
	// The global file states EUR and the project file USD: the project currency is
	// effective, so a global save must still be asked in the file's own currency.
	c := currencySetup{effective: domain.CurrencyUSD, global: domain.CurrencyEUR, project: domain.CurrencyUSD}
	cases := []struct{ option, want string }{
		{"global", "EUR per million tokens for shared-model "},
		{"project", "USD per million tokens for shared-model "},
	}
	for _, tc := range cases {
		t.Run(tc.option, func(t *testing.T) {
			ia := gapWithCurrency(t, c, tc.option)

			assertAllPromptsStartWith(t, ratePrompts(ia), tc.want)
		})
	}
}

func TestResolvePricingGaps_RatePrompt_KeepsUSDWhenNothingStatesACurrency(t *testing.T) {
	ia := gapWithCurrency(t, currencySetup{effective: domain.CurrencyUSD}, "project")

	assertAllPromptsStartWith(t, ratePrompts(ia), "USD per million tokens for shared-model ")
}

func TestResolvePricingGaps_SaveNotice_StatesTheCurrencyOfTheFileWritten(t *testing.T) {
	c := currencySetup{effective: domain.CurrencyUSD, global: domain.CurrencyEUR, project: domain.CurrencyUSD}
	cases := []struct{ option, want string }{
		{"global", "rates in EUR"},
		{"project", "rates in USD"},
	}
	for _, tc := range cases {
		t.Run(tc.option, func(t *testing.T) {
			ia := gapWithCurrency(t, c, tc.option)

			notices := savedNotices(ia)
			if len(notices) != 1 || !strings.Contains(notices[0].Message, tc.want) {
				t.Errorf("saved notices = %+v, want one containing %q", notices, tc.want)
			}
		})
	}
}

func TestResolvePricingGaps_SaveNotice_FallsBackToTheEffectiveCurrency(t *testing.T) {
	ia := gapWithCurrency(t, currencySetup{effective: domain.CurrencyEUR}, "project")

	notices := savedNotices(ia)
	if len(notices) != 1 || !strings.Contains(notices[0].Message, "rates in EUR") {
		t.Errorf("saved notices = %+v, want one containing %q", notices, "rates in EUR")
	}
}

func TestResolvePricingGaps_SaveNotice_SaysWhenTheEntryIsIgnoredByACurrencyMismatch(t *testing.T) {
	c := currencySetup{effective: domain.CurrencyUSD, global: domain.CurrencyEUR, project: domain.CurrencyUSD}

	ia := gapWithCurrency(t, c, "global")

	notices := savedNotices(ia)
	want := "ignored while the project file's currency is USD"
	if len(notices) != 1 || !strings.Contains(notices[0].Message, want) {
		t.Errorf("saved notices = %+v, want one containing %q", notices, want)
	}
}

func TestResolvePricingGaps_SaveNotice_SaysNothingIgnoredWhenTheEntryApplies(t *testing.T) {
	cases := []struct {
		name   string
		c      currencySetup
		option string
	}{
		{"project save under a mismatch", currencySetup{effective: domain.CurrencyUSD, global: domain.CurrencyEUR, project: domain.CurrencyUSD}, "project"},
		{"global save with matching currencies", currencySetup{effective: domain.CurrencyEUR, global: domain.CurrencyEUR, project: domain.CurrencyEUR}, "global"},
		{"global save, project states none", currencySetup{effective: domain.CurrencyEUR, global: domain.CurrencyEUR}, "global"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ia := gapWithCurrency(t, tc.c, tc.option)

			for _, n := range savedNotices(ia) {
				if strings.Contains(n.Message, "ignored") {
					t.Errorf("notice %q must not say the entry is ignored", n.Message)
				}
			}
		})
	}
}

func TestEditPricing_RatePrompt_NamesTheCurrencyOfTheChosenFile(t *testing.T) {
	c := currencySetup{effective: domain.CurrencyUSD, global: domain.CurrencyEUR, project: domain.CurrencyUSD}
	cases := []struct{ option, want string }{
		{"global", "EUR per million tokens for shared-model "},
		{"project", "USD per million tokens for shared-model "},
	}
	for _, tc := range cases {
		t.Run(tc.option, func(t *testing.T) {
			ia := editWithCurrency(t, c, tc.option)

			assertAllPromptsStartWith(t, ratePrompts(ia), tc.want)
		})
	}
}

func TestEditPricing_RatePrompt_FallsBackToTheEffectiveCurrency(t *testing.T) {
	ia := editWithCurrency(t, currencySetup{effective: domain.CurrencyEUR}, "project")

	assertAllPromptsStartWith(t, ratePrompts(ia), "EUR per million tokens for shared-model ")
}

func TestEditPricing_SaveNotice_StatesTheCurrencyOfTheFileWritten(t *testing.T) {
	c := currencySetup{effective: domain.CurrencyUSD, global: domain.CurrencyEUR, project: domain.CurrencyUSD}

	ia := editWithCurrency(t, c, "global")

	notices := savedNotices(ia)
	if len(notices) != 1 || !strings.Contains(notices[0].Message, "rates in EUR") {
		t.Errorf("saved notices = %+v, want one containing %q", notices, "rates in EUR")
	}
}
