package app

// promote_batch.go holds the PromoteBatch entry point and the state of one batch run. The run
// is sequenced as: discover and select the agents, load them, ask the once-per-batch questions
// (promote_batch_once.go), then process each agent (promote_batch_agent.go). Nothing is written
// before the once-per-batch questions are answered.

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"mosaic-deploy/internal/domain"
)

// batchEntry is one selected agent of a run.
type batchEntry struct {
	// agent is the selected file.
	agent promotableAgent
	// src is the loaded source; nil when the entry settled before it could be loaded.
	src *promoteSource
	// settled reports that the outcome is already known and the agent is not processed further.
	settled bool
}

// batchLayer is the layer that receives the batch.
type batchLayer struct {
	root      string
	catalogID string
	label     string
}

// batchToolAnswer is the answer to the question about one unmapped harness tool name.
type batchToolAnswer struct {
	// generic is the generic tool name the user gave; empty keeps the harness name verbatim.
	generic string
}

// promoteBatchRun is the state of one PromoteBatch call.
type promoteBatchRun struct {
	s       *service
	ctx     context.Context
	desc    *domain.HarnessDescriptor
	result  PromoteBatchResult
	entries []*batchEntry
	layer   batchLayer

	// tools holds the answer per unmapped harness tool name asked so far; toolsSkipped latches
	// a skip-all.
	tools        map[string]batchToolAnswer
	toolsSkipped bool
	// tiers holds the tier answer per tier subject (the source model, or the agent key for an
	// agent without a model); tiersSkipped latches a skip-all.
	tiers        map[string]string
	tiersSkipped bool
	// rationaleSkipped and skillsSkipped latch a skip for the rest of the batch.
	rationaleSkipped bool
	skillsSkipped    bool

	// nextID is the numeric id the next new agent receives.
	nextID int
}

// PromoteBatch implements Service.PromoteBatch.
func (s *service) PromoteBatch(ctx context.Context, req PromoteBatchRequest) (PromoteBatchResult, error) {
	if req.HarnessID == "" {
		return PromoteBatchResult{}, ErrPromoteHarnessRequired
	}
	module, err := s.deps.Registry.Resolve(req.HarnessID)
	if err != nil {
		return PromoteBatchResult{}, fmt.Errorf("%w: %w", ErrPromoteHarnessUnresolvable, err)
	}
	desc := module.Descriptor()
	_ = module.Close() // only the descriptor is needed; each agent resolves its own module
	if err := checkPromoteAgentsDir(req.AgentsDir); err != nil {
		return PromoteBatchResult{}, err
	}
	dir, err := filepath.Abs(req.AgentsDir)
	if err != nil {
		return PromoteBatchResult{}, fmt.Errorf("%w: %s", ErrPromoteAgentsDirInvalid, req.AgentsDir)
	}

	agents := discoverPromotableAgents(dir, desc)
	if len(agents) == 0 {
		return PromoteBatchResult{}, fmt.Errorf("%w in %s", ErrPromoteNoPromotableAgents, dir)
	}
	chosen, err := s.askPromoteAgents(ctx, dir, agents)
	if err != nil {
		return PromoteBatchResult{}, err
	}

	run := &promoteBatchRun{
		s: s, ctx: ctx, desc: desc,
		result: PromoteBatchResult{HarnessID: req.HarnessID, AgentsDir: req.AgentsDir},
		tools:  map[string]batchToolAnswer{},
		tiers:  map[string]string{},
	}
	for _, a := range chosen {
		run.entries = append(run.entries, &batchEntry{agent: a})
		run.result.Agents = append(run.result.Agents, PromoteBatchAgentResult{
			SourcePath: a.path, Key: a.key, Status: PromoteBatchNotProcessed,
		})
	}
	defer run.closeSources()

	run.execute(req.HarnessID)
	return run.result, nil
}

// execute runs the batch after the selection: duplicates and unreadable files settle first,
// then the once-per-batch questions, then the agents one by one. An abort leaves every agent
// without an outcome as not processed.
func (r *promoteBatchRun) execute(harnessID string) {
	r.settleDuplicatesAndLoad(harnessID)
	if !r.chooseLayer() || !r.askUnmappedTools() || !r.askTiers() || !r.askProjectRegions() {
		return
	}
	r.nextID = r.firstNewID()
	for i, e := range r.entries {
		if e.settled {
			continue
		}
		if !r.processAgent(i, e) {
			return
		}
	}
}

// settleDuplicatesAndLoad skips every file whose key an earlier selected file already has and
// loads the others; a file that cannot be loaded fails.
func (r *promoteBatchRun) settleDuplicatesAndLoad(harnessID string) {
	seen := map[string]bool{}
	for i, e := range r.entries {
		if seen[e.agent.key] {
			r.settle(i, PromoteBatchSkipped, "another selected file has the same key "+e.agent.key, nil)
			continue
		}
		seen[e.agent.key] = true
		src, err := r.s.loadPromoteSource(e.agent.path, harnessID)
		if err != nil {
			r.settle(i, PromoteBatchFailed, err.Error(), err)
			continue
		}
		e.src = src
	}
}

// settle records the outcome of entry i that needs no further processing.
func (r *promoteBatchRun) settle(i int, status PromoteBatchStatus, reason string, err error) {
	r.entries[i].settled = true
	res := &r.result.Agents[i]
	res.Status, res.Reason, res.Err = status, reason, err
}

// closeSources releases the harness modules of the loaded sources.
func (r *promoteBatchRun) closeSources() {
	for _, e := range r.entries {
		if e.src != nil {
			_ = e.src.module.Close()
		}
	}
}

// firstNewID is the numeric id one greater than the largest the catalog holds.
func (r *promoteBatchRun) firstNewID() int {
	n, err := strconv.Atoi(nextNumericID(r.s.deps.Catalog))
	if err != nil {
		return 1
	}
	return n
}

// abort marks the run aborted and reports false so callers can return it directly.
func (r *promoteBatchRun) abort() bool {
	r.result.Aborted = true
	return false
}

// selectOne asks a choice question. It reports false, and marks the run aborted, when the
// question was cancelled, failed, or the context ended while or after it was answered.
func (r *promoteBatchRun) selectOne(q domain.ChoiceQuestion) (domain.ChoiceAnswer, bool) {
	ans, err := r.s.deps.Interaction.SelectOne(r.ctx, q)
	if err != nil || r.ctx.Err() != nil || ans.Status == domain.Cancelled {
		return ans, r.abort()
	}
	return ans, true
}

// askText asks a text question with the same abort rule as selectOne.
func (r *promoteBatchRun) askText(q domain.TextQuestion) (domain.TextAnswer, bool) {
	ans, err := r.s.deps.Interaction.AskText(r.ctx, q)
	if err != nil || r.ctx.Err() != nil || ans.Status == domain.Cancelled {
		return ans, r.abort()
	}
	return ans, true
}
