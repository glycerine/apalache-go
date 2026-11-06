package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:722-793.

import "github.com/glycerine/g3/pkg/math/polynomial"

func simpleEndpointZero(am *polynomial.AlgebraicManager, e *simpleEndpoint) (int, error) {
	if e.infinite {
		if e.lower {
			return -1, nil
		}
		return 1, nil
	}
	return am.Compare(&e.value, &polynomial.Algebraic{})
}
func (d *simplePowerDomain) termSign(am *polynomial.AlgebraicManager, t *simpleSumTerm) (simpleSign, error) {
	neg, err := am.IsNeg(&t.coefficient)
	if err != nil {
		return simpleNone, err
	}
	var sign simpleSign
	if t.degree&1 == 0 {
		order, err := simpleEndpointZero(am, &d.magnitude.lower)
		if err != nil {
			return simpleNone, err
		}
		if order == 0 && !d.magnitude.lower.open {
			sign = simpleGE
		} else {
			sign = simpleGT
		}
	} else {
		lo, err := simpleEndpointZero(am, &d.original.lower)
		if err != nil {
			return simpleNone, err
		}
		hi, err := simpleEndpointZero(am, &d.original.upper)
		if err != nil {
			return simpleNone, err
		}
		if lo > 0 || (lo == 0 && d.original.lower.open) {
			sign = simpleGT
		} else if lo == 0 {
			sign = simpleGE
		} else if hi < 0 || (hi == 0 && d.original.upper.open) {
			sign = simpleLT
		} else if hi == 0 {
			sign = simpleLE
		} else {
			return simpleNone, nil
		}
	}
	if neg {
		sign = sign.flipped()
	}
	return sign, nil
}
func mergeSimpleSumSign(a, b simpleSign) simpleSign {
	if a == simpleEQ {
		return b
	}
	if b == simpleEQ {
		return a
	}
	if a == simpleNone || b == simpleNone {
		return simpleNone
	}
	if (a == simpleLT || a == simpleLE) && (b == simpleLT || b == simpleLE) {
		if a != b {
			return simpleLT
		}
		return a
	}
	if (a == simpleGT || a == simpleGE) && (b == simpleGT || b == simpleGE) {
		if a != b {
			return simpleGT
		}
		return a
	}
	return simpleNone
}
