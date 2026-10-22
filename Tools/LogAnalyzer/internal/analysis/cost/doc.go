// Package cost provides the per-request pricing arithmetic for
// mosaic-log-analyzer. It selects the rate-set tier for each
// request (standard vs. long-context) and computes the exact
// nanodollar cost from token counts and rates. The package is pure:
// it imports only internal/domain and contains no I/O, no randomness,
// and no mutable global state, making every function deterministically
// table-testable.
package cost
