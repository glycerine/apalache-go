package interval

// Source: src/math/interval/dep_intervals.cpp:69-87,119-149.

import "github.com/glycerine/g3/pkg/util/numeral"

func (v *DependentRationalInterval) SeparatedFromZeroLower() (bool, error) {
	if err := v.open(); err != nil {
		return false, err
	}
	if v.Value.lowerInf {
		return false, nil
	}
	negative, err := v.manager.Numbers.IsNeg(&v.Value.lower)
	if err != nil || negative {
		return false, err
	}
	zero, err := v.manager.Numbers.IsZero(&v.Value.lower)
	return !zero || v.Value.lowerOpen, err
}

func (v *DependentRationalInterval) SeparatedFromZeroUpper() (bool, error) {
	if err := v.open(); err != nil {
		return false, err
	}
	if v.Value.upperInf {
		return false, nil
	}
	positive, err := v.manager.Numbers.IsPos(&v.Value.upper)
	if err != nil || positive {
		return false, err
	}
	zero, err := v.manager.Numbers.IsZero(&v.Value.upper)
	return !zero || v.Value.upperOpen, err
}

func (v *DependentRationalInterval) IsEmpty() (bool, error) {
	if err := v.open(); err != nil {
		return false, err
	}
	if v.Value.lowerInf || v.Value.upperInf {
		return false, nil
	}
	comparison, err := v.manager.Numbers.Compare(&v.Value.lower, &v.Value.upper)
	if err != nil {
		return false, err
	}
	return comparison > 0 || comparison == 0 && (v.Value.lowerOpen || v.Value.upperOpen), nil
}

func (v *DependentRationalInterval) IsAbove(value *numeral.Rational) (bool, error) {
	if err := v.open(); err != nil {
		return false, err
	}
	if v.Value.lowerInf {
		return false, nil
	}
	comparison, err := v.manager.Numbers.Compare(value, &v.Value.lower)
	return comparison < 0 || comparison == 0 && v.Value.lowerOpen, err
}

func (v *DependentRationalInterval) IsBelow(value *numeral.Rational) (bool, error) {
	if err := v.open(); err != nil {
		return false, err
	}
	if v.Value.upperInf {
		return false, nil
	}
	comparison, err := v.manager.Numbers.Compare(&v.Value.upper, value)
	return comparison < 0 || comparison == 0 && v.Value.upperOpen, err
}
