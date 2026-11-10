package app

import (
	"context"

	"mosaic-log-analyzer/internal/domain"
)

// digestResolver resolves sources for Analyze: a usable log source keeps its
// kind and gains the digest files found beside it; a path that is not a log
// source but holds digests becomes a digest file or digest directory source.
func (s *Service) digestResolver() sourceResolver {
	if s.deps.Digests == nil {
		return s.logsOnlyResolver()
	}
	return sourceResolver{
		classify: func(path string) (domain.Source, []string) {
			src := s.deps.Source.Classify(path)
			files := s.deps.Digests.Discover(path)
			if src.IsUsable() || len(files) == 0 {
				return src, files
			}
			if len(files) == 1 && files[0] == path {
				return domain.Source{Kind: domain.SourceDigestFile, Path: path}, files
			}
			return domain.Source{Kind: domain.SourceDigestDir, Path: path}, files
		},
		digestsAt: func(src domain.Source) []string {
			return s.deps.Digests.Discover(src.Path)
		},
	}
}

// loadedDigest is one successfully decoded digest and the file it came from.
type loadedDigest struct {
	path    string
	digest  domain.Digest
	carried []domain.Finding // findings the digest carries, scoped by their Run
}

// digestLoad is the outcome of reading every discovered digest file.
type digestLoad struct {
	digests []loadedDigest
	// loadFindings are raised while loading (invalid or unsupported files);
	// they stay apart from the findings carried inside digests.
	loadFindings []domain.Finding
}

// readDigests reads every digest file in order. A file that cannot be used
// yields a load finding and the remaining files still load. An error is
// returned only when ctx is cancelled.
func (s *Service) readDigests(ctx context.Context, files []string) (digestLoad, error) {
	var out digestLoad
	for _, f := range files {
		d, findings, err := s.deps.Digests.Read(ctx, f)
		if err != nil {
			return out, err
		}
		var carried []domain.Finding
		for _, fi := range findings {
			if isLoadTimeKind(fi.Kind) {
				out.loadFindings = append(out.loadFindings, fi)
			} else {
				carried = append(carried, fi)
			}
		}
		if isZeroDigest(d) {
			continue
		}
		carried = append(carried, d.Findings...)
		out.digests = append(out.digests, loadedDigest{path: f, digest: d, carried: carried})
	}
	return out, nil
}

// isZeroDigest reports whether d is the empty value Read returns for a file it
// could not use.
func isZeroDigest(d domain.Digest) bool {
	return d.Header == (domain.DigestHeader{}) &&
		len(d.Aggregate.Runs) == 0 && d.Aggregate.Unattributable == nil &&
		len(d.Findings) == 0
}

// isLoadTimeKind reports whether a finding kind is raised while loading
// digests, as opposed to being carried inside a digest.
func isLoadTimeKind(k domain.FindingKind) bool {
	switch k {
	case domain.FindingInvalidDigest, domain.FindingUnsupportedDigestVersion, domain.FindingDuplicateRun:
		return true
	}
	return false
}

// mergeDigestSources reads the discovered digest files and merges their runs
// into the log aggregate. It returns the merged aggregate, every finding the
// digests contribute: load-time findings, duplicate findings and the findings
// carried by the digests that were not discarded. The carried findings alone
// are returned separately for export. With no digest files the aggregate comes
// back unchanged.
func (s *Service) mergeDigestSources(ctx context.Context, logs domain.Aggregate, files []string) (domain.Aggregate, []domain.Finding, []domain.Finding, error) {
	if len(files) == 0 || s.deps.Digests == nil {
		return logs, nil, nil, nil
	}
	load, err := s.readDigests(ctx, files)
	if err != nil {
		return logs, nil, nil, err
	}
	merged := mergeDigests(logs, load.digests)

	findings := make([]domain.Finding, 0, len(load.loadFindings)+len(merged.duplicates)+len(merged.carried))
	findings = append(findings, load.loadFindings...)
	findings = append(findings, merged.duplicates...)
	findings = append(findings, merged.carried...)
	return merged.aggregate, findings, merged.carried, nil
}
