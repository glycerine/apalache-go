package realclosure

// Source: src/math/realclosure/realclosure.cpp:4247-4344 and 4768-4815.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *realValueStore) refineAndCertifyRetainedValue(value *realValue, bounds *numeral.BinaryRationalManager, precision uint32, usePrem bool) (sign int32, refined bool, err error) {
	refined, err = s.refineAnyRetainedInterval(value, bounds, precision)
	if err != nil || value == nil {
		return 0, refined, err
	}
	sign, err = s.determineAnyRetainedValueSign(value, usePrem)
	return sign, refined, err
}
