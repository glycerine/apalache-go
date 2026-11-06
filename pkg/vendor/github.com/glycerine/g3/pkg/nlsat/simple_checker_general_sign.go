package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1350-1438.

import "github.com/glycerine/g3/pkg/math/polynomial"

func classifySimpleInterval(am *polynomial.AlgebraicManager, d *simpleInterval) (simpleSign, error) {
	if d.lower.infinite {
		order, err := am.Compare(&d.upper.value, &polynomial.Algebraic{})
		if err != nil {
			return simpleNone, err
		}
		if order < 0 {
			return simpleLT, nil
		}
		if order == 0 {
			if d.upper.open {
				return simpleLT, nil
			}
			return simpleLE, nil
		}
		return simpleNone, nil
	}
	if d.upper.infinite {
		order, err := am.Compare(&d.lower.value, &polynomial.Algebraic{})
		if err != nil {
			return simpleNone, err
		}
		if order > 0 {
			return simpleGT, nil
		}
		if order == 0 {
			if d.lower.open {
				return simpleGT, nil
			}
			return simpleGE, nil
		}
		return simpleNone, nil
	}
	lo, err := am.Compare(&d.lower.value, &polynomial.Algebraic{})
	if err != nil {
		return simpleNone, err
	}
	hi, err := am.Compare(&d.upper.value, &polynomial.Algebraic{})
	if err != nil {
		return simpleNone, err
	}
	if lo == 0 && hi == 0 {
		return simpleEQ, nil
	}
	if lo == 0 {
		if d.lower.open {
			return simpleGT, nil
		}
		return simpleGE, nil
	}
	if hi == 0 {
		if d.upper.open {
			return simpleLT, nil
		}
		return simpleLE, nil
	}
	if lo > 0 {
		return simpleGT, nil
	}
	if hi < 0 {
		return simpleLT, nil
	}
	return simpleNone, nil
}
func paritySimpleSign(sign simpleSign, even bool) simpleSign {
	if !even {
		return sign
	}
	if sign == simpleEQ {
		return sign
	}
	if sign == simpleGE || sign == simpleLE || sign == simpleNone {
		return simpleGE
	}
	return simpleGT
}
func multiplySimpleSign(a, b simpleSign) simpleSign {
	if a == simpleEQ || b == simpleEQ {
		return simpleEQ
	}
	if a == simpleNone || b == simpleNone {
		return simpleNone
	}
	negative := func(s simpleSign) bool { return s == simpleLT || s == simpleLE }
	strict := func(s simpleSign) bool { return s == simpleLT || s == simpleGT }
	neg := negative(a) != negative(b)
	isStrict := strict(a) && strict(b)
	if neg {
		if isStrict {
			return simpleLT
		}
		return simpleLE
	}
	if isStrict {
		return simpleGT
	}
	return simpleGE
}
