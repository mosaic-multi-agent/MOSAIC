package stack_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/catalog/stack/stackerr"
)

func asLayerError(t *testing.T, err error) *stackerr.LayerError {
	t.Helper()
	var le *stackerr.LayerError
	if !errors.As(err, &le) {
		t.Fatalf("error %v is not a *stackerr.LayerError", err)
	}
	return le
}

func TestNew_LayerWithoutIdentity_IsRejectedNamingTheLayer(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	good := newTestCatalogRoot(t, "good")
	bare := t.TempDir() // exists, has no identity file

	c, err := stack.New(mosaic, []stack.Layer{{Root: good}, {Root: bare}})

	if err == nil || c != nil {
		t.Fatalf("New = %v, %v; want nil catalog and an error", c, err)
	}
	if !errors.Is(err, stackerr.ErrLayerIdentity) || !errors.Is(err, identity.ErrIdentityMissing) {
		t.Errorf("err = %v; want ErrLayerIdentity wrapping identity.ErrIdentityMissing", err)
	}
	le := asLayerError(t, err)
	if le.Root != bare || le.CatalogID != "" {
		t.Errorf("LayerError = root %q id %q; want the bare layer's root and empty id", le.Root, le.CatalogID)
	}
	assertContains(t, err.Error(), bare)
}

func TestNew_LayerWithInvalidIdentity_IsRejected(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	root := t.TempDir()
	mustWrite(t, root, "catalog-identity.yaml", "id: \"Not Valid\"\nname: X\nversion: \"1\"\n")

	_, err := stack.New(mosaic, []stack.Layer{{Root: root}})

	if !errors.Is(err, stackerr.ErrLayerIdentity) || !errors.Is(err, identity.ErrIdentityInvalid) {
		t.Errorf("err = %v; want ErrLayerIdentity wrapping identity.ErrIdentityInvalid", err)
	}
}

// Fail-fast order: the layer without identity is reported, not the one that fails to load.
func TestNew_ReportsFirstFailingLayerHighestFirst(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	noIdentity := t.TempDir()
	broken := newTestCatalogRoot(t, "broken")
	mustWrite(t, broken, filepath.Join("Subagents", "Cat", "nofm.md"), "No frontmatter.\n")

	_, err := stack.New(mosaic, []stack.Layer{{Root: noIdentity}, {Root: broken}})

	if le := asLayerError(t, err); le.Root != noIdentity || !errors.Is(err, stackerr.ErrLayerIdentity) {
		t.Errorf("err = %v; want the highest failing layer (no identity)", err)
	}
}

func TestNew_UninterpretableSourceFile_FailsCompositionNamingCatalogAndFile(t *testing.T) {
	cases := map[string]string{
		"agent without frontmatter": "Just a body, no frontmatter.\n",
		"subagent missing version":  "---\nid: \"x-id\"\nname: x\nrole: subagent\n---\nBody.\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			mosaic := newTestMosaicRoot(t)
			good := newTestCatalogRoot(t, "good")
			broken := newTestCatalogRoot(t, "broken-cat")
			agentPath := filepath.Join(broken, "Subagents", "Cat", "x.md")
			mustWrite(t, broken, filepath.Join("Subagents", "Cat", "x.md"), content)

			c, err := stack.New(mosaic, []stack.Layer{{Root: good}, {Root: broken}})

			if err == nil || c != nil {
				t.Fatalf("New = %v, %v; want nil catalog and an error (layer must not be dropped)", c, err)
			}
			if !errors.Is(err, stackerr.ErrLayerLoad) || !errors.Is(err, catalog.ErrCatalogSourceInvalid) {
				t.Errorf("err = %v; want ErrLayerLoad wrapping ErrCatalogSourceInvalid", err)
			}
			le := asLayerError(t, err)
			if le.CatalogID != "broken-cat" || le.Root != broken {
				t.Errorf("LayerError = id %q root %q; want the broken layer", le.CatalogID, le.Root)
			}
			assertContains(t, err.Error(), "broken-cat")
			assertContains(t, err.Error(), agentPath)
			var sv *catalog.SourceValidationError
			if !errors.As(err, &sv) || len(sv.Failures) != 1 || sv.Failures[0].Path != agentPath {
				t.Errorf("errors.As(SourceValidationError) = %+v; want one failure at %s", sv, agentPath)
			}
		})
	}
}

