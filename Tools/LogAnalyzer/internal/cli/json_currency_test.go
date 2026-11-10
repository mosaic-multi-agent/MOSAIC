package cli_test

// Tests that the JSON report and the per-run total carry the effective currency
// (a string, as before) and that the report gains the additive top-level
// unpriced_reasons list, omitted when empty.

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

const reasonsKey = "unpriced_reasons"

func TestEncodeReport_Currency_CarriesTheEffectiveCurrencyAsAString(t *testing.T) {
	for _, cur := range []domain.CurrencyCode{domain.CurrencyEUR, domain.CurrencyUSD} {
		m := decodeReportJSON(t, domain.Report{Currency: cur})

		got, isString := m["currency"].(string)
		if !isString || got != string(cur) {
			t.Errorf("currency = %#v, want the string %q", m["currency"], cur)
		}
	}
}

func TestEncodeRunTotal_Currency_CarriesTheEffectiveCurrencyAsAString(t *testing.T) {
	total := app.RunTotal{RunID: cliTestRunID, Currency: domain.CurrencyEUR}

	var buf bytes.Buffer
	if err := cli.EncodeRunTotal(&buf, total); err != nil {
		t.Fatalf("EncodeRunTotal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if got, isString := m["currency"].(string); !isString || got != "EUR" {
		t.Errorf("currency = %#v, want the string \"EUR\"", m["currency"])
	}
}

func TestRunTotalFor_CarriesTheReportCurrency(t *testing.T) {
	report := domain.Report{
		Currency: domain.CurrencyEUR,
		Runs: []domain.RunReport{{
			Run:          domain.NamedRun(cliTestRunID),
			Orchestrator: domain.ActorReport{Actor: domain.Orchestrator()},
		}},
	}

	total, ok := app.RunTotalFor(report, cliTestRunID)

	if !ok || total.Currency != domain.CurrencyEUR {
		t.Errorf("RunTotalFor = (%+v, %v), want Currency EUR", total, ok)
	}
}

func TestEncodeReport_UnpricedReasons_ListedWithProviderModelAndReason(t *testing.T) {
	report := domain.Report{
		Currency: domain.CurrencyEUR,
		UnpricedModels: []domain.PricingKey{
			{Model: "a-model"}, {Provider: "acme", Model: "b-model"},
		},
		UnpricedReasons: []domain.UnpricedReason{
			{Key: domain.PricingKey{Model: "a-model"}, Reason: "reason one"},
			{Key: domain.PricingKey{Provider: "acme", Model: "b-model"}, Reason: "reason two"},
		},
	}

	m := decodeReportJSON(t, report)

	want := []any{
		map[string]any{"provider": "", "model": "a-model", "reason": "reason one"},
		map[string]any{"provider": "acme", "model": "b-model", "reason": "reason two"},
	}
	if !reflect.DeepEqual(m[reasonsKey], want) {
		t.Errorf("%s = %#v, want %#v", reasonsKey, m[reasonsKey], want)
	}
}

func TestEncodeReport_UnpricedReasons_ObjectsCarryExactlyProviderModelReason(t *testing.T) {
	report := domain.Report{
		Currency: domain.CurrencyEUR,
		UnpricedReasons: []domain.UnpricedReason{
			{Key: domain.PricingKey{Provider: "acme", Model: "m"}, Reason: "r"},
		},
	}

	list, _ := decodeReportJSON(t, report)[reasonsKey].([]any)

	if len(list) != 1 {
		t.Fatalf("%s = %v, want one object", reasonsKey, list)
	}
	obj, _ := list[0].(map[string]any)
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"model", "provider", "reason"}) {
		t.Errorf("reason object keys = %v, want [model provider reason]", keys)
	}
}

func TestEncodeReport_UnpricedReasons_OmittedWhenEmpty(t *testing.T) {
	for name, reasons := range map[string][]domain.UnpricedReason{"nil": nil, "empty": {}} {
		m := decodeReportJSON(t, domain.Report{Currency: domain.CurrencyEUR, UnpricedReasons: reasons})

		if _, present := m[reasonsKey]; present {
			t.Errorf("%s: %s must be omitted when there are no reasons; got %v", name, reasonsKey, m[reasonsKey])
		}
	}
}

func TestEncodeReport_UnpricedReasons_ExistingTopLevelFieldsStayUnchanged(t *testing.T) {
	report := domain.Report{
		Currency:       domain.CurrencyEUR,
		UnpricedModels: []domain.PricingKey{{Model: "a-model"}},
	}
	without := decodeReportJSON(t, report)
	report.UnpricedReasons = []domain.UnpricedReason{{Key: domain.PricingKey{Model: "a-model"}, Reason: "r"}}

	with := decodeReportJSON(t, report)

	delete(with, reasonsKey)
	if !reflect.DeepEqual(with, without) {
		t.Errorf("adding reasons changed other fields:\nwith:    %v\nwithout: %v", with, without)
	}
	if !reflect.DeepEqual(with["unpriced_models"], []any{"a-model"}) {
		t.Errorf("unpriced_models = %#v, want [a-model]", with["unpriced_models"])
	}
}
