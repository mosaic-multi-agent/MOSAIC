package app

// save_offer.go offers to remember a custom tool answer as a tool_destinations entry of the run's
// harness in user-config.yaml, so a later run on that harness does not ask for the tool again.

import (
	"context"
	"fmt"

	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/customtool"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

// saveOffer makes the save offers of one run for one harness and, when an entry was saved,
// refreshes the service's registry once the run's questions are over.
type saveOffer struct {
	s         *service
	harnessID string
	module    domain.HarnessModule
	saved     bool
}

// newSaveOffer returns the offers of a run on harnessID, whose resolved module is module.
func (s *service) newSaveOffer(harnessID string, module domain.HarnessModule) *saveOffer {
	return &saveOffer{s: s, harnessID: harnessID, module: module}
}

// applies reports whether a saved entry can stop the question on this harness: the module is a
// built-in or descriptor harness whose descriptor declares MCP name forms. External modules map
// tools through their own protocol, so a configuration entry never reaches them.
func (o *saveOffer) applies() bool {
	if o.s.deps.UserConfig == nil || o.module == nil {
		return false
	}
	if tier := o.module.Ref().Tier; tier != domain.TierBuiltin && tier != domain.TierDescriptor {
		return false
	}
	desc := o.module.Descriptor()
	return desc != nil && (desc.Tools.MCP.ServerForm != "" || desc.Tools.MCP.ToolForm != "")
}

// mapping returns the tool_destinations entry that reproduces the answer for generic on the
// module, as the module resolves it for the given agent.
func (o *saveOffer) mapping(generic, agentKey, placeholder string, entries []domain.MCPEntry) (domain.ToolMapping, error) {
	result, err := o.module.Tools(domain.ToolRequest{
		AgentKey: agentKey, Generic: []string{generic}, Placeholder: placeholder,
		CustomNames: domain.CustomToolAnswers{generic: entries},
	})
	if err != nil {
		return domain.ToolMapping{}, err
	}
	for _, res := range result.Resolutions {
		if res.Generic == generic {
			return config.CustomToolMapping(res)
		}
	}
	return domain.ToolMapping{}, fmt.Errorf("the harness did not resolve tool %q", generic)
}

// offer asks whether to remember the answer for generic and saves it when accepted. Anything but
// an accepted offer saves nothing and changes nothing else; a failed save is a warning.
func (o *saveOffer) offer(ctx context.Context, generic, agentKey, placeholder string, entries []domain.MCPEntry) {
	if len(entries) == 0 || !o.applies() {
		return
	}
	mapping, err := o.mapping(generic, agentKey, placeholder, entries)
	if err != nil {
		return
	}
	path := o.s.deps.UserConfig.Path()
	ans, err := o.s.deps.Interaction.Confirm(ctx, domain.Question{
		ID: domain.QSaveCustomTool, Subject: generic,
		Title: "Remember the tool answer for " + generic + "?",
		Prompt: fmt.Sprintf("Save it to tool_destinations for %s, so later runs on this harness do not ask for %s again.",
			o.harnessID, generic),
		Detail:    fmt.Sprintf("%s = %s\nSaved to %s", generic, customtool.Text(entries), path),
		AllowSkip: true,
	})
	if err != nil || ans.Status != domain.Answered || !ans.Confirm {
		return
	}
	if err := config.SaveToolDestination(o.s.deps.UserConfig, o.harnessID, mapping); err != nil {
		o.s.deps.Interaction.Notify(ctx, domain.Notice{
			Level: domain.NoticeWarning,
			Title: "Tool answer was not saved",
			Message: fmt.Sprintf("%s could not be updated for tool %s: %v. The run continues; "+
				"you will be asked for this tool again next time.", path, generic, err),
		})
		return
	}
	o.saved = true
	o.s.deps.Interaction.Notify(ctx, domain.Notice{
		Level: domain.NoticeInfo,
		Title: "Tool answer saved",
		Message: fmt.Sprintf("Remembered %s for %s in %s. Later runs on this harness will not ask for it.",
			generic, o.harnessID, path),
	})
}

// finish makes later runs on this service see the entries saved by this run: the registry is
// built again from the saved configuration. The current run keeps the modules it resolved. A
// registry that cannot be rebuilt is kept; a failed rebuild is a warning and keeps the old one.
func (o *saveOffer) finish(ctx context.Context) {
	if !o.saved {
		return
	}
	rebuilder, ok := o.s.deps.Registry.(registry.Rebuilder)
	if !ok {
		return
	}
	rebuilt, err := rebuilder.Rebuild()
	if err != nil {
		o.s.deps.Interaction.Notify(ctx, domain.Notice{
			Level: domain.NoticeWarning,
			Title: "Saved tool answer not applied yet",
			Message: fmt.Sprintf("The harness registry could not be rebuilt: %v. The answer is saved; "+
				"it takes effect after Deploy restarts.", err),
		})
		return
	}
	o.s.deps.Registry = rebuilt
}
