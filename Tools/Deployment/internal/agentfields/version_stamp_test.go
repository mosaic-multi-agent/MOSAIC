package agentfields

import (
	"reflect"
	"strings"
	"testing"
)

func mapLookup(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestVersionStampKeys_ListsNewStampThenOldStampPrefixedFirst(t *testing.T) {
	want := []string{
		"mosaic_harness_version", "harness_version",
		"mosaic_transform_version", "transform_version",
	}

	got := VersionStampKeys()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("VersionStampKeys() = %v, want %v", got, want)
	}
}

func TestVersionStampKeys_ReturnsIndependentSlice(t *testing.T) {
	first := VersionStampKeys()
	if len(first) == 0 {
		t.Fatal("VersionStampKeys() returned no keys")
	}
	first[0] = "mutated"

	second := VersionStampKeys()

	if len(second) == 0 || second[0] == "mutated" {
		t.Errorf("mutating the returned slice leaked into later calls: %v", second)
	}
}

func TestReadVersionStamp_EachKeyAlone_IsRecognised(t *testing.T) {
	for _, key := range []string{
		"mosaic_harness_version", "harness_version",
		"mosaic_transform_version", "transform_version",
	} {
		t.Run(key, func(t *testing.T) {
			stamp, ok := ReadVersionStamp(mapLookup(map[string]string{key: "2.1.0"}))

			if !ok || stamp != "2.1.0" {
				t.Errorf("ReadVersionStamp with only %q = (%q, %v), want (\"2.1.0\", true)", key, stamp, ok)
			}
		})
	}
}

func TestReadVersionStamp_NoStamp_IsNotRecognised(t *testing.T) {
	cases := map[string]map[string]string{
		"absent":           {"mosaic_id": "3", "version": "1.0.0"},
		"all empty values": {"mosaic_harness_version": "", "harness_version": "", "mosaic_transform_version": "", "transform_version": ""},
	}
	for name, m := range cases {
		t.Run(name, func(t *testing.T) {
			stamp, ok := ReadVersionStamp(mapLookup(m))

			if ok || stamp != "" {
				t.Errorf("ReadVersionStamp = (%q, %v), want (\"\", false)", stamp, ok)
			}
		})
	}
}

func TestReadVersionStamp_NilLookup_IsNotRecognised(t *testing.T) {
	stamp, ok := ReadVersionStamp(nil)

	if ok || stamp != "" {
		t.Errorf("ReadVersionStamp(nil) = (%q, %v), want (\"\", false)", stamp, ok)
	}
}

func TestReadVersionStamp_OldAndNewTogether_NewWins(t *testing.T) {
	stamp, ok := ReadVersionStamp(mapLookup(map[string]string{
		"mosaic_transform_version": "1.0.0",
		"mosaic_harness_version":   "3.0.0",
	}))

	if !ok || stamp != "3.0.0" {
		t.Errorf("ReadVersionStamp = (%q, %v), want (\"3.0.0\", true)", stamp, ok)
	}
}

func TestReadVersionStamp_PrefixedAndLegacyOfOneStamp_PrefixedWins(t *testing.T) {
	stamp, ok := ReadVersionStamp(mapLookup(map[string]string{
		"harness_version":        "legacy",
		"mosaic_harness_version": "prefixed",
	}))

	if !ok || stamp != "prefixed" {
		t.Errorf("ReadVersionStamp = (%q, %v), want (\"prefixed\", true)", stamp, ok)
	}
}

func TestReadVersionStamp_EmptyNewStampWithOldStamp_FallsBackToOld(t *testing.T) {
	stamp, ok := ReadVersionStamp(mapLookup(map[string]string{
		"mosaic_harness_version": "",
		"transform_version":      "2.1.0",
	}))

	if !ok || stamp != "2.1.0" {
		t.Errorf("ReadVersionStamp = (%q, %v), want (\"2.1.0\", true)", stamp, ok)
	}
}

func TestReadVersionStamp_QueriesOnlyStampKeys(t *testing.T) {
	allowed := map[string]bool{}
	for _, k := range VersionStampKeys() {
		allowed[k] = true
	}
	var unexpected []string

	ReadVersionStamp(func(k string) string {
		if !allowed[k] {
			unexpected = append(unexpected, k)
		}
		return ""
	})

	if len(unexpected) > 0 {
		t.Errorf("ReadVersionStamp queried non-stamp keys: %s", strings.Join(unexpected, ", "))
	}
}
