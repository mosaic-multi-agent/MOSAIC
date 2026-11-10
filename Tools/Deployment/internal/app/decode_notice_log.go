package app

// decode_notice_log.go routes decode notices collected while indexing or probing deployed
// agent files (for example a body-less Codex agent) to the run log, so they reach the user.

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/logging"
)

// logDecodeNotices writes each notice to the run log as a decode warning.
func (s *service) logDecodeNotices(notices []string) {
	for _, notice := range notices {
		s.deps.Logger.Event(logging.Event{Level: logging.LevelWarn, Kind: "decode", Message: notice})
	}
}

// logProbeDecodeNotices writes the decode notices carried by every probed deployed state.
func (s *service) logProbeDecodeNotices(states map[string]domain.DeployedArtifactState) {
	for _, state := range states {
		s.logDecodeNotices(state.DecodeNotices)
	}
}
