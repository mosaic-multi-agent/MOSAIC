package digest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
)

var timeType = reflect.TypeOf(time.Time{})

// diffValue returns "" when a and b are equal, else a description of the first
// difference. Times compare with Equal; nil and empty slices are alike. It walks
// unexported fields (TokenCount) through their kinds only.
func diffValue(path string, a, b reflect.Value) string {
	if a.Type() == timeType {
		ta, tb := a.Interface().(time.Time), b.Interface().(time.Time)
		if !ta.Equal(tb) {
			return fmt.Sprintf("%s: %v != %v", path, ta, tb)
		}
		return ""
	}
	switch a.Kind() {
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if d := diffValue(path+"."+a.Type().Field(i).Name, a.Field(i), b.Field(i)); d != "" {
				return d
			}
		}
	case reflect.Slice:
		if a.Len() != b.Len() {
			return fmt.Sprintf("%s: length %d != %d", path, a.Len(), b.Len())
		}
		for i := 0; i < a.Len(); i++ {
			if d := diffValue(fmt.Sprintf("%s[%d]", path, i), a.Index(i), b.Index(i)); d != "" {
				return d
			}
		}
	case reflect.Ptr:
		if a.IsNil() != b.IsNil() {
			return fmt.Sprintf("%s: nil-ness differs (%v vs %v)", path, a.IsNil(), b.IsNil())
		}
		if !a.IsNil() {
			return diffValue(path+"*", a.Elem(), b.Elem())
		}
	case reflect.String:
		if a.String() != b.String() {
			return fmt.Sprintf("%s: %q != %q", path, a.String(), b.String())
		}
	case reflect.Int, reflect.Int64:
		if a.Int() != b.Int() {
			return fmt.Sprintf("%s: %d != %d", path, a.Int(), b.Int())
		}
	case reflect.Uint8:
		if a.Uint() != b.Uint() {
			return fmt.Sprintf("%s: %d != %d", path, a.Uint(), b.Uint())
		}
	case reflect.Bool:
		if a.Bool() != b.Bool() {
			return fmt.Sprintf("%s: %v != %v", path, a.Bool(), b.Bool())
		}
	default:
		panic("diffValue: unhandled kind " + a.Kind().String())
	}
	return ""
}

// requireSame fails the test when want and got differ.
func requireSame(t *testing.T, what string, want, got any) {
	t.Helper()
	if d := diffValue(what, reflect.ValueOf(want), reflect.ValueOf(got)); d != "" {
		t.Fatalf("%s", d)
	}
}

// stripFinding clears the fields a digest never carries.
func stripFinding(f domain.Finding) domain.Finding {
	f.Path, f.Line, f.Detail = "", 0, ""
	return f
}

// digestPath returns a fresh digest file path inside a temp directory.
func digestPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "out.digest.json")
}

// writeDigest writes d to a fresh path and returns the path.
func writeDigest(t *testing.T, d domain.Digest) string {
	t.Helper()
	path := digestPath(t)
	if err := digest.NewStore().Write(context.Background(), path, d, false); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return path
}

// readDigest reads path and requires a nil error.
func readDigest(t *testing.T, path string) (domain.Digest, []domain.Finding) {
	t.Helper()
	d, findings, err := digest.NewStore().Read(context.Background(), path)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	return d, findings
}

// roundTrip writes d and reads it back.
func roundTrip(t *testing.T, d domain.Digest) (domain.Digest, []domain.Finding) {
	t.Helper()
	return readDigest(t, writeDigest(t, d))
}

// readJSONObject parses a file as a generic JSON object.
func readJSONObject(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("file is not a JSON object: %v", err)
	}
	return obj
}

// validDoc returns a minimal valid digest document at the given version.
func validDoc(version string) map[string]any {
	return map[string]any{
		"format":         "mosaic-run-digest",
		"digest_version": version,
		"runs":           []any{},
	}
}

// writeRawDoc marshals doc into a fresh digest path.
func writeRawDoc(t *testing.T, doc any) string {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}
	return writeRawBytes(t, raw)
}

// writeRawBytes writes raw bytes into a fresh digest path.
func writeRawBytes(t *testing.T, raw []byte) string {
	t.Helper()
	path := digestPath(t)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	return path
}

// requireAtLeast stops the test cleanly when a decoded slice is too short to
// index, so a missing behavior fails instead of panicking.
func requireAtLeast(t *testing.T, what string, have, want int) {
	t.Helper()
	if have < want {
		t.Fatalf("%s: have %d entries, need at least %d", what, have, want)
	}
}
