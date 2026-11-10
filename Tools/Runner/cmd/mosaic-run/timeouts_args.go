package main

import (
	"time"

	"mosaic-run/internal/domain"
)

// timeoutsFromArgs pre-scans the raw CLI args for --timeout and --hitl-timeout
// (both "--flag value" and "--flag=value" forms) and returns the invocation
// timeouts for buildAdapter. An absent, unparsable or non-positive value falls
// back to the matching domain default (cli.Run reports invalid values to the
// user separately).
func timeoutsFromArgs(args []string) domain.InvocationTimeouts {
	return domain.InvocationTimeouts{
		NonHITL: scanDuration(args, "--timeout", domain.DefaultTimeout),
		HITL:    scanDuration(args, "--hitl-timeout", domain.DefaultHITLTimeout),
	}
}

// scanDuration returns the positive duration given for flag, or fallback.
func scanDuration(args []string, flag string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(scanFlag(args, flag)); err == nil && d > 0 {
		return d
	}
	return fallback
}
