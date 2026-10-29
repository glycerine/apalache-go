package lp

// Source: src/util/lp/lu.h:122-155; lu.hpp:149-160,717-726.

import "errors"

type LUStatus int32

const (
	LUOK LUStatus = iota
	LUDegenerated
)

var ErrLUClosed = errors.New("lp: LU factorization is closed")
var ErrLUStopped = errors.New("lp: LU factorization has a stopped operation")

func luDebugBasis(rows, columns uint32, basis []uint32) error {
	if !sourceDebugEnabled {
		return nil
	}
	seen := make(map[uint32]bool)
	for i := uint32(0); i < rows; i++ {
		if uint64(i) >= uint64(len(basis)) {
			return ErrBounds
		}
		if basis[i] >= columns {
			return ErrAssertion
		}
		seen[basis[i]] = true
	}
	if uint32(len(seen)) != rows {
		return ErrAssertion
	}
	return nil
}
func luTooDense(dim, j, active uint32, s *Settings) bool {
	r := dim - j
	if r < 5 {
		return false
	}
	// Source unsigned multiplication precedes floating conversion.
	return float64(r*r)*s.DensityThreshold <= float64(active)
}
