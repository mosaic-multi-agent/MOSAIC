package registry_test

// injection_version_tiers_test.go specifies where the descriptor-only and external tiers get
// their injection versions: only from the frontmatter version of the HarnessInjections.md and
// HarnessInjectionsOrchestrator.md next to the harness descriptor. A missing file means no
// injections and no version; a file without a frontmatter version makes that harness unusable
// with a reason naming the file; a descriptor-declared injections_version is ignored.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

const tierNoVersionDoc = "---\nharness: x\n---\n<HarnessConstraints type=\"managed\">\nbody\n</HarnessConstraints>\n"

func tierVersionedDoc(version string) string {
	return "---\nversion: \"" + version + "\"\n---\n<HarnessConstraints type=\"managed\">\nbody\n</HarnessConstraints>\n"
}

func tierYAML(id, extra string) string {
	return fmt.Sprintf("schema_version: \"1\"\nid: %q\ndisplay_name: %q\n%s", id, id, extra)
}

// writeTierFiles writes the given injection files (nil leaves the file out) into the harness
// folder of id under root.
func writeTierFiles(t *testing.T, root, id string, shared, orch *string) string {
	t.Helper()
	dir := filepath.Join(root, "MosaicDeploy", "harnesses", id)
	for name, content := range map[string]*string{
		"HarnessInjections.md":             shared,
		"HarnessInjectionsOrchestrator.md": orch,
	} {
		if content == nil {
			continue
		}
		must(t, os.WriteFile(filepath.Join(dir, name), []byte(*content), 0o644))
	}
	return dir
}

func tierStr(s string) *string { return &s }

// tierHarness is one way to build a harness folder of a tier and discover it.
type tierHarness struct {
	name     string
	setup    func(t *testing.T, id, yamlExtra string) string // returns the MOSAIC root
	discover func(t *testing.T, root string) registry.Registry
}

func tierHarnesses() []tierHarness {
	discover := func(allowExternal bool) func(*testing.T, string) registry.Registry {
		return func(t *testing.T, root string) registry.Registry {
			t.Helper()
			reg, err := registry.Discover(registry.Options{MosaicRoot: root, AllowExternal: allowExternal})
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			return reg
		}
	}
	return []tierHarness{
		{"descriptor-only", func(t *testing.T, id, extra string) string {
			return makeDescriptorRoot(t, id, tierYAML(id, extra))
		}, discover(false)},
		{"external", func(t *testing.T, id, extra string) string {
			root := makeRootWithOpenCodeContent(t)
			addProtocolBinary(t, root, id, id, buildProtocolBinaryForRegistry(t))
			dir := filepath.Join(root, "MosaicDeploy", "harnesses", id)
			must(t, os.WriteFile(filepath.Join(dir, "harness.yaml"), []byte(tierYAML(id, extra)), 0o644))
			return root
		}, discover(true)},
	}
}

func resolveTier(t *testing.T, reg registry.Registry, id string) domain.HarnessModule {
	t.Helper()
	m, err := reg.Resolve(id)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", id, err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func TestTierInjectionVersions_ReadFromFilesNextToDescriptor(t *testing.T) {
	for _, tier := range tierHarnesses() {
		t.Run(tier.name, func(t *testing.T) {
			id := uniqueID(t, "ver")
			root := tier.setup(t, id, "")
			writeTierFiles(t, root, id, tierStr(tierVersionedDoc("3.1.0")), tierStr(tierVersionedDoc("4.2.0")))

			desc := resolveTier(t, tier.discover(t, root), id).Descriptor()

			if desc.InjectionsVersion != "3.1.0" {
				t.Errorf("InjectionsVersion = %q, want %q", desc.InjectionsVersion, "3.1.0")
			}
			if desc.OrchestratorInjectionsVersion != "4.2.0" {
				t.Errorf("OrchestratorInjectionsVersion = %q, want %q", desc.OrchestratorInjectionsVersion, "4.2.0")
			}
		})
	}
}

func TestTierInjectionVersions_EditingTheFileChangesTheVersionWithoutRebuild(t *testing.T) {
	for _, tier := range tierHarnesses() {
		t.Run(tier.name, func(t *testing.T) {
			id := uniqueID(t, "edit")
			root := tier.setup(t, id, "")
			writeTierFiles(t, root, id, tierStr(tierVersionedDoc("1.0.0")), nil)
			before := resolveTier(t, tier.discover(t, root), id).Descriptor().InjectionsVersion

			writeTierFiles(t, root, id, tierStr(tierVersionedDoc("1.1.0")), nil)
			after := resolveTier(t, tier.discover(t, root), id).Descriptor().InjectionsVersion

			if before != "1.0.0" || after != "1.1.0" {
				t.Errorf("versions before/after edit = %q/%q, want 1.0.0/1.1.0", before, after)
			}
		})
	}
}

func TestTierInjectionVersions_DescriptorDeclaredInjectionsVersionIsIgnored(t *testing.T) {
	for _, tier := range tierHarnesses() {
		t.Run(tier.name+"/file present", func(t *testing.T) {
			id := uniqueID(t, "ignored")
			root := tier.setup(t, id, "injections_version: \"9.9.9\"\n")
			writeTierFiles(t, root, id, tierStr(tierVersionedDoc("3.1.0")), nil)

			got := resolveTier(t, tier.discover(t, root), id).Descriptor().InjectionsVersion

			if got != "3.1.0" {
				t.Errorf("InjectionsVersion = %q, want the file version %q", got, "3.1.0")
			}
		})
		t.Run(tier.name+"/file absent", func(t *testing.T) {
			id := uniqueID(t, "ignored-absent")
			root := tier.setup(t, id, "injections_version: \"9.9.9\"\n")

			got := resolveTier(t, tier.discover(t, root), id).Descriptor().InjectionsVersion

			if got != "" {
				t.Errorf("InjectionsVersion = %q, want empty: no file means no version", got)
			}
		})
	}
}

