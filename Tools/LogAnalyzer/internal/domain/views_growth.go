package domain

// GrowthState says whether and how a call's context growth was measured.
type GrowthState uint8

const (
	GrowthNotSingular   GrowthState = iota // zero value: the call is not singular
	GrowthMeasured                         // singular, growth >= 0
	GrowthNegative                         // singular, growth < 0: left out and counted
	GrowthNotMeasurable                    // singular, a needed token category absent: left out and counted
)

// CallGrowth is one call's context growth.
type CallGrowth struct {
	State  GrowthState
	Tool   string // the single requested tool; "" for GrowthNotSingular
	Tokens int64  // GrowthMeasured: growth (>= 0); GrowthNegative: the negative value; else 0
}

// ToolGrowth summarises the measured growth of one tool within a grouping.
type ToolGrowth struct {
	Samples       int        // GrowthMeasured calls
	Average       TokenCount // sum / Samples rounded half-up; absent when Samples == 0
	Min, Max      TokenCount // absent when Samples == 0
	Negative      int        // GrowthNegative calls
	NotMeasurable int        // GrowthNotMeasurable calls
}

// IsProvided reports whether the tool has at least one sample.
func (g ToolGrowth) IsProvided() bool { return g.Samples > 0 }
