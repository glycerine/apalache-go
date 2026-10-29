package simplex

// Source: src/smt/theory_arith_int.h integer base selection and mk_gomory_cut.

import (
	"fmt"
	"math/big"

	z3rand "github.com/glycerine/g3/pkg/util/random"
)

type ArithmeticCutTerm struct {
	Column      uint32
	Coefficient big.Rat
}

type ArithmeticGomoryCut struct {
	Terms []ArithmeticCutTerm
	// Bounds retain the endpoint coefficients before polynomial denominator
	// clearing or single-term normalization, as in the source justification.
	Bounds   []ArithmeticBoundUse
	Bound    big.Rat
	Lower    bool
	Conflict bool
}

func (s *Solver) ArithmeticIntegerBase(integers map[uint32]bool, random *z3rand.Generator) (uint32, bool) {
	target, choices := NullVariable, uint32(0)
	var best big.Rat
	for _, base := range s.rowBase {
		if base == NullVariable || !integers[base] || arithmeticInteger(&s.vars[base].value) {
			continue
		}
		x := &s.vars[base]
		if !x.lowerValid || !x.upperValid {
			continue
		}
		width := new(big.Rat).Sub(&x.upper.Main, &x.lower.Main)
		if width.Cmp(big.NewRat(1024, 1)) > 0 {
			continue
		}
		if target == NullVariable || width.Cmp(&best) < 0 {
			target, choices = base, 1
			best.Set(width)
		} else if width.Cmp(&best) == 0 {
			choices++
			if random.Bounded(choices) == 0 {
				target = base
			}
		}
	}
	if target != NullVariable {
		return target, true
	}
	for _, base := range s.rowBase {
		if base == NullVariable || !integers[base] || arithmeticInteger(&s.vars[base].value) {
			continue
		}
		choices++
		if target == NullVariable || random.Bounded(choices) == 0 {
			target = base
		}
	}
	return target, target != NullVariable
}

func arithmeticFraction(value *big.Rat) *big.Rat {
	inf := RationalInfinitesimal(value)
	return new(big.Rat).Sub(value, new(big.Rat).SetInt(arithmeticFloor(&inf)))
}

func (s *Solver) ArithmeticGomory(row Row, integers map[uint32]bool) (cut ArithmeticGomoryCut, ready bool, err error) {
	base, err := s.BaseVariable(row)
	if err != nil {
		return cut, false, err
	}
	if !integers[base] || arithmeticInteger(&s.vars[base].value) {
		return cut, false, fmt.Errorf("simplex: Gomory target is not a fractional integer base")
	}
	entries, err := s.RowEntries(row)
	if err != nil {
		return cut, false, err
	}
	for _, entry := range entries {
		if entry.Variable != base && ((!s.atLower(entry.Variable) && !s.atUpper(entry.Variable)) || !s.vars[entry.Variable].value.IsRational()) {
			return cut, false, nil
		}
	}
	f0 := arithmeticFraction(&s.vars[base].value.Main)
	if f0.Sign() == 0 {
		return cut, false, nil
	}
	oneMinus := new(big.Rat).Sub(big.NewRat(1, 1), f0)
	cut.Bound.SetInt64(1)
	cut.Lower = true
	lcm := big.NewInt(1)
	integerCount := 0
	for _, entry := range entries {
		column := entry.Variable
		if column == base {
			continue
		}
		a := new(big.Rat).Neg(new(big.Rat).Quo(entry.Coefficient, &s.vars[base].baseCoeff))
		coefficient := new(big.Rat)
		lower := s.atLower(column)
		if !integers[column] {
			if lower {
				if a.Sign() > 0 {
					coefficient.Quo(a, oneMinus)
				} else {
					coefficient.Neg(new(big.Rat).Quo(a, f0))
				}
			} else {
				if a.Sign() > 0 {
					coefficient.Neg(new(big.Rat).Quo(a, f0))
				} else {
					coefficient.Quo(a, oneMinus)
				}
			}
		} else {
			integerCount++
			fraction := arithmeticFraction(a)
			if fraction.Sign() == 0 {
				continue
			}
			if lower {
				if fraction.Cmp(oneMinus) <= 0 {
					coefficient.Quo(fraction, oneMinus)
				} else {
					coefficient.Quo(new(big.Rat).Sub(big.NewRat(1, 1), fraction), f0)
				}
			} else {
				if fraction.Cmp(f0) <= 0 {
					coefficient.Quo(fraction, f0)
				} else {
					coefficient.Quo(new(big.Rat).Sub(big.NewRat(1, 1), fraction), oneMinus)
				}
				coefficient.Neg(coefficient)
			}
			denominator := coefficient.Denom()
			gcd := new(big.Int).GCD(nil, nil, lcm, denominator)
			lcm.Mul(new(big.Int).Quo(lcm, gcd), denominator)
		}
		bound := &s.vars[column].upper.Main
		if lower {
			bound = &s.vars[column].lower.Main
		}
		cut.Bound.Add(&cut.Bound, new(big.Rat).Mul(coefficient, bound))
		cut.Bounds = append(cut.Bounds, ArithmeticBoundUse{column, lower, *new(big.Rat).Set(coefficient)})
		cut.Terms = append(cut.Terms, ArithmeticCutTerm{column, *coefficient})
	}
	if len(cut.Terms) == 0 {
		cut.Conflict = true
		return cut, true, nil
	}
	if len(cut.Terms) == 1 {
		term := &cut.Terms[0]
		cut.Bound.Quo(&cut.Bound, &term.Coefficient)
		cut.Lower = term.Coefficient.Sign() > 0
		if integers[term.Column] && !cut.Bound.IsInt() {
			bound := RationalInfinitesimal(&cut.Bound)
			integer := arithmeticFloor(&bound)
			if cut.Lower {
				integer = arithmeticCeil(&bound)
			}
			cut.Bound.SetInt(integer)
		}
		term.Coefficient.SetInt64(1)
	} else if integerCount > 0 {
		denominator := cut.Bound.Denom()
		gcd := new(big.Int).GCD(nil, nil, lcm, denominator)
		lcm.Mul(new(big.Int).Quo(lcm, gcd), denominator)
		scale := new(big.Rat).SetInt(lcm)
		for i := range cut.Terms {
			cut.Terms[i].Coefficient.Mul(&cut.Terms[i].Coefficient, scale)
		}
		cut.Bound.Mul(&cut.Bound, scale)
	}
	return cut, true, nil
}
