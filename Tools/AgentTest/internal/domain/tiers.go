package domain

// TierTestSubject is the recommended_tier every agent-under-test in the test
// catalogue declares. Any number of agent files may carry it (for example the
// orchestrator copies in catalog/Orchestrator/); the pre-spawn check expects
// the subject model on every deployed agent that declares it.
//
// This is the single spelling of the tier name in Go source. No other package
// in this module may spell the literal; reference this constant instead.
const TierTestSubject = "TEST-SUBJECT"

// TierTestStub is the recommended_tier every test stub in the test catalogue
// declares, in both agents/ and catalog/Subagents/TestStubs/.
//
// This is the single spelling of the tier name in Go source. No other package
// in this module may spell the literal; reference this constant instead.
const TierTestStub = "TEST-STUB"
