package artifact

import "errors"

// ErrPre30Layout marks the pre-3.0 run artifact layout. For an Execution Log
// with a "Stage" column or a current_state with a "stage" key, Parse returns a
// *domain.RefusalError whose Cause satisfies errors.Is(cause, ErrPre30Layout),
// so errors.Is(err, ErrPre30Layout) holds through RefusalError.Unwrap. How the
// cause wraps the sentinel is not part of the contract.
var ErrPre30Layout = errors.New("pre-3.0 run artifact layout")
