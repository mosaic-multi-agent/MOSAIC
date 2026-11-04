package registry

import (
	"os"
	"path/filepath"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
	"mosaic-deploy/internal/harness/injectionfile"
)

// runtimeModule implements domain.HarnessModule for harnesses provided by an on-disk
// harness.yaml folder. The module's entire behaviour is driven by the descriptor through
// shared algorithms in harness/descriptor (CD-2):
//   - descriptor.MapTools handles tool mapping
//   - descriptor.ApplyFrontmatterSpec handles frontmatter shaping
//   - descriptor.ResolveTargetPath handles deployment paths
//
// This is the zero-code runtime harness tier (AC6.2): placing a folder with a valid
// harness.yaml under MosaicDeploy/harnesses/ makes it immediately available with no rebuild.
//
// The same struct is used as a placeholder for the external tier (TierExternal) until
// Stage 23 adds the JSON-over-stdio protocol. At that point, the external tier gains its
// own implementation that forwards calls across the process boundary.
type runtimeModule struct {
	ref             domain.HarnessRef
	desc            *domain.HarnessDescriptor
	content injectionfile.HarnessContent // HarnessInjections.md and HarnessInjectionsOrchestrator.md regions; either map may be empty
}

// newRuntimeModule constructs a descriptor-driven module for the given ref and descriptor.
// Both the descriptor-only and (for now) external tiers use this implementation.
// injections may be nil when no HarnessInjections.md is available (external tier, or absent file).
// orchInjections may be nil when no HarnessInjectionsOrchestrator.md is available.
func newRuntimeModule(ref domain.HarnessRef, desc *domain.HarnessDescriptor, injections map[string]string, orchInjections map[string]string) *runtimeModule {
	if injections == nil {
		injections = make(map[string]string)
	}
	if orchInjections == nil {
		orchInjections = make(map[string]string)
	}
	return &runtimeModule{ref: ref, desc: desc, content: injectionfile.HarnessContent{Shared: injections, Orchestrator: orchInjections}}
}

// tierInjections is the injection content read from the files next to a descriptor.
type tierInjections struct {
	shared         map[string]string
	sharedVersion  string
	orch           map[string]string
	orchVersion    string
}

// loadTierInjections reads HarnessInjections.md and HarnessInjectionsOrchestrator.md from the
// directory of the harness YAML file. A missing or unparseable file means no injections of that
// kind and no version. A file that parses but has no frontmatter version is an error naming the
// file path.
func loadTierInjections(yamlPath string) (tierInjections, error) {
	dir := filepath.Dir(yamlPath)
	shared, sharedVersion, err := loadTierFile(dir, injectionfile.SharedFileName)
	if err != nil {
		return tierInjections{}, err
	}
	orch, orchVersion, err := loadTierFile(dir, injectionfile.OrchestratorFileName)
	if err != nil {
		return tierInjections{}, err
	}
	return tierInjections{shared: shared, sharedVersion: sharedVersion, orch: orch, orchVersion: orchVersion}, nil
}

func loadTierFile(dir, name string) (map[string]string, string, error) {
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}, "", nil
	}
	regions, version, err := injectionfile.ParseDeployedRegionsWithVersion(data)
	if err != nil {
		return map[string]string{}, "", nil
	}
	if version == "" {
		return nil, "", injectionfile.NewMissingVersionError(name, path)
	}
	return regions, version, nil
}

// Ref returns the identity and provenance of this harness. The value is stable; calling Ref
// twice returns equal values.
func (m *runtimeModule) Ref() domain.HarnessRef {
	return m.ref
}

// Descriptor returns the parsed descriptor. The same pointer is returned on every call;
// callers must treat it as read-only.
func (m *runtimeModule) Descriptor() *domain.HarnessDescriptor {
	return m.desc
}

// Close releases resources held by this module. The descriptor-only tier holds no resources
// (no child processes, no open files), so this always returns nil.
func (m *runtimeModule) Close() error {
	return nil
}

// Tools maps an agent's generic tool list to harness-specific fields using the shared
// descriptor-driven algorithm. Every entry in req.Generic appears exactly once in the
// returned Resolutions, in the same order.
func (m *runtimeModule) Tools(req domain.ToolRequest) (domain.ToolResult, error) {
	return descriptor.MapTools(m.desc, req)
}

// Frontmatter returns the ordered field operations the harness applies to a generic agent's
// frontmatter. The plan is derived from the descriptor's frontmatter.add / frontmatter.drop /
// frontmatter.key_order declarations.
func (m *runtimeModule) Frontmatter(req domain.FrontmatterRequest) (domain.FrontmatterPlan, error) {
	return descriptor.ApplyFrontmatterSpec(m.desc, req)
}

// TargetPath returns the deployment path for one artifact, delegating to the shared
// descriptor path-resolution algorithm. Returns domain.ErrArtifactUnsupported when the
// descriptor declares no path for the requested artifact kind.
func (m *runtimeModule) TargetPath(req domain.TargetPathRequest) (string, error) {
	return descriptor.ResolveTargetPath(m.desc, req)
}

// Injection returns the content this harness serves for one region and role: the orchestrator
// region from HarnessInjectionsOrchestrator.md (orchestrator role only), every other region
// from HarnessInjections.md. The two files are never merged.
func (m *runtimeModule) Injection(req domain.InjectionRequest) (string, bool) {
	return m.content.Injection(req)
}

// HookPlan returns a plan for deploying one hook bundle with this harness. When the
// descriptor declares hooks.supported: false, the returned plan carries Supported: false
// and a non-empty Reason. Full hook variant file resolution for the descriptor-only tier
// is out of Stage 6's scope.
func (m *runtimeModule) HookPlan(req domain.HookPlanRequest) (domain.HookPlan, error) {
	if !m.desc.Hooks.Supported {
		return domain.HookPlan{
			Supported: false,
			Reason:    "harness descriptor declares hooks not supported",
		}, nil
	}
	// Descriptor-only harnesses do not ship hook variant files. Hook deployment for
	// runtime-provisioned harnesses is out of scope for this stage.
	return domain.HookPlan{
		Supported: false,
		Reason:    "hook variant files are not available for runtime-provisioned harnesses",
	}, nil
}