func TestNew_BrokenItemShadowedByHigherLayer_StillFailsComposition(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high := newTestCatalogRoot(t, "high")
	low := newTestCatalogRoot(t, "low-broken")
	addTestSubagent(t, high, "Cat", "x", "9.0.0")
	lowPath := filepath.Join(low, "Subagents", "Cat", "x.md")
	mustWrite(t, low, filepath.Join("Subagents", "Cat", "x.md"), "No frontmatter.\n")

	c, err := stack.New(mosaic, []stack.Layer{{Root: high}, {Root: low}})

	if err == nil || c != nil {
		t.Fatalf("New = %v, %v; want failure even though the broken agent is shadowed", c, err)
	}
	if !errors.Is(err, stackerr.ErrLayerLoad) {
		t.Errorf("err = %v; want ErrLayerLoad", err)
	}
	assertContains(t, err.Error(), "low-broken")
	assertContains(t, err.Error(), lowPath)
}

func TestNew_EmptyLayerList_ReturnsErrEmptyStack(t *testing.T) {
	for name, layers := range map[string][]stack.Layer{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			c, err := stack.New(newTestMosaicRoot(t), layers)
			if c != nil || !errors.Is(err, stackerr.ErrEmptyStack) {
				t.Errorf("New = %v, %v; want nil and ErrEmptyStack", c, err)
			}
		})
	}
}

func TestNew_InvalidLayerShape_ReturnsErrInvalidLayerNamingRoot(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	defRoot := filepath.Join(mosaic, "Catalog")
	writeTestIdentity(t, defRoot, "mosaic")
	user := newTestCatalogRoot(t, "user")
	cases := map[string]struct {
		layers []stack.Layer
		root   string
	}{
		"empty root":        {[]stack.Layer{{Root: ""}}, ""},
		"relative root":     {[]stack.Layer{{Root: "relative/dir"}}, "relative/dir"},
		"default not last":  {[]stack.Layer{{Root: defRoot, Default: true}, {Root: user}}, defRoot},
		"wrong default dir": {[]stack.Layer{{Root: user, Default: true}}, user},
		"second default":    {[]stack.Layer{{Root: defRoot, Default: true}, {Root: defRoot, Default: true}}, defRoot},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, err := stack.New(mosaic, tc.layers)
			if c != nil || !errors.Is(err, stackerr.ErrInvalidLayer) {
				t.Fatalf("New = %v, %v; want nil and ErrInvalidLayer", c, err)
			}
			if le := asLayerError(t, err); le.Root != tc.root || le.CatalogID != "" {
				t.Errorf("LayerError = root %q id %q, want root %q and empty id", le.Root, le.CatalogID, tc.root)
			}
		})
	}
}

func TestNew_MissingLayerRoot_ReturnsErrLayerLoadWrappingNotExist(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := stack.New(mosaic, []stack.Layer{{Root: missing}})

	if !errors.Is(err, stackerr.ErrLayerLoad) || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v; want ErrLayerLoad wrapping fs.ErrNotExist", err)
	}
	if le := asLayerError(t, err); le.Root != missing {
		t.Errorf("LayerError.Root = %q, want %q", le.Root, missing)
	}
}

// Fail-fast order, reverse case: layer 0 fails to load, layer 1 has no identity. Each layer is
// fully checked before the next is visited, so layer 0's load failure is reported.
func TestNew_LayerLoadFailureReportedBeforeLowerLayersMissingIdentity(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	broken := newTestCatalogRoot(t, "broken")
	mustWrite(t, broken, filepath.Join("Subagents", "Cat", "nofm.md"), "No frontmatter.\n")
	noIdentity := t.TempDir()

	_, err := stack.New(mosaic, []stack.Layer{{Root: broken}, {Root: noIdentity}})

	if !errors.Is(err, stackerr.ErrLayerLoad) || errors.Is(err, stackerr.ErrLayerIdentity) {
		t.Errorf("err = %v; want the highest layer's ErrLayerLoad", err)
	}
	if le := asLayerError(t, err); le.Root != broken || le.CatalogID != "broken" {
		t.Errorf("LayerError = root %q id %q; want the broken layer", le.Root, le.CatalogID)
	}
}

func TestNew_LayerRootThatIsAFile_ReturnsErrLayerLoadNotADirectory(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	file := filepath.Join(t.TempDir(), "afile")
	mustWrite(t, filepath.Dir(file), "afile", "not a directory\n")

	_, err := stack.New(mosaic, []stack.Layer{{Root: file}})

	if !errors.Is(err, stackerr.ErrLayerLoad) || errors.Is(err, stackerr.ErrLayerIdentity) {
		t.Fatalf("err = %v; want ErrLayerLoad", err)
	}
	le := asLayerError(t, err)
	if le.Root != file || le.CatalogID != "" {
		t.Errorf("LayerError = root %q id %q; want the file path and empty id", le.Root, le.CatalogID)
	}
	assertContains(t, err.Error(), "not a directory")
}
