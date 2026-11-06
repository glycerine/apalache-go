package simplex

// Source: src/smt/theory_arith_int.h patch_int_infeasible_vars and
// fix_non_base_vars. Assignment updates preserve the canonical row equations.

import "math/big"

func (s *Solver) PatchArithmeticIntegerValues(integers map[uint32]bool) (bool, error) {
	changed := false
	for variable := uint32(0); variable < s.NumVariables(); variable++ {
		if s.isBase(variable) {
			continue
		}
		interval, _, err := s.ArithmeticFreedom(variable, integers)
		if err != nil {
			return changed, err
		}
		value := s.Value(variable)
		if interval.Multiple.Cmp(big.NewRat(1, 1)) == 0 && arithmeticInteger(&value) {
			continue
		}
		if new(big.Rat).Quo(&value.Main, &interval.Multiple).IsInt() {
			continue
		}
		for _, lower := range []bool{true, false} {
			bound, valid := &interval.Upper, interval.UpperValid
			if lower {
				bound, valid = &interval.Lower, interval.LowerValid
			}
			if !valid {
				continue
			}
			integer := arithmeticFloor(bound)
			if lower {
				integer = arithmeticCeil(bound)
			}
			bound.Main.SetInt(integer)
			bound.Infinity.SetInt64(0)
			bound.Epsilon.SetInt64(0)
			bound.Quo(&interval.Multiple)
			integer = arithmeticFloor(bound)
			if lower {
				integer = arithmeticCeil(bound)
			}
			bound.Main.Mul(new(big.Rat).SetInt(integer), &interval.Multiple)
		}
		if interval.LowerValid && interval.UpperValid && interval.Lower.Cmp(&interval.Upper) > 0 {
			continue
		}
		next := RationalInfinitesimal(new(big.Rat))
		if interval.LowerValid {
			next = interval.Lower.Clone()
		} else if interval.UpperValid {
			next = interval.Upper.Clone()
		}
		if value.Cmp(&next) != 0 {
			s.SetValue(variable, next)
			changed = true
		}
	}
	return changed, nil
}

func (s *Solver) FixArithmeticIntegerNonBases(integers map[uint32]bool) bool {
	changed := false
	for variable := uint32(0); variable < s.NumVariables(); variable++ {
		if s.isBase(variable) || !integers[variable] {
			continue
		}
		value := s.Value(variable)
		if arithmeticInteger(&value) {
			continue
		}
		s.SetValue(variable, RationalInfinitesimal(new(big.Rat).SetInt(arithmeticFloor(&value))))
		changed = true
	}
	return changed
}
