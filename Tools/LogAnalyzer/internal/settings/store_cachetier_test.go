package settings_test

// Tests for the default_cache_write_tier setting: read per harness, merged
// across the global and project levels, validated against the log-format tiers,
// and documented in the template as a comment only.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const tierKey = "default_cache_write_tier"

func assertTier(t *testing.T, got domain.AnalysisSettings, h domain.Harness, want domain.CacheWriteTier) {
	t.Helper()
	if tier := got.CacheWriteTierFor(h); tier != want {
		t.Errorf("CacheWriteTierFor(%s) = %q, want %q", h, tier, want)
	}
}

func TestLoadCacheTier_NoSetting_EveryHarnessKeepsFiveMinutes(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.projFile, "detected_compaction:\n  drop_percent: 40\n")

	got, findings := loadLayered(t, l)

	for _, h := range []domain.Harness{
		domain.HarnessClaudeCode, domain.HarnessOpenCode, domain.HarnessVSCodeGHCP, domain.HarnessGHCPCLI,
	} {
		assertTier(t, got, h, domain.CacheWriteTier5m)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestLoadCacheTier_ProjectFileSetsOneHarness_OthersKeepDefault(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.projFile, tierKey+":\n  claude-code: 1h\n")

	got, findings := loadLayered(t, l)

	assertTier(t, got, domain.HarnessClaudeCode, domain.CacheWriteTier1h)
	assertTier(t, got, domain.HarnessVSCodeGHCP, domain.CacheWriteTier5m)
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestLoadCacheTier_GlobalFileOnly_AppliesGlobalTier(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, tierKey+":\n  opencode: 1h\n")

	got, _ := loadLayered(t, l)

	assertTier(t, got, domain.HarnessOpenCode, domain.CacheWriteTier1h)
	assertTier(t, got, domain.HarnessClaudeCode, domain.CacheWriteTier5m)
}

func TestLoadCacheTier_ProjectOverridesGlobalPerHarness(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, tierKey+":\n  claude-code: 1h\n  vscode-ghcp: 1h\n")
	writeAt(t, l.projFile, tierKey+":\n  claude-code: 5m\n")

	got, _ := loadLayered(t, l)

	assertTier(t, got, domain.HarnessClaudeCode, domain.CacheWriteTier5m)
	assertTier(t, got, domain.HarnessVSCodeGHCP, domain.CacheWriteTier1h)
}

func TestLoadCacheTier_ProjectAddsHarnessToGlobalSetting(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, tierKey+":\n  claude-code: 1h\n")
	writeAt(t, l.projFile, tierKey+":\n  ghcp-cli: 1h\n")

	got, _ := loadLayered(t, l)

	assertTier(t, got, domain.HarnessClaudeCode, domain.CacheWriteTier1h)
	assertTier(t, got, domain.HarnessGHCPCLI, domain.CacheWriteTier1h)
	assertTier(t, got, domain.HarnessOpenCode, domain.CacheWriteTier5m)
}

func TestLoadCacheTier_EveryLogFormatTierIsAccepted(t *testing.T) {
	tiers := domain.CacheWriteTiers()
	if len(tiers) == 0 {
		t.Fatal("CacheWriteTiers() is empty")
	}
	for _, tier := range tiers {
		l := newLayered(t)
		writeAt(t, l.projFile, tierKey+":\n  claude-code: "+string(tier)+"\n")

		got, findings := loadLayered(t, l)

		assertTier(t, got, domain.HarnessClaudeCode, tier)
		if len(findings) != 0 {
			t.Errorf("tier %q: findings = %v, want none", tier, findings)
		}
	}
}

func TestLoadCacheTier_InvalidValue_FindingWithFilePathAndDefaultApplies(t *testing.T) {
	for _, v := range []string{"2h", "1H", "5M", "\"\"", "60", "true", "[1h]", "{a: b}"} {
		l := newLayered(t)
		writeAt(t, l.projFile, tierKey+":\n  claude-code: "+v+"\n  opencode: 1h\n")

		got, findings := loadLayered(t, l)

		if len(findings) != 1 || findings[0].Kind != domain.FindingInvalidSettingsEntry {
			t.Errorf("value %s: findings = %v, want one invalid-settings finding", v, findings)
			continue
		}
		if findings[0].Path != l.projFile || findings[0].Severity != domain.SeverityWarning {
			t.Errorf("value %s: finding = %+v, want a warning naming %s", v, findings[0], l.projFile)
		}
		if d := findings[0].Detail; strings.Contains(d, "unknown") || !strings.Contains(d, "claude-code") {
			t.Errorf("value %s: detail %q should name the harness with the bad value, not an unknown key", v, d)
		}
		assertTier(t, got, domain.HarnessClaudeCode, domain.CacheWriteTier5m)
		assertTier(t, got, domain.HarnessOpenCode, domain.CacheWriteTier1h) // the valid sibling still loads
	}
}

