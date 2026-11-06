package realclosure

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) firstNonzeroValue(values []*realValue, usePrem bool) (int, int32, error) {
	for i, value := range values {
		if value == nil {
			continue
		}
		sign, err := s.determineRankedRetainedValueSign(value, usePrem)
		if err != nil {
			return -1, 0, err
		}
		if sign != 0 {
			return i, sign, nil
		}
	}
	return -1, 0, arena.ErrInvalidSlot
}

func (s *realValueStore) determineInfinitesimalValueSign(value *realValue, usePrem bool) (sign int32, err error) {
	if s == nil || value == nil || value.root == nil || value.root.kind != realExtensionInfinitesimal || value.functionInterval == nil {
		return 0, arena.ErrInvalidSlot
	}
	numIdx, numSign, err := s.firstNonzeroValue(value.functionNumerator.coefficients, usePrem)
	if err != nil {
		return 0, err
	}
	denIdx, denSign, err := s.firstNonzeroValue(value.functionDenominator.coefficients, usePrem)
	if err != nil {
		return 0, err
	}
	sign = numSign * denSign
	if numIdx == 0 && denIdx == 0 {
		if err = s.setInfinitesimalLeadingRatio(value, s.rationalInitialPrecision(), usePrem); err != nil {
			return 0, err
		}
		return sign, nil
	}
	if numIdx == 0 {
		if denIdx == 0 {
			return 0, arena.ErrInvalidSlot
		}
		power := numeral.NewBinaryRational()
		defer func() { err = errors.Join(err, s.releaseSturmBinary(value.functionInterval.numbers, &power)) }()
		if err = setPositivePowerOfTwo(value.functionInterval.numbers, &power, s.infinitesimalPrecision()); err != nil {
			return 0, err
		}
		if sign > 0 {
			if err = value.functionInterval.numbers.Copy(&value.functionInterval.lower, &power); err != nil {
				return 0, err
			}
			value.functionInterval.lowerInf, value.functionInterval.lowerOpen = false, false
			if err = value.functionInterval.ResetUpper(); err != nil {
				return 0, err
			}
		} else {
			if err = value.functionInterval.ResetLower(); err != nil {
				return 0, err
			}
			negative := numeral.NewBinaryRational()
			if err = value.functionInterval.numbers.Copy(&negative, &power); err == nil {
				err = value.functionInterval.numbers.Neg(&negative)
			}
			if err == nil {
				err = value.functionInterval.numbers.Copy(&value.functionInterval.upper, &negative)
			}
			err = errors.Join(err, s.releaseSturmBinary(value.functionInterval.numbers, &negative))
			if err != nil {
				return 0, err
			}
			value.functionInterval.upperInf, value.functionInterval.upperOpen = false, false
		}
		return sign, nil
	}
	if denIdx != 0 {
		return 0, arena.ErrInvalidSlot
	}
	zero, upper := numeral.NewBinaryRational(), numeral.NewBinaryRational()
	defer func() {
		err = errors.Join(err, s.releaseSturmBinary(value.functionInterval.numbers, &zero))
		err = errors.Join(err, s.releaseSturmBinary(value.functionInterval.numbers, &upper))
	}()
	if err = value.functionInterval.numbers.SetInt32(&zero, 0); err != nil {
		return 0, err
	}
	if err = value.functionInterval.numbers.SetInt32Power(&upper, 1, s.rationalInitialPrecision()); err != nil {
		return 0, err
	}
	if sign > 0 {
		err = value.functionInterval.SetFinite(&zero, &upper, false, false)
	} else {
		lower := numeral.NewBinaryRational()
		if err = value.functionInterval.numbers.Copy(&lower, &upper); err == nil {
			err = value.functionInterval.numbers.Neg(&lower)
		}
		if err == nil {
			err = value.functionInterval.SetFinite(&lower, &zero, false, false)
		}
		cleanup := s.releaseSturmBinary(value.functionInterval.numbers, &lower)
		err = errors.Join(err, cleanup)
	}
	return sign, err
}

