package domain

// UsageIngestRequest names the run folder and the usage export to ingest into it.
type UsageIngestRequest struct {
	RunDir     string
	ExportPath string
}

// UsageIngestStatus is the outcome class of one recovery ingest.
type UsageIngestStatus uint8

const (
	IngestDone         UsageIngestStatus = iota // lines read and state written
	IngestLockHeld                              // another ingest holds the run's lock
	IngestFileMissing                           // the export file does not exist
	IngestUnknownState                          // the run's ingest state is not readable by this version
	IngestFailed                                // infrastructure failure
)

// UsageIngestDiagnostic is one message the ingest reported.
type UsageIngestDiagnostic struct {
	Kind    string
	Message string
}

// UsageIngestResult reports one recovery ingest.
type UsageIngestResult struct {
	Status         UsageIngestStatus
	RecordsWritten int
	Finalized      int
	Pending        int
	Diagnostics    []UsageIngestDiagnostic
}

// UsageIngestState is what a run's ingest state says about usage not yet placed.
type UsageIngestState struct {
	PendingRecords  int  // pending spans left in the run's ingest state
	StateUnreadable bool // state present but not readable by this version
}
