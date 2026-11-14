package testjson_test

// decode_malformed_test.go: every way a file can fail to be a test-format agent is
// reported as ErrMalformedDeployed in the decode phase.

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/agentformat"
)

func TestDecode_MalformedFile_ReportsErrMalformedDeployed(t *testing.T) {
	cases := map[string]string{
		"empty file":                     ``,
		"plain markdown":                 "---\nname: x\n---\nbody\n",
		"invalid json":                   `{"mosaic_test_agent":1,`,
		"top-level array":                `[1]`,
		"top-level null":                 `null`,
		"top-level string":               `"text"`,
		"marker missing":                 `{"instructions":"b","meta":[]}`,
		"marker is 2":                    `{"mosaic_test_agent":2,"instructions":"b","meta":[]}`,
		"marker is a string":             `{"mosaic_test_agent":"1","instructions":"b","meta":[]}`,
		"instructions missing":           `{"mosaic_test_agent":1,"meta":[]}`,
		"instructions not a string":      `{"mosaic_test_agent":1,"instructions":5,"meta":[]}`,
		"meta missing":                   `{"mosaic_test_agent":1,"instructions":"b"}`,
		"meta not an array":              `{"mosaic_test_agent":1,"instructions":"b","meta":{}}`,
		"meta entry not an array":        `{"mosaic_test_agent":1,"instructions":"b","meta":["name"]}`,
		"meta entry with one element":    `{"mosaic_test_agent":1,"instructions":"b","meta":[["name"]]}`,
		"meta entry with three elements": `{"mosaic_test_agent":1,"instructions":"b","meta":[["name","a","b"]]}`,
		"meta key not a string":          `{"mosaic_test_agent":1,"instructions":"b","meta":[[1,"a"]]}`,
		"meta value is a number":         `{"mosaic_test_agent":1,"instructions":"b","meta":[["name",3]]}`,
		"meta value is a boolean":        `{"mosaic_test_agent":1,"instructions":"b","meta":[["name",true]]}`,
		"meta value is null":             `{"mosaic_test_agent":1,"instructions":"b","meta":[["name",null]]}`,
		"meta value is unknown object":   `{"mosaic_test_agent":1,"instructions":"b","meta":[["name",{"other":["a"]}]]}`,
		"list value is not an array":     `{"mosaic_test_agent":1,"instructions":"b","meta":[["name",{"list":"a"}]]}`,
		"list item is a number":          `{"mosaic_test_agent":1,"instructions":"b","meta":[["name",{"list":[1]}]]}`,
		"map value is not pairs":         `{"mosaic_test_agent":1,"instructions":"b","meta":[["name",{"map":["a"]}]]}`,
		"duplicate meta key":             `{"mosaic_test_agent":1,"instructions":"b","meta":[["name","a"],["name","b"]]}`,
		"native is an array":             `{"mosaic_test_agent":1,"instructions":"b","meta":[],"native":[]}`,
		"native is null":                 `{"mosaic_test_agent":1,"instructions":"b","meta":[],"native":null}`,
		"native is a string":             `{"mosaic_test_agent":1,"instructions":"b","meta":[],"native":"x"}`,
		"extra top-level member":         `{"mosaic_test_agent":1,"instructions":"b","meta":[],"extra":1}`,
		"trailing garbage after object":  `{"mosaic_test_agent":1,"instructions":"b","meta":[]} x`,
		"second object after the first":  `{"mosaic_test_agent":1,"instructions":"b","meta":[]}{"a":1}`,
	}
	for name, file := range cases {
		file := file
		t.Run(name, func(t *testing.T) {
			tr := translator(t)

			_, _, err := tr.Decode([]byte(file), decodeCtx())

			if err == nil {
				t.Fatalf("Decode returned nil error for a malformed file: %s", file)
			}
			if !errors.Is(err, agentformat.ErrMalformedDeployed) {
				t.Errorf("error %v does not wrap ErrMalformedDeployed", err)
			}
			var ae *agentformat.ArtifactError
			if !errors.As(err, &ae) || ae.Phase != "decode" {
				t.Errorf("error %v is not an ArtifactError in phase decode", err)
			}
		})
	}
}

func TestDecode_ValidEdgeFiles_AreAccepted(t *testing.T) {
	cases := map[string]string{
		"empty instructions and meta":    `{"mosaic_test_agent":1,"instructions":"","meta":[]}`,
		"empty native object":            `{"mosaic_test_agent":1,"instructions":"b","meta":[],"native":{}}`,
		"empty list and map values":      `{"mosaic_test_agent":1,"instructions":"b","meta":[["a",{"list":[]}],["b",{"map":[]}]]}`,
		"nested list in map":             `{"mosaic_test_agent":1,"instructions":"b","meta":[["a",{"map":[["k",{"list":["x"]}]]}]]}`,
	}
	for name, file := range cases {
		file := file
		t.Run(name, func(t *testing.T) {
			tr := translator(t)

			_, rep, err := tr.Decode([]byte(file), decodeCtx())

			if err != nil {
				t.Fatalf("Decode rejected a valid file: %v", err)
			}
			if len(rep.Entries) != 0 {
				t.Errorf("report entries for a valid file: %+v", rep.Entries)
			}
		})
	}
}
