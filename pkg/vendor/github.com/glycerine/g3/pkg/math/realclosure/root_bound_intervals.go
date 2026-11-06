package realclosure

// Source: src/math/realclosure/realclosure.cpp:2118-2195.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func isNonzeroBinomial(p []*realValue) bool {
	if len(p) < 2 || p[0] == nil || p[len(p)-1] == nil {
		return false
	}
	for i := 1; i+1 < len(p); i++ {
		if p[i] != nil {
			return false
		}
	}
	return true
}

func magnitudeToBinary(bounds *numeral.BinaryRationalManager, magnitude int32, negative bool, out *numeral.BinaryRational) error {
	if bounds == nil || out == nil {
		return arena.ErrInvalidSlot
	}
	if err := bounds.SetInt32(out, 1); err != nil {
		return err
	}
	if magnitude < 0 {
		if err := bounds.Div2k(out, uint32(-magnitude)); err != nil {
			return err
		}
	} else if err := bounds.Mul2k(out, uint32(magnitude)); err != nil {
		return err
	}
	if negative {
		return bounds.Neg(out)
	}
	return nil
}

func makeRootBoundInterval(bounds *numeral.BinaryRationalManager, lowerMagnitude int32, hasLower bool, upperMagnitude int32, hasUpper, negative bool) (result *BinaryInterval, err error) {
	if bounds == nil {
		return nil, arena.ErrInvalidSlot
	}
	result, err = NewBinaryInterval(bounds)
	if err != nil {
		return nil, err
	}
	lower, upper := numeral.NewBinaryRational(), numeral.NewBinaryRational()
	defer func() {
		err = errors.Join(err, bounds.Del(&lower), bounds.Del(&upper))
		if err != nil && result != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	if negative {
		if hasLower {
			if err = magnitudeToBinary(bounds, lowerMagnitude, true, &lower); err != nil {
				return nil, err
			}
		} else if err = result.ResetLower(); err != nil {
			return nil, err
		}
		if hasUpper {
			if err = magnitudeToBinary(bounds, upperMagnitude, true, &upper); err != nil {
				return nil, err
			}
		} else if err = bounds.SetInt32(&upper, 0); err != nil {
			return nil, err
		}
	} else {
		if hasLower {
			if err = magnitudeToBinary(bounds, lowerMagnitude, false, &lower); err != nil {
				return nil, err
			}
		} else if err = bounds.SetInt32(&lower, 0); err != nil {
			return nil, err
		}
		if hasUpper {
			if err = magnitudeToBinary(bounds, upperMagnitude, false, &upper); err != nil {
				return nil, err
			}
		} else if err = result.ResetUpper(); err != nil {
			return nil, err
		}
	}
	if (negative && !hasLower) || (!negative && !hasUpper) {
		if negative {
			if err = bounds.Copy(&result.upper, &upper); err != nil {
				return nil, err
			}
			result.upperInf, result.upperOpen = false, false
		} else {
			if err = bounds.Copy(&result.lower, &lower); err != nil {
				return nil, err
			}
			result.lowerInf, result.lowerOpen = false, false
		}
		return result, result.CheckInvariant()
	}
	err = result.SetFinite(&lower, &upper, false, false)
	return result, err
}
