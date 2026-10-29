package simplex

// Source: src/smt/theory_arith_aux.h init_gains, update_gains,
// pick_var_to_leave, move_to_bound and max_min. This is the concrete rational
// tableau path retained by nonlinear arithmetic (maintain_integrality=false).

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/util/control"
)

type ArithmeticOptStatus uint8

const (
	ArithmeticUnbounded ArithmeticOptStatus = iota
	ArithmeticAtBound
	ArithmeticOptimized
	ArithmeticBestEffort
)

type ArithmeticOptConfig struct {
	IntegerColumns map[uint32]bool
	SharedColumns  map[uint32]bool
	MaxEfforts     uint32
	Limit          *control.Reslimit
}

type ArithmeticOptResult struct {
	Status             ArithmeticOptStatus
	HasShared, Changed bool
	Value              Infinitesimal
	Bounds             []ArithmeticBoundUse
}

type arithmeticOptimizer struct {
	solver          *Solver
	config          ArithmeticOptConfig
	objective       uint32
	efforts         uint32
	shared, changed bool
}

func arithmeticMinusOne(value *Infinitesimal) bool {
	return value.Infinity.Sign() == 0 && value.Main.Cmp(big.NewRat(-1, 1)) == 0 && value.Epsilon.Sign() == 0
}
func arithmeticInteger(value *Infinitesimal) bool {
	return value.IsRational() && value.Main.IsInt()
}
func arithmeticSafeGain(min, max *Infinitesimal) bool {
	return arithmeticMinusOne(max) || min.Cmp(max) <= 0
}

func arithmeticFloor(value *Infinitesimal) *big.Int {
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(value.Main.Num(), value.Main.Denom(), remainder)
	if value.Main.Sign() < 0 && remainder.Sign() != 0 || value.Main.IsInt() && value.Epsilon.Sign() < 0 {
		quotient.Sub(quotient, big.NewInt(1))
	}
	return quotient
}

func arithmeticNormalizeGain(divisor *big.Rat, gain *Infinitesimal) {
	if divisor.Cmp(big.NewRat(-1, 1)) == 0 || arithmeticMinusOne(gain) {
		return
	}
	quotient := gain.Clone()
	quotient.Quo(divisor)
	gain.Main.Mul(new(big.Rat).SetInt(arithmeticFloor(&quotient)), divisor)
	gain.Infinity.SetInt64(0)
	gain.Epsilon.SetInt64(0)
}

func (o *arithmeticOptimizer) initGains(variable uint32, increase bool) (min, max Infinitesimal) {
	s := o.solver
	min.Main.SetInt64(-1)
	max.Main.SetInt64(-1)
	x := &s.vars[variable]
	if increase && x.upperValid {
		max = x.upper.Clone()
		max.Sub(&x.value)
	} else if !increase && x.lowerValid {
		max = x.value.Clone()
		max.Sub(&x.lower)
	}
	if o.config.IntegerColumns[variable] {
		min.Main.SetInt64(1)
	}
	return
}

// coefficient is normalized so the row reads base + coefficient*x + ...=0.
func (o *arithmeticOptimizer) updateGains(increase bool, base uint32, coefficient *big.Rat, min, max *Infinitesimal) bool {
	if !arithmeticSafeGain(min, max) {
		return false
	}
	x := &o.solver.vars[base]
	decreaseBase := increase == (coefficient.Sign() > 0)
	available := RationalInfinitesimal(big.NewRat(-1, 1))
	if decreaseBase && x.lowerValid {
		available = x.value.Clone()
		available.Sub(&x.lower)
		available.Quo(coefficient)
		available.Abs()
	} else if !decreaseBase && x.upperValid {
		available = x.upper.Clone()
		available.Sub(&x.value)
		available.Quo(coefficient)
		available.Abs()
	}
	if o.config.IntegerColumns[base] {
		denominator := coefficient.Denom()
		if denominator.Cmp(big.NewInt(1)) != 0 {
			if min.Main.Sign() < 0 {
				min.Main.SetInt(denominator)
			} else {
				gcd := new(big.Int).GCD(nil, nil, min.Main.Num(), denominator)
				lcm := new(big.Int).Mul(new(big.Int).Quo(min.Main.Num(), gcd), denominator)
				min.Main.SetInt(lcm)
			}
			min.Epsilon.SetInt64(0)
			arithmeticNormalizeGain(&min.Main, max)
		}
		if !arithmeticInteger(max) {
			max.Main.SetInt(arithmeticFloor(max))
			max.Infinity.SetInt64(0)
			max.Epsilon.SetInt64(0)
			arithmeticNormalizeGain(&min.Main, max)
		}
	}
	if arithmeticMinusOne(&available) {
		return false
	}
	if o.config.IntegerColumns[base] {
		available.Main.SetInt(arithmeticFloor(&available))
		available.Infinity.SetInt64(0)
		available.Epsilon.SetInt64(0)
		arithmeticNormalizeGain(&min.Main, &available)
	}
	if arithmeticMinusOne(max) || max.Cmp(&available) > 0 {
		max.Set(&available)
		return true
	}
	return false
}

