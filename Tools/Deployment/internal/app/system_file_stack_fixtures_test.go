package app_test

// system_file_stack_fixtures_test.go holds the fixtures shared by the tests of protocol and
// bundle loading through the catalog stack: catalog layers that carry (or lack) the two system
// files, a real stack composed from them, and a catalog wrapper that gives the app's minimal
// test catalog the stack's resolution. Nothing in this file is a test itself.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/catalog/stack/systemfile"
	"mosaic-deploy/internal/domain"
)

// protocolDocFmt is a structurally valid protocol source. The first verb is the document
// version, the second names the layer in the block text so the supplier can be recognised.
const protocolDocFmt = `---
version: "%s"
---

<CommunicationProtocol type="core" name="Subagent">
## Communication Protocol

Subagent protocol text from %s.
</CommunicationProtocol>

<CommunicationProtocol type="core" name="Orchestrator">
## Communication Protocol

Orchestrator protocol text from %s.
</CommunicationProtocol>
`

// bundleDocFmt is a structurally valid bundle source with one block. The verb is the bundle
// version.
const bundleDocFmt = `---
bundle_version: "%s"
blocks:
  - name: "ClosingProcedure:Subagent"
    target: ClosingProcedure
    applies_to: subagent
    specified_in: Development/Designs/AgentTemplateArchitecture.md
---

<ClosingProcedure type="core" name="Subagent">
Follow the closing procedure.
</ClosingProcedure>
`

// sysLayer describes one fixture catalog layer. An empty protocolVersion or bundleVersion
// leaves that system file out of the layer.
type sysLayer struct {
	id              string
	protocolVersion string
	bundleVersion   string
}

// writeSysLayer creates a catalog root with an identity and the system files described by l.
// For the default layer, root is <mosaicRoot>/Catalog.
func writeSysLayer(t *testing.T, root string, l sysLayer) {
	t.Helper()
	writeSysFile(t, catalogpaths.IdentityFile(root),
		"id: "+l.id+"\nname: Fixture "+l.id+"\nversion: \"1.0.0\"\n")
	if l.protocolVersion != "" {
		writeSysFile(t, filepath.Join(root, catalogpaths.RelProtocolFile),
			fmt.Sprintf(protocolDocFmt, l.protocolVersion, l.id, l.id))
	}
	if l.bundleVersion != "" {
		writeSysFile(t, filepath.Join(root, catalogpaths.RelBundleFile),
			fmt.Sprintf(bundleDocFmt, l.bundleVersion))
	}
}

func writeSysFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent of %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

// newSysMosaicRoot returns a MOSAIC root whose default catalog (<root>/Catalog) carries the
// given layer. Whether it takes part in a stack is up to the test.
func newSysMosaicRoot(t *testing.T, def sysLayer) string {
	t.Helper()
	root := t.TempDir()
	writeSysFile(t, filepath.Join(root, catalogpaths.MosaicRelSourceFilesFormatFile), "# CatalogFilesFormat\n")
	writeSysLayer(t, filepath.Join(root, catalogpaths.CatalogDirName), def)
	return root
}

// newSysUserLayer returns a user catalog root described by l.
func newSysUserLayer(t *testing.T, l sysLayer) string {
	t.Helper()
	root := t.TempDir()
	writeSysLayer(t, root, l)
	return root
}

// resolvedStackCatalog is the app's minimal test catalog with the resolution of a real stack.
// It satisfies stack.Catalog. Its inherited CatalogRoot() points nowhere useful, so a
// loader that ignores the stack's resolution cannot find a file through it.
type resolvedStackCatalog struct {
	catalog.Catalog
	st stack.Catalog
}

func (c resolvedStackCatalog) Layers() []domain.CatalogLayer { return c.st.Layers() }
func (c resolvedStackCatalog) Provenance(i domain.CatalogItem) (domain.CatalogLayer, bool) {
	return c.st.Provenance(i)
}
func (c resolvedStackCatalog) Shadows() []stackrec.Shadow           { return c.st.Shadows() }
func (c resolvedStackCatalog) SystemFiles() systemfile.Result       { return c.st.SystemFiles() }
func (c resolvedStackCatalog) HarnessInjections() injections.Result { return c.st.HarnessInjections() }

var _ stack.Catalog = resolvedStackCatalog{}

// useResolvedStack builds a real stack from layers (highest first) over mosaicRoot and
// installs it as deps.Catalog on top of the current catalog. The real file loaders are
// installed too, so what the run parses is decided by the stack's resolution alone.
func useResolvedStack(t *testing.T, deps *app.Deps, mosaicRoot string, layers ...stack.Layer) {
	t.Helper()
	st, err := stack.New(mosaicRoot, layers)
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}
	deps.Catalog = resolvedStackCatalog{Catalog: deps.Catalog, st: st}
	deps.MosaicRoot = mosaicRoot
	deps.ProtocolLoader = catalog.FileProtocolLoader{}
	deps.BundleLoader = catalog.FileBundleLoader{}
}

// defaultStackLayer is the stack layer entry for the default catalog of mosaicRoot.
func defaultStackLayer(mosaicRoot string) stack.Layer {
	return stack.Layer{Root: filepath.Join(mosaicRoot, catalogpaths.CatalogDirName), Default: true}
}
