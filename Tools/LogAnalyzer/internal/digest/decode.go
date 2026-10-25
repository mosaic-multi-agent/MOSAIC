package digest

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// invalidError describes why a file is not a valid digest. Its text is built
// from format key names only, never from file content.
type invalidError struct{ detail string }

func (e *invalidError) Error() string { return e.detail }

func invalidf(format string, args ...any) error {
	return &invalidError{detail: fmt.Sprintf(format, args...)}
}

// unsupportedVersionError marks a well-formed digest of another major version.
type unsupportedVersionError struct{ version string }

func (e *unsupportedVersionError) Error() string {
	return "digest_version " + e.version + " is not supported"
}

// headerDoc is the part of a digest every major version shares.
type headerDoc struct {
	Format        *string `json:"format"`
	DigestVersion *string `json:"digest_version"`
}

// decode turns file bytes into a Digest, or into exactly one load finding.
func decode(raw []byte, path string) (domain.Digest, []domain.Finding) {
	if err := checkHeader(raw); err != nil {
		return domain.Digest{}, []domain.Finding{loadFinding(err, path)}
	}
	var doc digestDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return domain.Digest{}, []domain.Finding{loadFinding(jsonError(err), path)}
	}
	d, findings, err := convertDigest(doc, path)
	if err != nil {
		return domain.Digest{}, []domain.Finding{loadFinding(err, path)}
	}
	return d, findings
}

// checkHeader validates format and digest_version without decoding anything else.
func checkHeader(raw []byte) error {
	var h headerDoc
	if err := json.Unmarshal(raw, &h); err != nil {
		return jsonError(err)
	}
	if h.Format == nil || *h.Format != formatName {
		return invalidf("format is not %s", formatName)
	}
	if h.DigestVersion == nil {
		return invalidf("digest_version is missing")
	}
	major, ok := majorOf(*h.DigestVersion)
	if !ok {
		return invalidf("digest_version is not MAJOR.MINOR.PATCH")
	}
	if own, _ := majorOf(Version); major != own {
		return &unsupportedVersionError{version: *h.DigestVersion}
	}
	return nil
}

// jsonError maps a decoding failure to a content-free invalidError.
func jsonError(err error) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		if typeErr.Field != "" {
			return invalidf("value of %s has the wrong type", typeErr.Field)
		}
		return invalidf("top level is not a digest object")
	}
	return invalidf("not valid JSON")
}

func loadFinding(err error, path string) domain.Finding {
	f := domain.Finding{
		Kind:     domain.FindingInvalidDigest,
		Severity: domain.SeverityWarning,
		Path:     path,
		Detail:   err.Error(),
	}
	var unsupported *unsupportedVersionError
	if errors.As(err, &unsupported) {
		f.Kind = domain.FindingUnsupportedDigestVersion
	}
	return f
}

func convertDigest(doc digestDoc, path string) (domain.Digest, []domain.Finding, error) {
	d := domain.Digest{Header: domain.DigestHeader{
		Version:     doc.DigestVersion,
		ToolVersion: doc.ToolVersion,
		WrittenAt:   parseTime(doc.WrittenAt),
	}}
	var findings []domain.Finding
	seen := make(map[string]bool, len(doc.Runs))
	for _, rd := range doc.Runs {
		if rd.RunID == "" {
			return domain.Digest{}, nil, invalidf("a runs entry has no run_id")
		}
		run, err := convertRun(rd, domain.NamedRun(rd.RunID))
		if err != nil {
			return domain.Digest{}, nil, err
		}
		if seen[rd.RunID] {
			findings = append(findings, domain.Finding{
				Kind:     domain.FindingDuplicateRun,
				Severity: domain.SeverityInfo,
				Path:     path,
				Run:      run.Run,
				Detail:   "run appears more than once in the file; the earlier entry is kept",
			})
			continue
		}
		seen[rd.RunID] = true
		d.Aggregate.Runs = append(d.Aggregate.Runs, run)
	}
	sort.SliceStable(d.Aggregate.Runs, func(i, j int) bool {
		return d.Aggregate.Runs[i].Run.ID < d.Aggregate.Runs[j].Run.ID
	})
	if doc.Unattributable != nil {
		bucket, err := convertRun(*doc.Unattributable, domain.UnattributableRun())
		if err != nil {
			return domain.Digest{}, nil, err
		}
		d.Aggregate.Unattributable = &bucket
	}
	for _, fd := range doc.Findings {
		f, err := convertFinding(fd)
		if err != nil {
			return domain.Digest{}, nil, err
		}
		d.Findings = append(d.Findings, f)
	}
	return d, findings, nil
}

func convertFinding(fd findingDoc) (domain.Finding, error) {
	f := domain.Finding{Kind: domain.FindingKind(fd.Kind)}
	switch fd.Severity {
	case severityInfo:
		f.Severity = domain.SeverityInfo
	case severityWarning:
		f.Severity = domain.SeverityWarning
	default:
		return f, invalidf("a finding has a missing or unknown severity")
	}
	switch fd.Scope {
	case scopeRun:
		if fd.RunID == "" {
			return f, invalidf("a run-scoped finding has no run_id")
		}
		f.Run = domain.NamedRun(fd.RunID)
	case scopeUnattributable:
		f.Run = domain.UnattributableRun()
	case scopeGeneral:
	default:
		return f, invalidf("a finding has a missing or unknown scope")
	}
	actor, err := decodeActor(fd.Actor)
	if err != nil {
		return f, err
	}
	f.Actor = actor
	return f, nil
}

// decodeActor reads an actor object; an absent one is the zero ActorRef.
func decodeActor(a *actorDoc) (domain.ActorRef, error) {
	if a == nil {
		return domain.ActorRef{}, nil
	}
	switch a.Kind {
	case actorOrchestrator:
		return domain.Orchestrator(), nil
	case actorAgentInstance:
		return domain.ActorRef{
			Kind:     domain.ActorAgentInstance,
			Instance: domain.AgentInstanceID(a.InstanceID),
			Type:     a.AgentType,
		}, nil
	default:
		return domain.ActorRef{}, invalidf("an actor has a missing or unknown kind")
	}
}
