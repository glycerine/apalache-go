package numeral

// Source: src/util/mpf.cpp:1850-1855; src/util/mpf.h:207-208.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

func (m *ArbitraryFloatManager) BiasExp(ebits uint32, unbiased int64) (int64, error) {
	bias, e := m.ExponentBias(ebits)
	if e != nil {
		return 0, e
	}
	if bias > 0 && unbiased > math.MaxInt64-bias || bias < 0 && unbiased < math.MinInt64-bias {
		return 0, arena.ErrBounds
	}
	return unbiased + bias, nil
}

func (m *ArbitraryFloatManager) UnbiasExp(ebits uint32, biased int64) (int64, error) {
	bias, e := m.ExponentBias(ebits)
	if e != nil {
		return 0, e
	}
	return roundSignedSub(biased, bias)
}