func TestTierInjectionVersions_MissingFilesMeanNoVersion(t *testing.T) {
	cases := []struct {
		name         string
		shared, orch *string
		wantShared   string
		wantOrch     string
	}{
		{"both absent", nil, nil, "", ""},
		{"orchestrator file absent", tierStr(tierVersionedDoc("3.1.0")), nil, "3.1.0", ""},
		{"shared file absent", nil, tierStr(tierVersionedDoc("4.2.0")), "", "4.2.0"},
	}
	for _, tier := range tierHarnesses() {
		for _, tc := range cases {
			t.Run(tier.name+"/"+tc.name, func(t *testing.T) {
				id := uniqueID(t, "missing")
				root := tier.setup(t, id, "")
				writeTierFiles(t, root, id, tc.shared, tc.orch)
				reg := tier.discover(t, root)

				ref, ok := findRef(reg.List(), id)
				if !ok || !ref.Usable {
					t.Fatalf("harness %q must stay usable when an injection file is absent (ref=%+v, found=%v)", id, ref, ok)
				}
				desc := resolveTier(t, reg, id).Descriptor()

				if desc.InjectionsVersion != tc.wantShared || desc.OrchestratorInjectionsVersion != tc.wantOrch {
					t.Errorf("versions = %q/%q, want %q/%q", desc.InjectionsVersion, desc.OrchestratorInjectionsVersion, tc.wantShared, tc.wantOrch)
				}
			})
		}
	}
}

func TestTierInjectionVersions_FileWithoutVersion_HarnessUnusableNamingTheFile(t *testing.T) {
	cases := []struct {
		name         string
		shared, orch *string
		wantFile     string
	}{
		{"shared without version", tierStr(tierNoVersionDoc), tierStr(tierVersionedDoc("1.0.0")), "HarnessInjections.md"},
		{"orchestrator without version", tierStr(tierVersionedDoc("1.0.0")), tierStr(tierNoVersionDoc), "HarnessInjectionsOrchestrator.md"},
		{"orchestrator without version, shared absent", nil, tierStr(tierNoVersionDoc), "HarnessInjectionsOrchestrator.md"},
	}
	for _, tier := range tierHarnesses() {
		for _, tc := range cases {
			t.Run(tier.name+"/"+tc.name, func(t *testing.T) {
				id := uniqueID(t, "noversion")
				root := tier.setup(t, id, "")
				dir := writeTierFiles(t, root, id, tc.shared, tc.orch)
				reg := tier.discover(t, root)

				ref, ok := findRef(reg.List(), id)

				if !ok {
					t.Fatalf("harness %q missing from List(); it must stay listed", id)
				}
				if ref.Usable {
					t.Error("Usable = true, want false for an injection file without a frontmatter version")
				}
				if want := filepath.Join(dir, tc.wantFile); !strings.Contains(ref.UnusableReason, want) {
					t.Errorf("UnusableReason = %q, want it to name %q", ref.UnusableReason, want)
				}
				if m, err := reg.Resolve(id); err == nil {
					_ = m.Close() // release the module process so the temp dir can be removed
					t.Error("Resolve returned nil error for an unusable harness")
				}
			})
		}
	}
}

func TestTierInjectionVersions_UnparseableFileStaysTreatedAsAbsent(t *testing.T) {
	const unparseable = "---\nversion: \"1.0.0\"\n---\n<HarnessConstraints type=\"managed\">\nnever closed\n"
	id := uniqueID(t, "unparseable")
	root := makeDescriptorRoot(t, id, tierYAML(id, ""))
	writeTierFiles(t, root, id, tierStr(tierVersionedDoc("3.1.0")), tierStr(unparseable))

	reg, err := registry.Discover(registry.Options{MosaicRoot: root})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	desc := resolveTier(t, reg, id).Descriptor()

	if desc.InjectionsVersion != "3.1.0" || desc.OrchestratorInjectionsVersion != "" {
		t.Errorf("versions = %q/%q, want 3.1.0/empty (failed file treated as absent)", desc.InjectionsVersion, desc.OrchestratorInjectionsVersion)
	}
}

// Without the external opt-in the injection files are not read, so a file without a version
// does not change the reason the harness is unusable.
func TestTierInjectionVersions_ExternalWithoutOptIn_ReasonIsOptInWhateverTheFilesHold(t *testing.T) {
	id := uniqueID(t, "noptin")
	root := makeRootWithOpenCodeContent(t)
	addProtocolBinary(t, root, id, id, buildProtocolBinaryForRegistry(t))
	writeTierFiles(t, root, id, tierStr(tierNoVersionDoc), nil)

	reg, err := registry.Discover(registry.Options{MosaicRoot: root, AllowExternal: false})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	ref, ok := findRef(reg.List(), id)

	if !ok || ref.Usable || !ref.RequiresOptIn || strings.Contains(ref.UnusableReason, "HarnessInjections.md") {
		t.Errorf("ref = %+v (found=%v), want unusable with the opt-in reason only", ref, ok)
	}
}
