package interval

// Source: src/math/interval/mod_interval.h:21-55,137-179.

import "math/big"

// ModularInterval is a cyclic unsigned bit-vector interval. Lower > Upper is
// the source wrapped representation. Widths through 64 bits retain the source
// uint64 fast path; wider values use arbitrary-precision integers.
type ModularInterval struct {
	small                  bool
	lowerSmall, upperSmall uint64
	lowerLarge, upperLarge *big.Int
	size                   uint32
	tight                  bool
}

func NewModularInterval(lower, upper *big.Int, size uint32, tight bool) (*ModularInterval, error) {
	if lower == nil || upper == nil || size == 0 || lower.Sign() < 0 || upper.Sign() < 0 {
		return nil, ErrExtendedDomain
	}
	bound := modularBound(size)
	if lower.Cmp(bound) > 0 || upper.Cmp(bound) > 0 {
		return nil, ErrExtendedDomain
	}
	v := &ModularInterval{size: size, tight: tight, small: size <= 64}
	if v.small {
		v.lowerSmall, v.upperSmall = lower.Uint64(), upper.Uint64()
	} else {
		v.lowerLarge, v.upperLarge = new(big.Int).Set(lower), new(big.Int).Set(upper)
	}
	if !v.Invariant() {
		return nil, ErrExtendedDomain
	}
	return v, nil
}

func modularBound(size uint32) *big.Int {
	if size == 0 {
		return new(big.Int)
	}
	return new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(size)), big.NewInt(1))
}

func modularSmallBound(size uint32) uint64 {
	if size == 64 {
		return ^uint64(0)
	}
	return (uint64(1) << size) - 1
}
