package interval

// Source: src/math/interval/mod_interval.h:119-135,181-186.

import "math/big"

func (v *ModularInterval) Negate(result *ModularInterval) (bool, error) {
	if v == nil || result == nil {
		return false, ErrExtendedDomain
	}
	result.size = v.size
	if !v.tight {
		if v.small {
			result.setSmall(0, modularSmallBound(v.size), true)
		} else {
			result.setLarge(bigZero, modularBound(v.size), true)
		}
		return true, nil
	}
	if v.IsFull() {
		return false, nil
	}
	if v.small {
		if v.lowerSmall == 0 {
			result.setSmall(v.upperSmall+1, modularSmallBound(v.size), false)
		} else if v.upperSmall == modularSmallBound(v.size) {
			result.setSmall(0, v.lowerSmall-1, false)
		} else {
			result.setSmall(v.upperSmall+1, v.lowerSmall-1, false)
		}
		return true, nil
	}
	bound := modularBound(v.size)
	if v.lowerLarge.Sign() == 0 {
		result.setLarge(new(big.Int).Add(v.upperLarge, bigOne), bound, false)
	} else if v.upperLarge.Cmp(bound) == 0 {
		result.setLarge(bigZero, new(big.Int).Sub(v.lowerLarge, bigOne), false)
	} else {
		result.setLarge(new(big.Int).Add(v.upperLarge, bigOne), new(big.Int).Sub(v.lowerLarge, bigOne), false)
	}
	return true, nil
}

var bigZero = big.NewInt(0)
