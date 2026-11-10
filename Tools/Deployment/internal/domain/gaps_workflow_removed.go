package domain

// GapWorkflowRemoved reports a custom workflow, one the catalog stack does not supply, that the
// user left unselected in the Update workflows checklist and that the run therefore removed
// from the orchestrator-role files that held it. One gap is produced per removed workflow id
// per run, whatever the number of files that held it.
const GapWorkflowRemoved GapKind = "workflow-removed"
