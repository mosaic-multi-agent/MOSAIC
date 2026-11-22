package opencodev2

import (
	"context"
	"errors"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

// WithActivityObserver sets the observer for every Invoke and InvokeRaw.
// nil selects domain.NopActivityObserver. Returns a.
func (a *OpenCodeV2Adapter) WithActivityObserver(o domain.ActivityObserver) *OpenCodeV2Adapter {
	a.activityObs = o
	return a
}

// Invoke implements domain.HarnessAdapter. The invocation is reported to the
// activity observer while the process runs.
func (a *OpenCodeV2Adapter) Invoke(ctx context.Context, agent domain.AgentReference, request domain.ProtocolRequest) (domain.ProtocolResponse, error) {
	onLine, report, end := activity.BeginReporting(a.activityObs, domain.ActivityInvocation{
		Source:  domain.ActivitySourceDispatch,
		Agent:   request.AgentInstanceID,
		Harness: harnessID,
	}, activity.NewOpenCodeV2Decoder())
	defer end()
	resp, err := a.invoke(ctx, agent, request, onLine)
	a.reportWatchdog(report, err, a.invokeNoOutput(request.HumanInTheLoop))
	return resp, err
}

// InvokeRaw implements domain.RawInvoker. The invocation is reported to the
// activity observer while the process runs.
func (a *OpenCodeV2Adapter) InvokeRaw(ctx context.Context, agent domain.AgentReference, payload []byte) ([]byte, error) {
	onLine, report, end := activity.BeginReporting(a.activityObs, domain.ActivityInvocation{
		Source:  domain.ActivitySourceConsultation,
		Agent:   agent.Identifier,
		Harness: harnessID,
	}, activity.NewOpenCodeV2Decoder())
	defer end()
	text, err := a.invokeRaw(ctx, agent, payload, onLine)
	a.reportWatchdog(report, err, a.noOutputTimeout)
	return text, err
}

// reportWatchdog reports one activity item when err is a no-output watchdog
// end, so the progress screen names the reason before the invocation ends.
func (a *OpenCodeV2Adapter) reportWatchdog(report func(domain.ActivityItem), err error, d time.Duration) {
	if !errors.Is(err, commonharness.ErrNoOutput) {
		return
	}
	report(domain.ActivityItem{
		Kind:    domain.ActivityText,
		Summary: "no output for " + d.String() + "; ended by the no-output watchdog",
	})
}
