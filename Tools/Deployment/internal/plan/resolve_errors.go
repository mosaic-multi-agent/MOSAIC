package plan

import (
	"errors"
	"fmt"

	"mosaic-deploy/internal/catalog"
)

// UnknownWorkflowError reports a workflow id that none of the searched catalogs provides.
// It matches ErrUnknownWorkflow through errors.Is and exposes the id to callers.
type UnknownWorkflowError struct {
	WorkflowID string
	Searched   string
}

// Error names the workflow id and where it was sought.
func (e *UnknownWorkflowError) Error() string {
	return fmt.Sprintf("%s: %q (%s)", ErrUnknownWorkflow.Error(), e.WorkflowID, e.Searched)
}

// Is reports a match for ErrUnknownWorkflow.
func (e *UnknownWorkflowError) Is(target error) bool {
	return target == ErrUnknownWorkflow
}

// NewUnknownWorkflowError builds the unknown-workflow error for workflowID, naming where the
// catalog was searched.
func NewUnknownWorkflowError(c catalog.Catalog, workflowID string) error {
	return unknownWorkflowError(c, workflowID)
}

// unknownWorkflowError builds the error for a workflow id that none of the searched catalogs provides.
func unknownWorkflowError(c catalog.Catalog, workflowID string) error {
	return &UnknownWorkflowError{WorkflowID: workflowID, Searched: searchedCatalogs(c)}
}

// RequestedAgentNotFoundError reports an agent named directly by the caller (not reached
// through a workflow) that no searched catalog provides. It matches ErrUnknownAgent through
// errors.Is while carrying its own message, free of workflow wording.
type RequestedAgentNotFoundError struct {
	Kind     string
	ID       string
	Searched string
}

// Error describes the requested agent and where it was sought.
func (e *RequestedAgentNotFoundError) Error() string {
	return fmt.Sprintf("requested %s %q not found (%s)", e.Kind, e.ID, e.Searched)
}

// Is reports a match for ErrUnknownAgent.
func (e *RequestedAgentNotFoundError) Is(target error) bool {
	return target == ErrUnknownAgent
}

// requestedAgentNotFound builds the error for an unknown directly requested agent of the given kind.
func requestedAgentNotFound(c catalog.Catalog, kind, id string) error {
	return &RequestedAgentNotFoundError{Kind: kind, ID: id, Searched: searchedCatalogs(c)}
}

// ErrUnknownHook is matched (errors.Is) by every UnknownHookError.
var ErrUnknownHook = errors.New("hook bundle not found")

// UnknownHookError reports a requested hook id that none of the searched catalogs provides.
type UnknownHookError struct {
	HookID   string
	Searched string // same text as UnknownWorkflowError.Searched
}

// Error describes the hook id and where it was sought.
func (e *UnknownHookError) Error() string {
	return fmt.Sprintf("%s: %q (%s)", ErrUnknownHook.Error(), e.HookID, e.Searched)
}

// Is reports a match for ErrUnknownHook.
func (e *UnknownHookError) Is(target error) bool {
	return target == ErrUnknownHook
}