func (o *arithmeticOptimizer) pickLeaving(variable uint32, increase bool) (base uint32, raw *big.Rat, min, max Infinitesimal, safe bool) {
	s := o.solver
	base = NullVariable
	min, max = o.initGains(variable, increase)
	o.shared = o.shared || o.config.SharedColumns[variable]
	if o.config.IntegerColumns[variable] && !arithmeticInteger(&s.vars[variable].value) {
		return
	}
	for _, row := range s.matrix.ColumnRows(variable) {
		current := s.rowBase[row.id]
		if current == o.objective {
			continue
		} // Source temporary row is not part of the tableau.
		coefficient, _ := s.matrix.coefficient(row, variable)
		normalized := new(big.Rat).Quo(coefficient, &s.vars[current].baseCoeff)
		if o.updateGains(increase, current, normalized, &min, &max) || base == NullVariable && !arithmeticMinusOne(&max) {
			base, raw = current, new(big.Rat).Set(coefficient)
		}
		o.shared = o.shared || o.config.SharedColumns[current]
	}
	safe = arithmeticSafeGain(&min, &max)
	return
}

func (o *arithmeticOptimizer) update(variable uint32, delta Infinitesimal) {
	if !delta.IsZero() {
		o.changed = true
		o.solver.updateValue(variable, &delta)
	}
}

func (o *arithmeticOptimizer) moveToBound(variable uint32, increase bool) {
	s := o.solver
	if o.config.IntegerColumns[variable] && !arithmeticInteger(&s.vars[variable].value) {
		o.efforts++
		return
	}
	min, max := o.initGains(variable, increase)
	for _, row := range s.matrix.ColumnRows(variable) {
		base := s.rowBase[row.id]
		if base == o.objective {
			continue
		}
		coefficient, _ := s.matrix.coefficient(row, variable)
		normalized := new(big.Rat).Quo(coefficient, &s.vars[base].baseCoeff)
		o.updateGains(increase, base, normalized, &min, &max)
		o.shared = o.shared || o.config.SharedColumns[base]
	}
	moved := false
	if arithmeticSafeGain(&min, &max) && !arithmeticMinusOne(&max) {
		if !increase {
			max.Neg()
		}
		o.update(variable, max)
		if min.Main.Sign() <= 0 || min.Main.Cmp(big.NewRat(1, 1)) == 0 {
			o.efforts++
		}
		moved = !max.IsZero()
	}
	if !moved {
		o.efforts++
	}
}

func (o *arithmeticOptimizer) optimize(maximize bool) (ArithmeticOptStatus, error) {
	s := o.solver
	status := ArithmeticOptimized
	stopped := false
	for o.efforts < o.config.MaxEfforts {
		if o.config.Limit != nil && !o.config.Limit.Inc() {
			stopped = true
			break
		}
		row := Row{s.vars[o.objective].row}
		entries, err := s.matrix.Entries(row)
		if err != nil {
			return ArithmeticBestEffort, err
		}
		enter, leave := NullVariable, NullVariable
		var raw *big.Rat
		var minGain, maxGain Infinitesimal
		increase, hasBound := false, false
		for _, entry := range entries {
			variable := entry.Variable
			if variable == o.objective {
				continue
			}
			coefficient := new(big.Rat).Neg(new(big.Rat).Quo(entry.Coefficient, &s.vars[o.objective].baseCoeff))
			inc := maximize == (coefficient.Sign() > 0)
			x := &s.vars[variable]
			if inc && x.upperValid || !inc && x.lowerValid {
				hasBound = true
			}
			if inc && s.atUpper(variable) || !inc && s.atLower(variable) {
				continue
			}
			candidate, candidateRaw, candidateMin, candidateMax, safe := o.pickLeaving(variable, inc)
			if !safe {
				hasBound = true
				o.efforts++
				continue
			}
			if candidate == NullVariable {
				leave, enter, increase, minGain, maxGain = candidate, variable, inc, candidateMin, candidateMax
				break
			}
			if candidateMax.Cmp(&maxGain) > 0 || candidateMax.IsZero() && (leave == NullVariable || candidate < leave) {
				leave, enter, raw, increase, minGain, maxGain = candidate, variable, candidateRaw, inc, candidateMin, candidateMax
			}
		}
		if !hasBound && leave == NullVariable && enter == NullVariable {
			o.shared = false
			o.efforts = 0
			status = ArithmeticUnbounded
			break
		}
		if enter == NullVariable {
			status = ArithmeticOptimized
			break
		}
		if minGain.Main.Sign() > 0 && minGain.Main.Cmp(big.NewRat(1, 1)) != 0 {
			o.efforts++
		}
		if leave == NullVariable {
			x := &s.vars[enter]
			if increase && x.upperValid || !increase && x.lowerValid {
				if maxGain.IsZero() {
					return ArithmeticBestEffort, nil
				}
				if !increase {
					maxGain.Neg()
				}
				o.update(enter, maxGain)
				continue
			}
			o.shared = false
			o.efforts = 0
			status = ArithmeticUnbounded
			break
		}
		x := &s.vars[enter]
		if x.lowerValid && x.upperValid && x.lower.Cmp(&x.upper) != 0 {
			width := x.upper.Clone()
			width.Sub(&x.lower)
			if width.Cmp(&maxGain) == 0 {
				if !increase {
					maxGain.Neg()
				}
				o.update(enter, maxGain)
				continue
			}
		}
		if raw == nil {
			return ArithmeticBestEffort, fmt.Errorf("simplex: arithmetic pivot has no coefficient")
		}
		// The source tableau normalizes every base coefficient to one.
		// Keep raw coefficients for the concrete matrix pivot and normalized
		// signs for the source gain and movement rules.
		pivotSign := raw.Sign() * s.vars[leave].baseCoeff.Sign()
		if err := s.pivot(leave, enter, raw); err != nil {
			return ArithmeticBestEffort, err
		}
		o.moveToBound(leave, increase == (pivotSign < 0))
	}
	if o.efforts > 0 || stopped {
		status = ArithmeticBestEffort
	}
	return status, nil
}

