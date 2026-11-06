package simplex

// Source: src/smt/theory_arith_int.h gcd_test(row) and ext_gcd_test.

import "math/big"

type ArithmeticGCDResult uint8

const (
	ArithmeticGCDPass ArithmeticGCDResult = iota
	ArithmeticGCDConflict
	ArithmeticExtendedGCDConflict
)

// ArithmeticGCDRow uses fixed bounds, never a possibly stale model value,
// to eliminate fixed columns from an integer row. Nonfixed real columns
// disable the test, as in the source mixed-arithmetic implementation.
func (s *Solver) ArithmeticGCDRow(row Row, integers map[uint32]bool) (ArithmeticGCDResult, error) {
	return s.arithmeticGCDRow(row, integers, nil)
}

func (s *Solver) arithmeticGCDRow(row Row, integers map[uint32]bool, explanation *[]ArithmeticBoundUse) (ArithmeticGCDResult, error) {
	entries, err := s.RowEntries(row)
	if err != nil {
		return ArithmeticGCDPass, err
	}
	base, err := s.BaseCoefficient(row)
	if err != nil {
		return ArithmeticGCDPass, err
	}
	coefficients := make([]big.Rat, len(entries))
	lcm := big.NewInt(1)
	for i, entry := range entries {
		coefficients[i].Quo(entry.Coefficient, base)
		denominator := coefficients[i].Denom()
		gcd := new(big.Int).GCD(nil, nil, lcm, denominator)
		lcm.Mul(new(big.Int).Quo(lcm, gcd), denominator)
	}
	var constant big.Rat
	var fixedBounds, intervalBounds []ArithmeticBoundUse
	var gcd, least big.Int
	leastBounded := false
	for i, entry := range entries {
		coefficient := new(big.Rat).Mul(&coefficients[i], new(big.Rat).SetInt(lcm))
		column := entry.Variable
		if s.arithmeticFixed(column) {
			lower := &s.vars[column].lower
			// Source fixed columns have rational values. Guard the invariant
			// for callers of the concrete Go infinitesimal tableau.
			if !lower.IsRational() {
				return ArithmeticGCDPass, nil
			}
			constant.Add(&constant, new(big.Rat).Mul(coefficient, &lower.Main))
			if explanation != nil {
				fixedBounds = append(fixedBounds,
					ArithmeticBoundUse{column, true, *new(big.Rat).Set(&coefficients[i])},
					ArithmeticBoundUse{column, false, *new(big.Rat).Set(&coefficients[i])})
			}
			continue
		}
		if !integers[column] {
			return ArithmeticGCDPass, nil
		}
		absolute := new(big.Int).Abs(coefficient.Num())
		bounded := s.vars[column].lowerValid && s.vars[column].upperValid
		if gcd.Sign() == 0 {
			gcd.Set(absolute)
			least.Set(absolute)
			leastBounded = bounded
		} else {
			gcd.GCD(nil, nil, &gcd, absolute)
			switch absolute.Cmp(&least) {
			case -1:
				least.Set(absolute)
				leastBounded = bounded
			case 0:
				leastBounded = leastBounded && bounded
			}
		}
	}
	if gcd.Sign() == 0 {
		return ArithmeticGCDPass, nil
	}
	if !new(big.Rat).Quo(&constant, new(big.Rat).SetInt(&gcd)).IsInt() {
		if explanation != nil {
			*explanation = fixedBounds
		}
		return ArithmeticGCDConflict, nil
	}
	if !leastBounded {
		return ArithmeticGCDPass, nil
	}
	// Terms with the least coefficient contribute a bounded interval;
	// all other terms contribute multiples of their coefficient GCD.
	lower, upper := new(big.Rat).Set(&constant), new(big.Rat).Set(&constant)
	gcd.SetInt64(0)
	for i, entry := range entries {
		column := entry.Variable
		if s.arithmeticFixed(column) {
			continue
		}
		coefficient := new(big.Rat).Mul(&coefficients[i], new(big.Rat).SetInt(lcm))
		absolute := new(big.Int).Abs(coefficient.Num())
		if absolute.Cmp(&least) != 0 {
			gcd.GCD(nil, nil, &gcd, absolute)
			continue
		}
		lo, hi := &s.vars[column].lower.Main, &s.vars[column].upper.Main
		if coefficient.Sign() < 0 {
			lo, hi = hi, lo
		}
		lower.Add(lower, new(big.Rat).Mul(coefficient, lo))
		upper.Add(upper, new(big.Rat).Mul(coefficient, hi))
		if explanation != nil {
			intervalBounds = append(intervalBounds,
				ArithmeticBoundUse{column, true, *new(big.Rat).Set(&coefficients[i])},
				ArithmeticBoundUse{column, false, *new(big.Rat).Set(&coefficients[i])})
		}
	}
	if gcd.Sign() == 0 {
		return ArithmeticGCDPass, nil
	}
	lower.Quo(lower, new(big.Rat).SetInt(&gcd))
	upper.Quo(upper, new(big.Rat).SetInt(&gcd))
	lo, hi := RationalInfinitesimal(lower), RationalInfinitesimal(upper)
	if arithmeticFloor(&hi).Cmp(arithmeticCeil(&lo)) < 0 {
		if explanation != nil {
			*explanation = append(intervalBounds, fixedBounds...)
		}
		return ArithmeticExtendedGCDConflict, nil
	}
	return ArithmeticGCDPass, nil
}

// ArithmeticGCD scans fractional integer basic rows for the deferred source check.
func (s *Solver) ArithmeticGCD(integers map[uint32]bool) (result ArithmeticGCDResult, checks uint64, err error) {
	result, checks, _, err = s.ArithmeticGCDWithExplanation(integers)
	return
}

// ArithmeticGCDWithExplanation returns the endpoint uses in source order:
// least-coefficient intervals first (extended test), then fixed columns.
func (s *Solver) ArithmeticGCDWithExplanation(integers map[uint32]bool) (result ArithmeticGCDResult, checks uint64, bounds []ArithmeticBoundUse, err error) {
	for _, base := range s.rowBase {
		if base == NullVariable || !integers[base] {
			continue
		}
		if arithmeticInteger(&s.vars[base].value) {
			continue
		}
		checks++
		result, err = s.arithmeticGCDRow(Row{s.vars[base].row}, integers, &bounds)
		if err != nil || result != ArithmeticGCDPass {
			return
		}
	}
	return
}