func TestLoadCacheTier_InvalidValueInGlobalFile_FindingNamesTheGlobalFile(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, tierKey+":\n  claude-code: 3h\n")

	got, findings := loadLayered(t, l)

	if len(findings) != 1 || findings[0].Kind != domain.FindingInvalidSettingsEntry || findings[0].Path != l.globalFile {
		t.Fatalf("findings = %v, want one invalid-settings finding for %s", findings, l.globalFile)
	}
	assertTier(t, got, domain.HarnessClaudeCode, domain.CacheWriteTier5m)
}

func TestLoadCacheTier_UnknownHarness_FindingAndOtherEntriesLoad(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.projFile, tierKey+":\n  not-a-harness: 1h\n  claude-code: 1h\n")

	got, findings := loadLayered(t, l)

	if len(findings) != 1 || findings[0].Kind != domain.FindingInvalidSettingsEntry {
		t.Fatalf("findings = %v, want one invalid-settings finding", findings)
	}
	if !strings.Contains(findings[0].Detail, "not-a-harness") {
		t.Errorf("finding detail %q should name the unknown harness", findings[0].Detail)
	}
	assertTier(t, got, domain.HarnessClaudeCode, domain.CacheWriteTier1h)
}

func TestLoadCacheTier_SettingIsNotAMapping_FindingAndDefaultApplies(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.projFile, tierKey+": 1h\n")

	got, findings := loadLayered(t, l)

	if len(findings) != 1 || findings[0].Kind != domain.FindingInvalidSettingsEntry {
		t.Fatalf("findings = %v, want one invalid-settings finding", findings)
	}
	if strings.Contains(findings[0].Detail, "unknown") || !strings.Contains(findings[0].Detail, tierKey) {
		t.Errorf("finding %q should name %s as a known setting with a bad shape", findings[0].Detail, tierKey)
	}
	assertTier(t, got, domain.HarnessClaudeCode, domain.CacheWriteTier5m)
}

func TestLoadCacheTier_SettingIsNotReportedAsUnknownKey(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.projFile, tierKey+":\n  claude-code: 1h\n")

	_, findings := loadLayered(t, l)

	for _, f := range findings {
		if strings.Contains(f.Detail, "unknown") {
			t.Errorf("finding %q reports the known key as unknown", f.Detail)
		}
	}
}

func TestTemplate_DocumentsCacheTierSettingAsCommentOnly(t *testing.T) {
	l := newLayered(t)
	if _, err := l.store.EnsureGlobalFile(); err != nil {
		t.Fatalf("EnsureGlobalFile: %v", err)
	}
	content := readAt(t, l.globalFile)

	if !strings.Contains(content, tierKey) {
		t.Fatalf("template does not mention %s", tierKey)
	}
	var commented bool
	for _, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, tierKey) {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("line %q mentions %s outside a comment", line, tierKey)
		}
		commented = true
	}
	if !commented {
		t.Errorf("no comment line mentions %s", tierKey)
	}
	for _, tier := range domain.CacheWriteTiers() {
		if !strings.Contains(content, string(tier)) {
			t.Errorf("template does not document the %q tier", tier)
		}
	}
	if lines := nonCommentLines(content); len(lines) != 0 {
		t.Errorf("template has active lines: %v", lines)
	}
}

func TestTemplate_FreshFileLoadsWithDefaultTierAndNoFindings(t *testing.T) {
	l := newLayered(t)
	if _, err := l.store.EnsureGlobalFile(); err != nil {
		t.Fatalf("EnsureGlobalFile: %v", err)
	}

	got, findings := loadLayered(t, l)

	assertTier(t, got, domain.HarnessClaudeCode, domain.CacheWriteTier5m)
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}
