package polynomial

// Source: src/math/polynomial/linear_eq_solver.h.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	// Shared lifecycle state; each concrete solver owns native typed matrix/RHS slices.
)

type linearStorage struct {
	running                          bool
	integer                          *unumeral.IntegerManager
	n                                uint32
	header                           uint64
	sourceCause                      error
	sourceStage                      uint8
	flushStage                       uint8
	flushRow, flushColumn            uint32
	closing, sourceFinalized, closed bool

	integerBinding, naturalBinding unumeral.NumericCallerBinding
}
