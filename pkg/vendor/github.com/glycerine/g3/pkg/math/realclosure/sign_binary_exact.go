package realclosure

// Source: src/math/realclosure/realclosure.cpp:3975-4030.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) exactSignAtBinaryBound(coefficients []*realValue, bounds *numeral.BinaryRationalManager, bound *numeral.BinaryRational, usePrem bool) (sign int32, err error) {
	restoreAuxiliary := s.withAuxiliaryValues()
	defer restoreAuxiliary()
	value, err := s.exactPolynomialValueAtBinaryBound(coefficients, bounds, bound, usePrem)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(value)) }()
	return s.determineRankedRetainedValueSign(value, usePrem)
}
