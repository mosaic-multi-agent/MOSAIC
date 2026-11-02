package session

import (
	"context"
	"errors"
	"sync"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/guidance"
)

// ErrNoStandingGuidance is returned by StandingGuidance.Update when no running
// session holds standing guidance.
var ErrNoStandingGuidance = errors.New("no standing guidance is active for this session")

// StandingGuidance is the live, shared holder of the run's standing guidance:
// the session publishes and reads it, and a frontend views and edits it during
// a run. Safe for concurrent use. Construct with NewStandingGuidance.
type StandingGuidance struct {
	mu     sync.Mutex
	active bool
	cur    domain.Guidance
	store  domain.GuidanceStore
	folder string
	log    domain.DebugLogger
}

// NewStandingGuidance returns a holder with no active guidance.
func NewStandingGuidance() *StandingGuidance { return &StandingGuidance{} }

// Current returns the guidance in effect and true while a running session holds
// standing guidance: an auto or auto-review run with pre-consultation enabled,
// from the end of the run-start guidance review until the session's Start
// returns. Otherwise it returns the zero Guidance and false.
func (s *StandingGuidance) Current() (domain.Guidance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return domain.Guidance{}, false
	}
	return s.cur, true
}

// Update replaces the guidance in effect with g.Normalized(). On success the
// guidance file is saved, session.guidance.changed is logged with change
// "edited-in-run", and every engine-routed dispatch whose request is built after
// Update returns uses it. A dispatch already running is unaffected.
// It returns ErrNoStandingGuidance when Current would report false, and the
// store's error when the file cannot be saved. In both error cases, the
// guidance in effect and the file are unchanged.
func (s *StandingGuidance) Update(_ context.Context, g domain.Guidance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return ErrNoStandingGuidance
	}
	g = g.Normalized()
	if err := s.store.Save(s.folder, g); err != nil {
		return err
	}
	s.cur = g
	logGuidanceChange(s.log, domain.GuidanceEditedInRun, g)
	return nil
}

// activate publishes g as the guidance in effect for a running session.
func (s *StandingGuidance) activate(g domain.Guidance, store domain.GuidanceStore, folder string, log domain.DebugLogger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = true
	s.cur = g
	s.store = store
	s.folder = folder
	s.log = log
}

// deactivate ends the session's hold on the guidance.
func (s *StandingGuidance) deactivate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	s.cur = domain.Guidance{}
}

// logGuidanceChange writes session.guidance.changed with the full guidance in
// the guidance file format.
func logGuidanceChange(log domain.DebugLogger, change domain.GuidanceChange, g domain.Guidance) {
	log.Log(domain.EventSessionGuidanceChanged, string(guidance.Format(g)),
		domain.F("change", string(change)),
	)
}

// noGuidanceStore is the store used when Deps.GuidanceStore is nil: it never
// finds a file and saves nothing.
type noGuidanceStore struct{}

func (noGuidanceStore) Load(string) (domain.Guidance, bool, error) {
	return domain.Guidance{}, false, nil
}

func (noGuidanceStore) Save(string, domain.Guidance) error { return nil }
