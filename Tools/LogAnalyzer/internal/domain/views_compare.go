package domain

// ChangeDirection is the direction of a change between two runs.
type ChangeDirection uint8

const (
	ChangeUnknown ChangeDirection = iota
	ChangeIncrease
	ChangeDecrease
	ChangeUnchanged
)

// Presence says which of the two compared runs holds a row.
type Presence uint8

const (
	PresentBoth Presence = iota
	OnlyInA
	OnlyInB
)

// ComparedQuantity is one integer figure of both runs and its change.
type ComparedQuantity struct {
	A, B       int64
	HasA, HasB bool
	Delta      int64           // B - A; valid when HasA && HasB
	Percent    Ratio           // Delta / A; valid when HasPercent
	HasPercent bool            // HasA && HasB && A != 0
	Direction  ChangeDirection // Unknown unless both present
}

// ComparedMoney is one money figure of both runs and its change.
type ComparedMoney struct {
	A, B       MoneyValue
	Delta      MoneyValue // Known only when A and B are Known
	Percent    Ratio      // valid when HasPercent (A Known and non-zero)
	HasPercent bool
	Complete   bool // both sides complete
	Direction  ChangeDirection
}

// ComparisonRow is one agent type's (or the run totals') figures in both runs.
type ComparisonRow struct {
	AgentType      string // "" = unknown type; ignored for Totals
	Presence       Presence
	Invocations    ComparedQuantity
	Tokens         ComparedQuantity
	Cost           ComparedMoney
	AvgPeakContext ComparedQuantity
}

// RunComparison compares two runs.
type RunComparison struct {
	A, B                       RunRef
	ProvisionalA, ProvisionalB bool
	Rows                       []ComparisonRow
	Totals                     ComparisonRow
}
