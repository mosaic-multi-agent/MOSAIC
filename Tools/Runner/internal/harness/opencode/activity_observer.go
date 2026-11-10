package opencode

import (
	"context"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

// WithActivityObserver sets the observer for every Invoke and InvokeRaw.
// nil selects domain.NopActivityObserver. Returns a. Call before the first
// invocation.
func (a *OpenCodeAdapter) WithActivityObserver(o domain.ActivityObserver) *OpenCodeAdapter {
	a.activityObs = o
	return a
}

// Invoke implements domain.HarnessAdapter. The invocation is reported to the
// activity observer while the process runs; see invoke for the behaviour.
func (a *OpenCodeAdapter) Invoke(ctx context.Context, agent domain.AgentReference, request domain.ProtocolRequest) (domain.ProtocolResponse, error) {
	onLine, end := activity.Begin(a.activityObs, domain.ActivityInvocation{
		Source:  domain.ActivitySourceDispatch,
		Agent:   request.AgentInstanceID,
		Harness: commonharness.HarnessIDOpenCode,
	}, activity.NewOpenCodeDecoder())
	defer end()
	return a.invoke(ctx, agent, request, onLine)
}

// InvokeRaw implements domain.RawInvoker. The invocation is reported to the
// activity observer while the process runs; see invokeRaw for the behaviour.
func (a *OpenCodeAdapter) InvokeRaw(ctx context.Context, agent domain.AgentReference, payload []byte) ([]byte, error) {
	onLine, end := activity.Begin(a.activityObs, domain.ActivityInvocation{
		Source:  domain.ActivitySourceConsultation,
		Agent:   agent.Identifier,
		Harness: commonharness.HarnessIDOpenCode,
	}, activity.NewOpenCodeDecoder())
	defer end()
	return a.invokeRaw(ctx, agent, payload, onLine)
}
