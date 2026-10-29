package simplex

// Source: src/smt/theory_arith_aux.h get_freedom_interval, random_update and
// remove_fixed_vars_from_base, specialized to the rational tableau.

import (
	"fmt"
	"math/big"

	z3rand "github.com/glycerine/g3/pkg/util/random"
)

type ArithmeticFreedomInterval struct {
	Lower, Upper           Infinitesimal
	LowerValid, UpperValid bool
	Multiple               big.Rat
}

func (s *Solver) arithmeticFixed(variable uint32) bool {
	x := &s.vars[variable]
	return x.lowerValid && x.upperValid && x.lower.Cmp(&x.upper) == 0
}

func (s *Solver) RemoveArithmeticFixedBases() error {
	for variable := uint32(0); variable < s.NumVariables(); variable++ {
		if !s.isBase(variable) || !s.arithmeticFixed(variable) {
			continue
		}
		entries, err := s.RowEntries(Row{s.vars[variable].row})
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Variable != variable && !s.arithmeticFixed(entry.Variable) {
				if err = s.pivot(variable, entry.Variable, new(big.Rat).Set(entry.Coefficient)); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

func (s *Solver) ArithmeticFreedom(variable uint32, integers map[uint32]bool) (ArithmeticFreedomInterval, bool, error) {
	var interval ArithmeticFreedomInterval
	interval.Multiple.SetInt64(1)
	if variable >= s.NumVariables() {
		return interval, false, fmt.Errorf("simplex: freedom variable out of range")
	}
	if s.isBase(variable) {
		return interval, false, nil
	}
	interval.Lower, interval.LowerValid = s.Lower(variable)
	interval.Upper, interval.UpperValid = s.Upper(variable)
	fixed := func() bool {
		return interval.LowerValid && interval.UpperValid && interval.Lower.Cmp(&interval.Upper) == 0
	}
	if fixed() {
		return interval, true, nil
	}
	for _, row := range s.matrix.ColumnRows(variable) {
		base := s.rowBase[row.id]
		raw, _ := s.matrix.coefficient(row, variable)
		coefficient := new(big.Rat).Quo(raw, &s.vars[base].baseCoeff)
		if integers[variable] && integers[base] && !coefficient.IsInt() {
			denominator := coefficient.Denom()
			gcd := new(big.Int).GCD(nil, nil, interval.Multiple.Num(), denominator)
			lcm := new(big.Int).Mul(new(big.Int).Quo(interval.Multiple.Num(), gcd), denominator)
			interval.Multiple.SetInt(lcm)
		}
		for _, lower := range []bool{true, false} {
			bound, valid := s.Upper(base)
			if lower {
				bound, valid = s.Lower(base)
			}
			if !valid {
				continue
			}
			value := s.Value(base)
			value.Sub(&bound)
			value.Quo(coefficient)
			current := s.Value(variable)
			value.Add(&current)
			isLower := lower == (coefficient.Sign() < 0)
			if isLower && (!interval.LowerValid || value.Cmp(&interval.Lower) > 0) {
				interval.Lower, interval.LowerValid = value, true
			} else if !isLower && (!interval.UpperValid || value.Cmp(&interval.Upper) < 0) {
				interval.Upper, interval.UpperValid = value, true
			}
			if fixed() {
				return interval, true, nil
			}
		}
	}
	return interval, true, nil
}

func arithmeticCeil(value *Infinitesimal) *big.Int {
	negative := value.Clone()
	negative.Neg()
	return new(big.Int).Neg(arithmeticFloor(&negative))
}

func (s *Solver) ArithmeticRandomUpdate(variable uint32, integers map[uint32]bool, random *z3rand.Generator) (bool, error) {
	if random == nil || variable >= s.NumVariables() {
		return false, fmt.Errorf("simplex: invalid random update")
	}
	if s.isBase(variable) || s.arithmeticFixed(variable) {
		return false, nil
	}
	interval, _, err := s.ArithmeticFreedom(variable, integers)
	if err != nil {
		return false, err
	}
	const randomRange = 10000
	if !interval.LowerValid && !interval.UpperValid {
		s.SetValue(variable, RationalInfinitesimal(new(big.Rat).SetInt64(int64(random.Bounded(randomRange+1)))))
		return true, nil
	}
	if integers[variable] {
		for _, lower := range []bool{true, false} {
			value, valid := &interval.Upper, interval.UpperValid
			if lower {
				value, valid = &interval.Lower, interval.LowerValid
			}
			if !valid {
				continue
			}
			integer := arithmeticFloor(value)
			if lower {
				integer = arithmeticCeil(value)
			}
			value.Main.SetInt(integer)
			value.Infinity.SetInt64(0)
			value.Epsilon.SetInt64(0)
			value.Quo(&interval.Multiple)
			integer = arithmeticFloor(value)
			if lower {
				integer = arithmeticCeil(value)
			}
			value.Main.Mul(new(big.Rat).SetInt(integer), &interval.Multiple)
		}
	}
	if interval.LowerValid && interval.UpperValid && interval.Lower.Cmp(&interval.Upper) >= 0 {
		return false, nil
	}
	var value Infinitesimal
	if !interval.UpperValid || !interval.LowerValid {
		delta := new(big.Rat).Mul(new(big.Rat).SetInt64(int64(random.Bounded(randomRange+1))), &interval.Multiple)
		value = interval.Lower.Clone()
		if !interval.LowerValid {
			value = interval.Upper.Clone()
			delta.Neg(delta)
		}
		step := RationalInfinitesimal(delta)
		value.Add(&step)
	} else if !integers[variable] {
		value = interval.Upper.Clone()
		value.Sub(&interval.Lower)
		value.Mul(big.NewRat(int64(random.Bounded(randomRange+1)), randomRange))
		value.Add(&interval.Lower)
	} else {
		width := new(big.Rat).Sub(&interval.Upper.Main, &interval.Lower.Main)
		width.Quo(width, &interval.Multiple)
		rangeSize := uint32(randomRange)
		if width.Cmp(big.NewRat(randomRange, 1)) < 0 {
			rangeSize = uint32(new(big.Int).Quo(width.Num(), width.Denom()).Uint64())
		}
		step := RationalInfinitesimal(new(big.Rat).Mul(&interval.Multiple, new(big.Rat).SetInt64(int64(random.Bounded(rangeSize+1)))))
		value = interval.Lower.Clone()
		value.Add(&step)
	}
	s.SetValue(variable, value)
	return true, nil
}