func (s *realValueStore) setInfinitesimalLeadingRatio(value *realValue, precision uint32, usePrem bool) (err error) {
	manager := value.functionInterval.numbers
	numerator, err := s.valueInterval(value.functionNumerator.coefficients[0], manager)
	if err != nil {
		return err
	}
	denominator, err := s.valueInterval(value.functionDenominator.coefficients[0], manager)
	if err != nil {
		return err
	}
	numeratorContainsZero, err := numerator.ContainsZero()
	if err != nil {
		return err
	}
	denominatorContainsZero, err := denominator.ContainsZero()
	if err != nil {
		return err
	}
	if numeratorContainsZero || denominatorContainsZero {
		return arena.ErrInvalidSlot
	}
	if numerator.lowerOpen && numerator.upperOpen && denominator.lowerOpen && denominator.upperOpen {
		return value.functionInterval.Div(numerator, denominator, precision)
	}
	tiny := numeral.NewBinaryRational()
	if err = manager.SetInt32Power(&tiny, 1, precision); err != nil {
		return errors.Join(err, s.releaseSturmBinary(manager, &tiny))
	}
	defer func() { err = errors.Join(err, s.releaseSturmBinary(manager, &tiny)) }()
	adjustedNumerator, adjustedDenominator := numerator, denominator
	var ownedNumerator, ownedDenominator *BinaryInterval
	if len(value.functionNumerator.coefficients) > 1 {
		plus, signErr := s.firstNonzeroSign(value.functionNumerator.coefficients[1:], usePrem)
		if signErr != nil {
			return signErr
		}
		ownedNumerator, err = NewBinaryInterval(manager)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, ownedNumerator.Close()) }()
		if err = s.addInfinitesimalInterval(numerator, plus > 0, &tiny, ownedNumerator); err != nil {
			return err
		}
		adjustedNumerator = ownedNumerator
	}
	if len(value.functionDenominator.coefficients) > 1 {
		plus, signErr := s.firstNonzeroSign(value.functionDenominator.coefficients[1:], usePrem)
		if signErr != nil {
			return signErr
		}
		ownedDenominator, err = NewBinaryInterval(manager)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, ownedDenominator.Close()) }()
		if err = s.addInfinitesimalInterval(denominator, plus > 0, &tiny, ownedDenominator); err != nil {
			return err
		}
		adjustedDenominator = ownedDenominator
	}
	return value.functionInterval.Div(adjustedNumerator, adjustedDenominator, precision)
}

func setPositivePowerOfTwo(manager *numeral.BinaryRationalManager, value *numeral.BinaryRational, exponent uint32) error {
	if manager == nil || value == nil {
		return arena.ErrInvalidSlot
	}
	if err := manager.SetInt32(value, 1); err != nil {
		return err
	}
	return manager.Z.Mul2k(&value.Num, exponent, &value.Num)
}

func (s *realValueStore) infinitesimalPrecision() uint32 {
	if s != nil && s.infPrecisionBound {
		return s.infPrecision
	}
	return 24
}

func (s *realValueStore) refineInfinitesimalPolynomialValue(value *realValue, precision uint32, usePrem bool) (refined bool, err error) {
	if s == nil || value == nil || value.root == nil || value.root.kind != realExtensionInfinitesimal || value.functionInterval == nil || len(value.functionNumerator.coefficients) == 0 || len(value.functionDenominator.coefficients) == 0 {
		return false, arena.ErrInvalidSlot
	}
	numIdx, _, err := s.firstNonzeroValue(value.functionNumerator.coefficients, usePrem)
	if err != nil {
		return false, err
	}
	denIdx, _, err := s.firstNonzeroValue(value.functionDenominator.coefficients, usePrem)
	if err != nil {
		return false, err
	}
	if numIdx != 0 || denIdx != 0 {
		return false, nil
	}
	manager := value.functionInterval.numbers
	numerator, err := NewBinaryInterval(manager)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, numerator.Close()) }()
	denominator, err := NewBinaryInterval(manager)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, denominator.Close()) }()
	for workPrecision := precision; ; workPrecision++ {
		if err = s.checkpoint(); err != nil {
			return false, err
		}
		for _, pair := range []struct {
			values []*realValue
			index  int
		}{{value.functionNumerator.coefficients, numIdx}, {value.functionDenominator.coefficients, denIdx}} {
			_, refineErr := s.refineAnyRetainedInterval(pair.values[pair.index], manager, workPrecision)
			if refineErr != nil {
				return false, refineErr
			}
		}
		numInterval, e := s.valueInterval(value.functionNumerator.coefficients[numIdx], manager)
		if e != nil {
			return false, e
		}
		denInterval, e := s.valueInterval(value.functionDenominator.coefficients[denIdx], manager)
		if e != nil {
			return false, e
		}
		numZero, e := numInterval.ContainsZero()
		if e != nil {
			return false, e
		}
		denZero, e := denInterval.ContainsZero()
		if e != nil {
			return false, e
		}
		if numZero || denZero {
			return false, nil
		}
		if numInterval.lowerOpen && numInterval.upperOpen && denInterval.lowerOpen && denInterval.upperOpen {
			e = value.functionInterval.Div(numInterval, denInterval, infinitesimalDivisionPrecision(precision))
		} else {
			power := uint32(workPrecision * 2)
			if workPrecision > ^uint32(0)/2 {
				power = ^uint32(0)
			}
			tiny := numeral.NewBinaryRational()
			if e = manager.SetInt32Power(&tiny, 1, power); e != nil {
				return false, errors.Join(e, s.releaseSturmBinary(manager, &tiny))
			}
			adjustedNumerator, adjustedDenominator := numInterval, denInterval
			var ownedNumerator, ownedDenominator *BinaryInterval
			if len(value.functionNumerator.coefficients) > numIdx+1 {
				plus, signErr := s.firstNonzeroSign(value.functionNumerator.coefficients[numIdx+1:], usePrem)
				if signErr != nil {
					return false, errors.Join(signErr, s.releaseSturmBinary(manager, &tiny))
				}
				ownedNumerator, e = NewBinaryInterval(manager)
				if e == nil {
					e = s.addInfinitesimalInterval(numInterval, plus > 0, &tiny, ownedNumerator)
				}
				adjustedNumerator = ownedNumerator
			}
			if e == nil && len(value.functionDenominator.coefficients) > denIdx+1 {
				plus, signErr := s.firstNonzeroSign(value.functionDenominator.coefficients[denIdx+1:], usePrem)
				if signErr != nil {
					e = signErr
				} else {
					ownedDenominator, e = NewBinaryInterval(manager)
					if e == nil {
						e = s.addInfinitesimalInterval(denInterval, plus > 0, &tiny, ownedDenominator)
					}
					adjustedDenominator = ownedDenominator
				}
			}
			if e == nil {
				e = value.functionInterval.Div(adjustedNumerator, adjustedDenominator, infinitesimalDivisionPrecision(precision))
			}
			if ownedNumerator != nil {
				e = errors.Join(e, ownedNumerator.Close())
			}
			if ownedDenominator != nil {
				e = errors.Join(e, ownedDenominator.Close())
			}
			e = errors.Join(e, s.releaseSturmBinary(manager, &tiny))
		}
		if e != nil {
			return false, e
		}
		precise, e := binaryIntervalPrecise(value.functionInterval, precision)
		if e != nil || precise {
			return precise, e
		}
		if workPrecision == ^uint32(0) {
			return false, arena.ErrBounds
		}
	}
}

