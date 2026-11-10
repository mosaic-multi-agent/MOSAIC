package cli_test

// check_index_catalog_test.go covers the per-catalog framing of the check-index output:
// every finding is shown under the catalog it belongs to, in text and in JSON.

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

// twoCatalogDriftResult is a stack result whose only drift is in the lower layer.
func twoCatalogDriftResult() app.IndexCheckResult {
	orphan := app.IndexCheckOrphan{Code: "file-orphan", Subject: "Cat/stray.md", Message: "m", Path: "/low/Workflows/Cat/stray.md"}
	high := app.IndexCheckCatalog{
		Catalog:      domain.CatalogIdentity{ID: "high-cat", Name: "High", Version: "1.0.0"},
		CatalogRoot:  "/high",
		IndexPath:    "/high/Workflows/Index.md",
		IndexPresent: true, DiskCount: 1, IndexCount: 1, Clean: true,
	}
	low := app.IndexCheckCatalog{
		Catalog:      domain.CatalogIdentity{ID: "low-cat", Name: "Low", Version: "2.0.0"},
		CatalogRoot:  "/low",
		IndexPath:    "/low/Workflows/Index.md",
		IndexPresent: true, DiskCount: 2, IndexCount: 1, Clean: false,
		FileOrphans: []app.IndexCheckOrphan{orphan},
	}
	return app.IndexCheckResult{
		CatalogRoot: "/high", IndexPath: high.IndexPath, IndexPresent: true,
		FileOrphans: []app.IndexCheckOrphan{orphan}, DiskCount: 3, IndexCount: 2, Clean: false,
		Catalogs: []app.IndexCheckCatalog{high, low},
	}
}

func runCheckIndex(t *testing.T, res app.IndexCheckResult, extra ...string) (int, string) {
	t.Helper()
	svc := &spyService{checkIndexResp: res}
	var out bytes.Buffer
	code := cli.Run(context.Background(), append([]string{"check-index"}, extra...), svc, &out, &bytes.Buffer{})
	return code, out.String()
}

func TestCheckIndexCmd_Text_ShowsEachFindingUnderItsCatalog(t *testing.T) {
	code, out := runCheckIndex(t, twoCatalogDriftResult())

	if code != cli.ExitWithSkips {
		t.Errorf("exit code = %d, want ExitWithSkips: drift in a non-top layer", code)
	}
	hi, lo := strings.Index(out, "catalog high-cat"), strings.Index(out, "catalog low-cat")
	if hi < 0 || lo < 0 || hi > lo {
		t.Fatalf("output lacks catalog headers in layer order (high %d, low %d):\n%s", hi, lo, out)
	}
	if !strings.Contains(out, "(/high)") || !strings.Contains(out, "(/low)") {
		t.Errorf("headers do not carry the catalog roots:\n%s", out)
	}
	stray := strings.Index(out, "Cat/stray.md")
	if stray < lo {
		t.Errorf("finding appears before the low catalog header (finding %d, header %d):\n%s", stray, lo, out)
	}
	if strings.Count(out, "Cat/stray.md") != 1 {
		t.Errorf("finding listed %d times, want once under its catalog:\n%s", strings.Count(out, "Cat/stray.md"), out)
	}
}

func TestCheckIndexCmd_Text_CatalogWithoutIdentityIsHeadedUnknown(t *testing.T) {
	res := app.IndexCheckResult{
		CatalogRoot: "/plain/Catalog", IndexPresent: true, Clean: true, DiskCount: 1, IndexCount: 1,
		Catalogs: []app.IndexCheckCatalog{{CatalogRoot: "/plain/Catalog", IndexPresent: true, Clean: true, DiskCount: 1, IndexCount: 1}},
	}

	code, out := runCheckIndex(t, res)

	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want ExitSuccess", code)
	}
	if !strings.Contains(out, "catalog unknown") || !strings.Contains(out, "/plain/Catalog") {
		t.Errorf("output = %q, want a header naming 'catalog unknown' and the root", out)
	}
}

// twoCatalogResult builds a two-layer result from the given per-catalog entries (high first).
func twoCatalogResult(high, low app.IndexCheckCatalog) app.IndexCheckResult {
	return app.IndexCheckResult{
		CatalogRoot: high.CatalogRoot, IndexPath: high.IndexPath, IndexPresent: high.IndexPresent,
		Clean: high.Clean && low.Clean, Catalogs: []app.IndexCheckCatalog{high, low},
	}
}

