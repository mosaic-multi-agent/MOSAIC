package screens

import "mosaic-log-analyzer/internal/domain"

// sessionRequest is the request of a per-run view to show the Session screen of
// an actor, made by pressing enter on its row. The router reads and clears it.
type sessionRequest struct {
	actor domain.ActorRef
	set   bool
}

// SessionRequested reports the actor whose session enter asked for.
func (r *sessionRequest) SessionRequested() (domain.ActorRef, bool) { return r.actor, r.set }

// ResetSessionRequest clears the session request.
func (r *sessionRequest) ResetSessionRequest() { r.actor, r.set = domain.ActorRef{}, false }

// requestSession records that the session of actor was asked for.
func (r *sessionRequest) requestSession(actor domain.ActorRef) { r.actor, r.set = actor, true }
