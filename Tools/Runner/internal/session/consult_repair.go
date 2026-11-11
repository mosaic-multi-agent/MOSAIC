package session

import (
	"context"
	"strconv"
	"strings"

	"mosaic-run/internal/dispatchlog"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/repair"
)

// repairDamage runs the repair cycle for damage a routing consultation left in
// the artifact: the damaged copy is saved, then the orchestrator is asked to
// repair the file and the Runner verifies the result, at most
// repair.MaxConsultations times. It returns the accepted repair consultation's
// reply with ended=false, or the outcome that ends the run with ended=true.
// The artifact is never written here.
func (s *sessionImpl) repairDamage(
	ctx context.Context, req domain.ConsultationRequest, config domain.RunConfig, dc *damagedConsultation,
) (reply domain.RoutingInstruction, outcome domain.RunOutcome, ended bool) {
	s.logDamageFound(dc.damage)
	lastGood, _ := s.deps.Evidence.LastGoodCopy()
	occ := repair.NewOccurrence(repair.OccurrenceInput{
		Store:               s.deps.Store,
		ArtifactPath:        dc.damage.Resource,
		RunID:               config.RunID,
		Damage:              dc.damage,
		LastGoodCopy:        lastGood,
		DamagedCopy:         s.saveDamagedCopy(),
		ReportedDispatchLog: s.ownRunDispatchLog(config.RunID),
	})
	callErr := dc.cause
	for {
		if s.repairStopped(ctx, callErr) {
			return reply, s.unrepairedOutcome(occ, domain.UnrepairedStopped, callErr), true
		}
		if s.deps.Routing == nil {
			return reply, s.unrepairedOutcome(occ, domain.UnrepairedNoConsultant, nil), true
		}
		report, ok := occ.NextReport()
		if !ok {
			return reply, s.unrepairedOutcome(occ, domain.UnrepairedRejected, nil), true
		}
		repairReq := req
		repairReq.ArtifactDamage = &report
		repairReq.OfferAskOrchestrator = false
		s.deps.Debug.Log(domain.EventSessionRepairConsult, "asking the orchestrator to repair the artifact",
			domain.F("attempt", strconv.Itoa(occ.Attempt())),
			domain.F("last_good", report.LastGoodCopy),
			domain.F("dispatch_log", report.DispatchLog),
		)
		reply, callErr = s.consultOrchestratorAttempt(ctx, repairReq, occ.Attempt())
		if s.repairStopped(ctx, callErr) {
			return reply, s.unrepairedOutcome(occ, domain.UnrepairedStopped, callErr), true
		}
		if callErr != nil {
			return reply, s.unrepairedOutcome(occ, domain.UnrepairedConsultationFailed, callErr), true
		}
		verdict := occ.Judge()
		s.announceRepairVerdict(ctx, occ.Attempt(), verdict)
		if verdict.Accepted {
			s.deps.Debug.Log(domain.EventSessionRepairAccepted, "repair accepted",
				domain.F("attempt", strconv.Itoa(occ.Attempt())))
			return reply, domain.RunOutcome{}, false
		}
		s.deps.Debug.Log(domain.EventSessionRepairRejected, "repair not accepted; its reply is discarded",
			domain.F("attempt", strconv.Itoa(occ.Attempt())),
			domain.F("problems", strconv.Itoa(len(verdict.Problems))),
			domain.F("reasons", problemReasons(verdict.Problems)),
		)
	}
}

// repairStopped reports whether the run is being stopped: a force stop that
// ended the last consultation, a confirmed graceful stop or a cancelled context.
func (s *sessionImpl) repairStopped(ctx context.Context, lastErr error) bool {
	return isForceStopped(lastErr) || s.deps.StopRequested() || ctx.Err() != nil
}

// unrepairedOutcome ends the run with the artifact still damaged. A stop is a
// resumable stop; every other reason fails the run. The damage report and the
// copy paths are in the message and the cause.
func (s *sessionImpl) unrepairedOutcome(occ *repair.Occurrence, reason domain.UnrepairedReason, cause error) domain.RunOutcome {
	unrepaired := occ.Unrepaired(reason, cause)
	s.deps.Debug.Log(domain.EventSessionRepairStop, "run ends with the artifact still damaged",
		domain.F("reason", string(reason)),
		domain.F("consultations", strconv.Itoa(unrepaired.Consultations)),
	)
	status := domain.RunFailed
	if reason == domain.UnrepairedStopped {
		status = domain.RunStopped
	}
	return domain.RunOutcome{Status: status, Message: unrepaired.Error(), Cause: unrepaired}
}

func (s *sessionImpl) logDamageFound(damage *domain.ArtifactDamageError) {
	s.deps.Debug.Log(domain.EventSessionRepairDamageFound, "routing consultation left the artifact damaged",
		domain.F("when", "consultation"),
		domain.F("problems", strconv.Itoa(len(damage.Problems))),
		domain.F("first", damage.Problems[0].String()),
	)
}

// saveDamagedCopy saves the damaged artifact beside the last-good copy and
// returns its path; "" when the save failed or no evidence location exists.
func (s *sessionImpl) saveDamagedCopy() string {
	path, err := s.deps.Evidence.SaveDamagedCopy(s.deps.Clock.Now())
	if err != nil {
		s.deps.Debug.Log(domain.EventSessionRepairDamagedCopyFailed, "damaged copy not saved",
			domain.F("error", err.Error()))
		return ""
	}
	s.deps.Debug.Log(domain.EventSessionRepairDamagedCopySaved, "damaged copy saved", domain.F("path", path))
	return path
}

// ownRunDispatchLog returns the dispatch logger's path when the dispatch-log
// reader reports it as this run's own log; "" for anything else.
func (s *sessionImpl) ownRunDispatchLog(runID string) string {
	path := s.deps.DispatchLog.Path()
	if !dispatchlog.IsRunLog(path, runID) {
		return ""
	}
	return path
}

// announceRepairVerdict tells the consultation observer the verdict on a repair
// consultation when it can show it.
func (s *sessionImpl) announceRepairVerdict(ctx context.Context, attempt int, verdict repair.Verdict) {
	if obs, ok := s.deps.Consultations.(domain.RepairOutcomeObserver); ok {
		obs.RepairJudged(ctx, domain.RepairOutcome{Attempt: attempt, Accepted: verdict.Accepted, Problems: len(verdict.Problems)})
	}
}

func problemReasons(problems []domain.ArtifactDamageProblem) string {
	parts := make([]string, len(problems))
	for i, p := range problems {
		parts[i] = p.String()
	}
	return strings.Join(parts, "; ")
}