// ArithmeticMaxMin preserves the source's integer-gain and sharing guards.
// Its temporary objective is removed before returning. Every exact optimum
// retains its row explanation; sharing affects the caller's propagation timing.
func (s *Solver) ArithmeticMaxMin(variable uint32, maximize bool, config ArithmeticOptConfig) (result ArithmeticOptResult, err error) {
	if variable >= s.NumVariables() {
		return result, fmt.Errorf("simplex: arithmetic variable out of range")
	}
	result.Value = s.Value(variable)
	if maximize && s.atUpper(variable) || !maximize && s.atLower(variable) {
		result.Status = ArithmeticAtBound
		return result, nil
	}
	if config.MaxEfforts == 0 {
		config.MaxEfforts = 10
	}
	objective := s.NumVariables()
	_, err = s.AddRationalRow(objective, []uint32{objective, variable}, []big.Rat{*big.NewRat(1, 1), *big.NewRat(-1, 1)})
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, s.DeleteRow(objective)); result.Value = s.Value(variable) }()
	o := arithmeticOptimizer{solver: s, config: config, objective: objective}
	result.Status, err = o.optimize(maximize)
	result.HasShared, result.Changed = o.shared, o.changed
	if err == nil && result.Status == ArithmeticOptimized {
		row, basic := s.BasicRow(objective)
		if !basic {
			return result, fmt.Errorf("simplex: optimized objective is not basic")
		}
		entries, e := s.RowEntries(row)
		if e != nil {
			return result, e
		}
		for _, entry := range entries {
			if entry.Variable == objective {
				continue
			}
			coefficient := new(big.Rat).Neg(new(big.Rat).Quo(entry.Coefficient, &s.vars[objective].baseCoeff))
			result.Bounds = append(result.Bounds, ArithmeticBoundUse{Column: entry.Variable, Lower: !maximize == (coefficient.Sign() > 0), Coefficient: *coefficient})
		}
	}
	return result, err
}

// MoveArithmeticNonBasesToBounds implements the assignment preparation from
// theory_arith_int.h used by process_non_linear before max_min_nl_vars.
func (s *Solver) MoveArithmeticNonBasesToBounds(integerColumns map[uint32]bool) {
	for variable := uint32(0); variable < s.NumVariables(); variable++ {
		if s.isBase(variable) {
			continue
		}
		x := &s.vars[variable]
		switch {
		case x.lowerValid && x.upperValid:
			if !s.atLower(variable) && !s.atUpper(variable) {
				s.SetValue(variable, x.lower.Clone())
			}
		case x.lowerValid:
			if !s.atLower(variable) {
				s.SetValue(variable, x.lower.Clone())
			}
		case x.upperValid:
			if !s.atUpper(variable) {
				s.SetValue(variable, x.upper.Clone())
			}
		case integerColumns[variable] && !arithmeticInteger(&x.value):
			s.SetValue(variable, RationalInfinitesimal(new(big.Rat).SetInt(arithmeticFloor(&x.value))))
		}
	}
}
