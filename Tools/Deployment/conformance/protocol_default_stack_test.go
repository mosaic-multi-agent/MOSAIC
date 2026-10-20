package conformance_test

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

// loadProtocolViaDefaultStack loads the protocol of a real MOSAIC root the way a run with no
// user catalog does: a stack holding only the default catalog resolves the file, and the loader
// parses exactly that file.
func loadProtocolViaDefaultStack(t *testing.T, mosaicRoot string) domain.ProtocolContent {
	t.Helper()
	st, err := stack.New(mosaicRoot, []stack.Layer{{Root: filepath.Join(mosaicRoot, "Catalog"), Default: true}})
	if err != nil {
		t.Fatalf("default-only stack from %s: %v", mosaicRoot, err)
	}
	res := st.SystemFiles().Protocol
	if err := res.Err(); err != nil {
		t.Fatalf("default stack does not supply the protocol: %v", err)
	}
	content, err := catalog.FileProtocolLoader{}.LoadProtocol(res.Supplier.Root)
	if err != nil {
		t.Fatalf("load protocol from %s: %v", res.Supplier.Root, err)
	}
	return content
}
