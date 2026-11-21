package app_test

// promote_batch_regions_test.go covers how PromoteBatch treats project and custom regions of
// the selected agents: project region content is kept (the default), no custom region reaches
// the written generic, and each agent's result names the custom regions removed from it.

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

const batchRegionsBody = "<Identity type=\"core\">\n" +
	"You are a batch regions agent.\n" +
	"<IdentityExtension type=\"project\">\n" +
	"Keep this batch project text.\n" +
	"<BatchNote type=\"custom\">\n" +
	"batch custom text\n" +
	"</BatchNote>\n" +
	"</IdentityExtension>\n" +
	"</Identity>\n"

// batchRegionsAgent is the bytes of a batch agent file whose body is batchRegionsBody.
func batchRegionsAgent() []byte {
	full := batchSrc{File: "a-agent.md"}.bytes()
	cut := bytes.Index(full, []byte("<Identity"))
	return append(append([]byte(nil), full[:cut]...), batchRegionsBody...)
}

func TestPromoteBatch_Default_KeepsProjectRegionContentRemovesCustomAndReportsIt(t *testing.T) {
	// Arrange
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t)
	writeBatchFile(t, dir, "a-agent.md", batchRegionsAgent())
	spy := batchSpyFor(t, ls, "a-agent.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")

	// Act
	res := mustRunBatch(t, ls.batchDeps(t, spy), dir)

	// Assert
	a := requirePromoted(t, res, "a-agent.md")
	data, err := os.ReadFile(a.Result.DestinationPath)
	if err != nil {
		t.Fatalf("read written generic: %v", err)
	}
	doc, err := docformat.Parse(data)
	if err != nil {
		t.Fatalf("parse written generic: %v", err)
	}
	injections := doc.Body().Injections()
	if len(injections) != 1 || string(bytes.TrimSpace(injections[0].Content())) != "Keep this batch project text." {
		t.Errorf("project regions = %d, want one holding the source text kept", len(injections))
	}
	if customs := doc.Body().CustomRegions(); len(customs) != 0 {
		t.Errorf("%d custom regions remain in the written generic", len(customs))
	}
	if want := []string{"BatchNote"}; !reflect.DeepEqual(a.Result.RemovedCustomRegions, want) {
		t.Errorf("RemovedCustomRegions = %v, want %v", a.Result.RemovedCustomRegions, want)
	}
}
