package catalog_test

// protocol_loader_path_test.go verifies where the protocol loader looks for the source
// document: <mosaicRoot>/Catalog/CommunicationProtocol.md, and nowhere else.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog"
)

// TestProtocolLoader_DocumentOnlyAtOldLocation_IsNotLoaded verifies that a valid document
// placed only at Development/Designs/CommunicationProtocol.md is ignored: loading fails with
// ErrProtocolSourceMissing and the error names the new location.
func TestProtocolLoader_DocumentOnlyAtOldLocation_IsNotLoaded(t *testing.T) {
	root := makeTempMosaicRoot(t)
	mustMkdir(t, root, "Development", "Designs")
	mustWriteFile(t, root, "Development/Designs/CommunicationProtocol.md", []byte(wellFormedProtocolDoc))

	content, err := catalog.FileProtocolLoader{}.LoadProtocol(catalogRootOf(root))

	if err == nil {
		t.Fatal("LoadProtocol succeeded with the document only at the old location; want ErrProtocolSourceMissing")
	}
	if !errors.Is(err, catalog.ErrProtocolSourceMissing) {
		t.Errorf("error = %v; want an error wrapping catalog.ErrProtocolSourceMissing", err)
	}
	if !strings.Contains(err.Error(), "Catalog/CommunicationProtocol.md") {
		t.Errorf("error %q does not name the new location Catalog/CommunicationProtocol.md", err.Error())
	}
	if content.Version != "" || len(content.Blocks) != 0 {
		t.Errorf("content = %+v on a failed load; want the zero value", content)
	}
}

// TestProtocolLoader_DocumentAtNewLocation_IsLoaded verifies that a document at the literal
// location Catalog/CommunicationProtocol.md is the one the loader reads.
func TestProtocolLoader_DocumentAtNewLocation_IsLoaded(t *testing.T) {
	root := makeTempMosaicRoot(t)
	mustWriteFile(t, root, "Catalog/CommunicationProtocol.md", []byte(wellFormedProtocolDoc))

	content, err := catalog.FileProtocolLoader{}.LoadProtocol(catalogRootOf(root))

	if err != nil {
		t.Fatalf("LoadProtocol error for a document at Catalog/CommunicationProtocol.md: %v", err)
	}
	if content.Version != "1.9" {
		t.Errorf("ProtocolContent.Version = %q, want %q", content.Version, "1.9")
	}
}

// TestProtocolLoader_ErrorNamesLiteralNewLocation verifies that the missing-file error text
// contains the literal new path, independent of the exported path constant.
func TestProtocolLoader_ErrorNamesLiteralNewLocation(t *testing.T) {
	root := makeTempMosaicRoot(t)

	_, err := catalog.FileProtocolLoader{}.LoadProtocol(catalogRootOf(root))

	if err == nil {
		t.Fatal("LoadProtocol returned nil error for a missing protocol file")
	}
	if !strings.Contains(err.Error(), "Catalog/CommunicationProtocol.md") {
		t.Errorf("error %q does not name Catalog/CommunicationProtocol.md", err.Error())
	}
}
