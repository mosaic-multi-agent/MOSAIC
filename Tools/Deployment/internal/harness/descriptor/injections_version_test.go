package descriptor_test

// injections_version_test.go specifies that a descriptor still declaring the deprecated
// injections_version field loads without error and that the value has no effect, while unknown
// fields stay rejected.

import (
	"testing"

	"mosaic-deploy/internal/harness/descriptor"
)

const injectionsVersionBase = "schema_version: \"1\"\nid: \"legacy\"\ndisplay_name: \"Legacy\"\n"

func TestParse_DeclaredInjectionsVersion_LoadsWithoutEffect(t *testing.T) {
	cases := map[string]string{
		"quoted":   "injections_version: \"9.9.9\"\n",
		"unquoted": "injections_version: 2.0\n",
		"empty":    "injections_version: \"\"\n",
	}
	for name, field := range cases {
		t.Run(name, func(t *testing.T) {
			d, err := descriptor.Parse([]byte(injectionsVersionBase+field), "test:legacy")

			if err != nil {
				t.Fatalf("Parse: unexpected error for a descriptor declaring injections_version: %v", err)
			}
			if d.InjectionsVersion != "" || d.OrchestratorInjectionsVersion != "" {
				t.Errorf("versions = %q/%q, want both empty", d.InjectionsVersion, d.OrchestratorInjectionsVersion)
			}
		})
	}
}

func TestParse_UnknownField_StillRejected(t *testing.T) {
	_, err := descriptor.Parse([]byte(injectionsVersionBase+"injections_versionn: \"1.0.0\"\n"), "test:legacy")

	if err == nil {
		t.Fatal("Parse accepted an unknown field; strict decoding must stay in place for every other key")
	}
}
