package interval

// Source: src/math/interval/mod_interval.h:181-230.

import "math/big"

func (v *ModularInterval) Size() uint32 {
	if v == nil {
		return 0
	}
	return v.size
}
func (v *ModularInterval) Tight() bool { return v != nil && v.tight }

func (v *ModularInterval) Lower() *big.Int {
	if v == nil {
		return nil
	}
	if v.small {
		return new(big.Int).SetUint64(v.lowerSmall)
	}
	return new(big.Int).Set(v.lowerLarge)
}

func (v *ModularInterval) Upper() *big.Int {
	if v == nil {
		return nil
	}
	if v.small {
		return new(big.Int).SetUint64(v.upperSmall)
	}
	return new(big.Int).Set(v.upperLarge)
}

func (v *ModularInterval) setSmall(lower, upper uint64, tight bool) {
	v.small, v.lowerSmall, v.upperSmall, v.tight = true, lower, upper, tight
	v.lowerLarge, v.upperLarge = nil, nil
}

func (v *ModularInterval) setLarge(lower, upper *big.Int, tight bool) {
	v.small, v.tight = false, tight
	v.lowerLarge, v.upperLarge = new(big.Int).Set(lower), new(big.Int).Set(upper)
	v.lowerSmall, v.upperSmall = 0, 0
}
