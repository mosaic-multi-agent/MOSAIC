package domain

// OutcomeSplit divides a run's dispatch cost by how each dispatch ended.
type OutcomeSplit struct {
	Success     GroupFigures // closed with StatusCode "SUCCESS"
	OtherStatus GroupFigures // closed with any other non-empty StatusCode
	OpenOrNone  GroupFigures // open, or closed without a status
}

// ReworkFigures are the repeat-dispatch and likely-rework figures of a scope.
type ReworkFigures struct {
	Repeats      GroupFigures // repeat dispatches; includes planned repeats
	LikelyRework GroupFigures // likely-rework subset; a heuristic
}

// ReworkLimitNote is shown wherever rework is shown.
const ReworkLimitNote = "logs do not record the workflow step of a dispatch, so rework cannot be measured exactly"
