package lp

// Source: src/util/lp/lp_settings.h:32-86; lp_settings.hpp:25-70.

import "errors"

var ErrAssertion = errors.New("lp: source assertion failed")

type ColumnType int32

const (
	ColumnFree ColumnType = iota
	ColumnLowBound
	ColumnUpperBound
	ColumnBoxed
	ColumnFixed
)

type SimplexStrategy int32

const (
	StrategyTableauRows SimplexStrategy = iota
	StrategyTableauCosts
	StrategyLU
	StrategyUndecided
)

type Status int32

const (
	StatusUnknown Status = iota
	StatusInfeasible
	StatusTentativeUnbounded
	StatusUnbounded
	StatusTentativeDualUnbounded
	StatusDualUnbounded
	StatusOptimal
	StatusFeasible
	StatusFloatingPointError
	StatusTimeExhausted
	StatusIterationsExhausted
	StatusEmpty
	StatusUnstable
	StatusCancelled
)

func ColumnTypeString(t ColumnType) (string, error) {
	switch t {
	case ColumnFree:
		return "free_column", nil
	case ColumnLowBound:
		return "low_bound", nil
	case ColumnUpperBound:
		return "upper_bound", nil
	case ColumnBoxed:
		return "boxed", nil
	case ColumnFixed:
		return "fixed", nil
	}
	return "unknown", sourceAssertionFailure()
}

func StatusString(s Status) (string, error) {
	switch s {
	case StatusUnknown:
		return "UNKNOWN", nil
	case StatusInfeasible:
		return "INFEASIBLE", nil
	case StatusUnbounded:
		return "UNBOUNDED", nil
	case StatusTentativeDualUnbounded:
		return "TENTATIVE_DUAL_UNBOUNDED", nil
	case StatusDualUnbounded:
		return "DUAL_UNBOUNDED", nil
	case StatusOptimal:
		return "OPTIMAL", nil
	case StatusFeasible:
		return "FEASIBLE", nil
	case StatusFloatingPointError:
		return "FLOATING_POINT_ERROR", nil
	case StatusTimeExhausted:
		return "TIME_EXHAUSTED", nil
	case StatusIterationsExhausted:
		return "ITERATIONS_EXHAUSTED", nil
	case StatusEmpty:
		return "EMPTY", nil
	case StatusUnstable:
		return "UNSTABLE", nil
	}
	return "UNKNOWN", sourceAssertionFailure()
}

// ParseStatus preserves the source's smaller accepted vocabulary. In particular
// its dual-unbounded and unstable output spellings are not accepted as input.
func ParseStatus(text string) (Status, error) {
	switch text {
	case "UNKNOWN":
		return StatusUnknown, nil
	case "INFEASIBLE":
		return StatusInfeasible, nil
	case "UNBOUNDED":
		return StatusUnbounded, nil
	case "OPTIMAL":
		return StatusOptimal, nil
	case "FEASIBLE":
		return StatusFeasible, nil
	case "FLOATING_POINT_ERROR":
		return StatusFloatingPointError, nil
	case "TIME_EXHAUSTED":
		return StatusTimeExhausted, nil
	case "ITERATIONS_EXHAUSTED":
		return StatusIterationsExhausted, nil
	case "EMPTY":
		return StatusEmpty, nil
	}
	return StatusUnknown, sourceAssertionFailure()
}
