package main

import (
	"time"

	"mosaic-run/internal/domain"
)

// timeoutsFromArgs pre-scans the raw CLI args for --timeout, --hitl-timeout
// and --no-output-timeout (both "--flag value" and "--flag=value" forms) and
// returns the invocation timeouts for buildAdapter. An absent, unparsable or
// non-positive value falls back to the matching domain default (cli.Run
// reports invalid values to the user separately). For --no-output-timeout a
// zero value disables the watchdog (domain.NoOutputDisabled).
func timeoutsFromArgs(args []string) domain.InvocationTimeouts {
	return domain.InvocationTimeouts{
		NonHITL:  scanDuration(args, "--timeout", domain.DefaultTimeout),
		HITL:     scanDuration(args, "--hitl-timeout", domain.DefaultHITLTimeout),
		NoOutput: scanNoOutput(args),
	}
}

// scanDuration returns the positive duration given for flag, or fallback.
func scanDuration(args []string, flag string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(scanFlag(args, flag)); err == nil && d > 0 {
		return d
	}
	return fallback
}

// scanNoOutput returns the no-output watchdog duration given on the command
// line: a positive value as given, domain.NoOutputDisabled for a zero value,
// and domain.DefaultNoOutputTimeout when absent, unparsable or negative.
func scanNoOutput(args []string) time.Duration {
	d, err := time.ParseDuration(scanFlag(args, "--no-output-timeout"))
	switch {
	case err != nil || d < 0:
		return domain.DefaultNoOutputTimeout
	case d == 0:
		return domain.NoOutputDisabled
	}
	return d
}