func infinitesimalDivisionPrecision(precision uint32) uint32 {
	if precision < 1<<16 {
		return precision + 2
	}
	return precision
}

func (s *realValueStore) firstNonzeroSign(values []*realValue, usePrem bool) (int32, error) {
	_, sign, err := s.firstNonzeroValue(values, usePrem)
	return sign, err
}

func (s *realValueStore) addInfinitesimalInterval(source *BinaryInterval, plus bool, tiny *numeral.BinaryRational, target *BinaryInterval) (err error) {
	if err := target.SetFrom(source); err != nil {
		return err
	}
	target.lowerOpen, target.upperOpen = true, true
	manager := source.numbers
	if plus && !source.upperInf && !source.upperOpen {
		step, upper := numeral.NewBinaryRational(), numeral.NewBinaryRational()
		defer func() {
			err = errors.Join(err, s.releaseSturmBinary(manager, &step))
			err = errors.Join(err, s.releaseSturmBinary(manager, &upper))
		}()
		if err := manager.Copy(&step, tiny); err != nil {
			return err
		}
		for {
			if err := manager.Add(&source.upper, &step, &upper); err != nil {
				return err
			}
			oldPositive, err := manager.IsPos(&source.upper)
			if err != nil {
				return err
			}
			newPositive, err := manager.IsPos(&upper)
			if err != nil {
				return err
			}
			if oldPositive == newPositive {
				if err = manager.Copy(&target.upper, &upper); err != nil {
					return err
				}
				return nil
			}
			if err = manager.Div2(&step); err != nil {
				return err
			}
			if err = s.checkpoint(); err != nil {
				return err
			}
		}
	}
	if !plus && !source.lowerInf && !source.lowerOpen {
		step, lower := numeral.NewBinaryRational(), numeral.NewBinaryRational()
		defer func() {
			err = errors.Join(err, s.releaseSturmBinary(manager, &step))
			err = errors.Join(err, s.releaseSturmBinary(manager, &lower))
		}()
		if err := manager.Copy(&step, tiny); err != nil {
			return err
		}
		for {
			if err := manager.Sub(&source.lower, &step, &lower); err != nil {
				return err
			}
			oldPositive, err := manager.IsPos(&source.lower)
			if err != nil {
				return err
			}
			newPositive, err := manager.IsPos(&lower)
			if err != nil {
				return err
			}
			if oldPositive == newPositive {
				if err = manager.Copy(&target.lower, &lower); err != nil {
					return err
				}
				return nil
			}
			if err = manager.Div2(&step); err != nil {
				return err
			}
			if err = s.checkpoint(); err != nil {
				return err
			}
		}
	}
	return nil
}
