// Package opencodev2 implements the OpenCode V2 built-in harness module. It embeds the
// shared opencode-v2 descriptor and applies the descriptor-driven transformation logic.
//
// Agents deploy to .opencode/agents/<id>.md with an explicit role-based mode and the
// V2-native permissions rendering. Skills deploy to .opencode/skills/<id>/SKILL.md.
// Hooks deploy as plugins into .opencode/plugins/ from the bundle variant "opencode-v2";
// a bundle without that variant yields a supported plan with no files. The module never
// writes harness settings or config files.
//
// Registration: init() calls registry.Register so the module is available whenever
// this package is imported.
package opencodev2

import (
	"fmt"
	"path"

	"mosaic-common/harness/builtindesc"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
	"mosaic-deploy/internal/harness/injectionfile"
	"mosaic-deploy/internal/harness/registry"
)

// Descriptor bytes come from the shared Common package, not a local copy.
var embeddedDescriptor, _ = builtindesc.Raw("opencode-v2")

func init() {
	registry.Register("opencode-v2", func(opts registry.BuiltinOptions) (domain.HarnessModule, error) {
		return New(opts)
	})
}

// module is the OpenCode V2 built-in HarnessModule implementation.
type module struct {
	ref            domain.HarnessRef
	desc           *domain.HarnessDescriptor
	injections     map[string]string
	orchInjections map[string]string
}

// New parses the embedded descriptor and reads HarnessInjections.md and
// HarnessInjectionsOrchestrator.md from the harness content directory at construction time.
func New(opts registry.BuiltinOptions) (domain.HarnessModule, error) {
	desc, err := descriptor.Parse(embeddedDescriptor, "builtin:opencode-v2")
	if err != nil {
		return nil, fmt.Errorf("parse embedded opencode-v2 descriptor: %w", err)
	}
	contentDir, err := opts.ResolveContentDir(catalogpaths.HarnessDirNameOpenCodeV2)
	if err != nil {
		return nil, fmt.Errorf("opencode-v2: %w", err)
	}
	content, err := injectionfile.LoadDir(contentDir)
	if err != nil {
		return nil, fmt.Errorf("opencode-v2: load harness content: %w", err)
	}
	desc.InjectionsVersion = content.SharedVersion
	desc.OrchestratorInjectionsVersion = content.OrchestratorVersion
	ref := domain.HarnessRef{
		ID:                 desc.ID,
		DisplayName:        desc.DisplayName,
		Tier:               domain.TierBuiltin,
		Usable:             true,
		ExperimentalReason: desc.ExperimentalReason,
	}
	return &module{ref: ref, desc: desc, injections: content.Shared, orchInjections: content.Orchestrator}, nil
}

// Ref returns identity and provenance.
func (m *module) Ref() domain.HarnessRef { return m.ref }

// Descriptor returns the parsed descriptor; callers must treat it as read-only.
func (m *module) Descriptor() *domain.HarnessDescriptor { return m.desc }

// Close releases resources; the module holds none.
func (m *module) Close() error { return nil }

// Tools maps generic tool names to the descriptor's permissions rendering.
func (m *module) Tools(req domain.ToolRequest) (domain.ToolResult, error) {
	return descriptor.MapTools(m.desc, req)
}

// Frontmatter builds the plan from the descriptor: role-based mode, dropped keys and key order.
// Model and version stamps are applied by the transform pipeline, not here.
func (m *module) Frontmatter(req domain.FrontmatterRequest) (domain.FrontmatterPlan, error) {
	resolved := descriptor.ResolveRoleConditionalFields(m.desc.Frontmatter.RoleConditionalAdd, req.Role)
	set := make([]domain.FrontmatterField, 0, len(m.desc.Frontmatter.Add)+len(resolved))
	set = append(set, m.desc.Frontmatter.Add...)
	set = append(set, resolved...)
	return domain.FrontmatterPlan{
		Set:      set,
		Remove:   m.desc.Frontmatter.Drop,
		KeyOrder: m.desc.Frontmatter.KeyOrder,
	}, nil
}

// TargetPath returns the deployment path for one artifact. Skills get a key-named
// subdirectory (<skills-dir>/<key>/SKILL.md); everything else follows the descriptor.
func (m *module) TargetPath(req domain.TargetPathRequest) (string, error) {
	if req.Kind != domain.ArtifactSkill {
		return descriptor.ResolveTargetPath(m.desc, req)
	}
	if req.Scope != domain.ScopeProject {
		return "", fmt.Errorf("%w: scope %q is not supported; only %q is accepted", domain.ErrUnsupportedScope, req.Scope, domain.ScopeProject)
	}
	sp := m.desc.Paths.Skills
	if !sp.Supported {
		return "", fmt.Errorf("%w: skill", domain.ErrArtifactUnsupported)
	}
	filename := req.FileName
	if filename == "" {
		filename = req.Key
	}
	return path.Join(sp.Project, req.Key, filename), nil
}

// Injection returns the content this harness serves for one region and role.
func (m *module) Injection(req domain.InjectionRequest) (string, bool) {
	return injectionfile.HarnessContent{Shared: m.injections, Orchestrator: m.orchInjections}.Injection(req)
}

// HookPlan resolves a hook bundle: plugin files from the bundle's opencode-v2 variant go to
// the plugins directory; no registration is needed. A bundle without the variant yields a
// supported plan with no files.
func (m *module) HookPlan(req domain.HookPlanRequest) (domain.HookPlan, error) {
	if !m.desc.Hooks.Supported {
		return domain.HookPlan{Supported: false, Reason: "harness descriptor declares hooks not supported"}, nil
	}
	variantKey := m.desc.Hooks.VariantKey
	if variantKey == "" {
		variantKey = m.desc.ID
	}
	targetDir := m.desc.Paths.Hooks.Project
	variant, ok := req.Bundle.Variants[variantKey]
	if !ok {
		return domain.HookPlan{Supported: true, TargetDir: targetDir}, nil
	}
	return domain.HookPlan{
		Supported:    true,
		TargetDir:    targetDir,
		Files:        variant.Files,
		Registration: variant.Registration,
	}, nil
}
