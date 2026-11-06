package interval

// Source: src/math/interval/mod_interval.h:31-52,202-216.

import "math/big"

func (v *ModularInterval) Invariant() bool {
	if v == nil || v.size == 0 {
		return false
	}
	if v.small {
		bound := modularSmallBound(v.size)
		return v.lowerSmall <= bound && v.upperSmall <= bound &&
			(v.lowerSmall <= v.upperSmall || v.lowerSmall != v.upperSmall+1)
	}
	if v.lowerLarge == nil || v.upperLarge == nil || v.lowerLarge.Sign() < 0 || v.upperLarge.Sign() < 0 || v.lowerLarge.Cmp(modularBound(v.size)) > 0 || v.upperLarge.Cmp(modularBound(v.size)) > 0 {
		return false
	}
	onePast := new(big.Int).Add(v.upperLarge, bigOne)
	return v.lowerLarge.Cmp(v.upperLarge) <= 0 || v.lowerLarge.Cmp(onePast) != 0
}

var bigOne = big.NewInt(1)

func (v *ModularInterval) IsFull() bool {
	if v == nil {
		return false
	}
	if v.small {
		return v.lowerSmall == 0 && v.upperSmall == modularSmallBound(v.size)
	}
	return v.lowerLarge.Sign() == 0 && v.upperLarge.Cmp(modularBound(v.size)) == 0
}
func (v *ModularInterval) IsWrapped() bool {
	if v == nil {
		return false
	}
	if v.small {
		return v.lowerSmall > v.upperSmall
	}
	return v.lowerLarge.Cmp(v.upperLarge) > 0
}
func (v *ModularInterval) IsSingleton() bool {
	if v == nil {
		return false
	}
	if v.small {
		return v.lowerSmall == v.upperSmall
	}
	return v.lowerLarge.Cmp(v.upperLarge) == 0
}