func cleanHighCatalog() app.IndexCheckCatalog {
	return app.IndexCheckCatalog{
		Catalog:     domain.CatalogIdentity{ID: "high-cat", Name: "High", Version: "1.0.0"},
		CatalogRoot: "/high", IndexPath: "/high/Workflows/Index.md", IndexPresent: true, Clean: true,
	}
}

func TestCheckIndexCmd_Text_CatalogWithoutIndex_NamesThatCatalogsIndexPath(t *testing.T) {
	low := app.IndexCheckCatalog{
		Catalog:     domain.CatalogIdentity{ID: "low-cat", Name: "Low", Version: "2.0.0"},
		CatalogRoot: "/low", IndexPath: "/low/Workflows/Index.md", IndexPresent: false, Clean: true,
	}

	_, out := runCheckIndex(t, twoCatalogResult(cleanHighCatalog(), low))

	lo := strings.Index(out, "catalog low-cat")
	if lo < 0 {
		t.Fatalf("output lacks the low catalog header:\n%s", out)
	}
	if !strings.Contains(out[lo:], "/low/Workflows/Index.md") {
		t.Errorf("no-index line under the low catalog does not name its index path:\n%s", out)
	}
	if strings.Contains(out[:lo], "/low/") {
		t.Errorf("low catalog's path appears before its header (in the high section):\n%s", out)
	}
}

func TestCheckIndexCmd_Text_IndexOrphanShownUnderItsCatalogOnly(t *testing.T) {
	low := app.IndexCheckCatalog{
		Catalog:     domain.CatalogIdentity{ID: "low-cat", Name: "Low", Version: "2.0.0"},
		CatalogRoot: "/low", IndexPath: "/low/Workflows/Index.md", IndexPresent: true, Clean: false,
		IndexOrphans: []app.IndexCheckOrphan{{Code: "index-orphan", Subject: "ghost-wf", Message: "m", Path: "/low/Workflows/Cat/ghost-wf.md"}},
	}

	code, out := runCheckIndex(t, twoCatalogResult(cleanHighCatalog(), low))

	if code != cli.ExitWithSkips {
		t.Errorf("exit code = %d, want ExitWithSkips", code)
	}
	lo := strings.Index(out, "catalog low-cat")
	ghost := strings.Index(out, "ghost-wf")
	if lo < 0 || ghost < lo {
		t.Errorf("index-orphan not listed after the low catalog header (header %d, finding %d):\n%s", lo, ghost, out)
	}
	if strings.Count(out, "ghost-wf") != 1 {
		t.Errorf("index-orphan listed %d times, want once:\n%s", strings.Count(out, "ghost-wf"), out)
	}
}

// Regression/contract coverage only, not evidence for new behavior: the JSON document is the
// IndexCheckResult struct encoded as is, so the per-catalog shape is fixed by the struct tags
// and the app-level tests are the evidence that the entries are produced.
func TestCheckIndexCmd_Json_EntriesCarryCatalogAndTheirOwnFindings(t *testing.T) {
	code, out := runCheckIndex(t, twoCatalogDriftResult(), "--output", "json")

	if code != cli.ExitWithSkips {
		t.Errorf("exit code = %d, want ExitWithSkips", code)
	}
	var doc struct {
		Catalogs []struct {
			Catalog     map[string]any   `json:"catalog"`
			CatalogRoot string           `json:"catalogRoot"`
			Clean       bool             `json:"clean"`
			FileOrphans []map[string]any `json:"fileOrphans"`
		} `json:"catalogs"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
	}
	if len(doc.Catalogs) != 2 {
		t.Fatalf("catalogs has %d entries, want 2:\n%s", len(doc.Catalogs), out)
	}
	if doc.Catalogs[0].Catalog["id"] != "high-cat" || doc.Catalogs[1].Catalog["id"] != "low-cat" {
		t.Errorf("catalog ids = %v, %v; want high-cat then low-cat", doc.Catalogs[0].Catalog["id"], doc.Catalogs[1].Catalog["id"])
	}
	if doc.Catalogs[1].CatalogRoot != "/low" || doc.Catalogs[1].Clean || len(doc.Catalogs[1].FileOrphans) != 1 {
		t.Errorf("low entry = %+v, want root /low, unclean, one orphan", doc.Catalogs[1])
	}
	if len(doc.Catalogs[0].FileOrphans) != 0 || !doc.Catalogs[0].Clean {
		t.Errorf("high entry = %+v, want clean without orphans", doc.Catalogs[0])
	}
}
