package arith

// Source: src/tactic/arith/bound_propagator.cpp:361-427,465-744.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *BoundEquationStore) relevantBound(x uint32, candidate float64, lower bool) (bool, error) {
	v := &s.variables[x]
	b := v.upper
	if lower {
		b = v.lower
	}
	if err := s.traceRelevantBound(x, candidate, lower, b); err != nil {
		return false, err
	}
	if b == nil {
		return true, nil
	}
	bounded := v.lower != nil && v.upper != nil
	intervalSize := 0.0
	if bounded {
		intervalSize = v.upper.approx - v.lower.approx
	}
	if !v.integer {
		absK := b.approx
		if absK < 0 {
			absK -= absK // preserve the source branch, including its zero result
		}
		improvement := s.threshold * math.Max(absK, 1)
		if bounded {
			improvement = s.threshold * math.Max(math.Min(intervalSize, absK), 1)
		}
		if lower && candidate <= b.approx+improvement || !lower && candidate >= b.approx-improvement {
			s.traceBoundSmallImprovement(candidate, b.approx, lower)
			return false, nil
		}
	} else if lower && candidate < b.approx+1 || !lower && candidate > b.approx-1 {
		return false, nil
	}
	if bounded && intervalSize <= s.smallInterval {
		return true, nil
	}
	if lower {
		return v.lowerRefinements < s.maxRefinements, nil
	}
	return v.upperRefinements < s.maxRefinements, nil
}

// propagateExact recomputes a source candidate using rationals after the
// approximate relevance check. Every other row variable must have its bound.
func (s *BoundEquationStore) propagateExact(index uint32, position int, lower bool) (accepted bool, err error) {
	eq := s.constraints[index].eq
	z := s.manager.Integers()
	negative, err := z.IsNeg(&eq.as[position])
	if err != nil {
		return false, err
	}
	k := numeral.NewRational()
	defer func() { err = errors.Join(err, s.manager.Del(&k)) }()
	strict := false
	for j, x := range eq.xs {
		if j == position {
			continue
		}
		otherNegative, e := z.IsNeg(&eq.as[j])
		if e != nil {
			return false, e
		}
		var b *equationBound
		if lower {
			if otherNegative == negative {
				b = s.variables[x].upper
			} else {
				b = s.variables[x].lower
			}
		} else if otherNegative == negative {
			b = s.variables[x].lower
		} else {
			b = s.variables[x].upper
		}
		if b == nil {
			return false, arena.ErrInvalidSlot
		}
		if lower {
			if err = s.traceLowerStepInput(&k, b, &eq.as[j]); err != nil {
				return false, err
			}
		}
		strict = strict || b.strict
		if err = s.manager.AddMulInt(&k, &eq.as[j], &b.value, &k); err != nil {
			return false, err
		}
	}
	if lower {
		if err = s.traceLowerStepSum(&k); err != nil {
			return false, err
		}
	}
	if err = s.manager.Neg(&k); err != nil {
		return false, err
	}
	if err = s.manager.DivInt(&k, &eq.as[position], &k); err != nil {
		return false, err
	}
	if err = s.tracePropagationStep(eq, eq.xs[position], &k, strict, lower); err != nil {
		return false, err
	}
	return s.assertBound(eq.xs[position], &k, strict, lower, true)
}

func (s *BoundEquationStore) propagateEquation(index uint32) (bool, error) {
	if uint64(index) >= uint64(len(s.constraints)) {
		return false, arena.ErrBounds
	}
	c := &s.constraints[index]
	if c.dead {
		return false, nil
	}
	eq := c.eq
	s.tracePropagatingEquation(eq)
	llPosition, uuPosition := -1, -1
	llFailed, uuFailed := false, false
	ll, uu := 0.0, 0.0
	for i, x := range eq.xs {
		a := eq.approx[i]
		v := &s.variables[x]
		llBound, uuBound := v.upper, v.lower
		if a < 0 {
			llBound, uuBound = v.lower, v.upper
		}
		if !llFailed {
			if llBound == nil {
				if llPosition == -1 {
					llPosition = i
				} else {
					llFailed = true
				}
			} else {
				ll -= a * llBound.approx
			}
		}
		if !uuFailed {
			if uuBound == nil {
				if uuPosition == -1 {
					uuPosition = i
				} else {
					uuFailed = true
				}
			} else {
				uu -= a * uuBound.approx
			}
		}
		if llFailed && uuFailed {
			return false, nil
		}
	}
	propagated := false
	try := func(position int, candidate float64, lower bool) error {
		x := eq.xs[position]
		relevant, e := s.relevantBound(x, candidate, lower)
		if e != nil {
			return e
		}
		if !relevant {
			return nil
		}
		accepted, err := s.propagateExact(index, position, lower)
		if accepted {
			propagated = true
		}
		return err
	}
	if llPosition == -1 || uuPosition == -1 {
		for i, x := range eq.xs {
			a := eq.approx[i]
			v := &s.variables[x]
			lowerBound, upperBound := v.lower, v.upper
			if llPosition == -1 {
				if a > 0 {
					if err := try(i, (ll+a*upperBound.approx)/a, true); err != nil {
						return propagated, err
					}
				} else if err := try(i, (ll+a*lowerBound.approx)/a, false); err != nil {
					return propagated, err
				}
			}
			if uuPosition == -1 {
				if a > 0 {
					if err := try(i, (uu+a*lowerBound.approx)/a, false); err != nil {
						return propagated, err
					}
				} else if err := try(i, (uu+a*upperBound.approx)/a, true); err != nil {
					return propagated, err
				}
			}
		}
	}
	if !llFailed && llPosition != -1 {
		a := eq.approx[llPosition]
		if err := try(llPosition, ll/a, a > 0); err != nil {
			return propagated, err
		}
	}
	if !uuFailed && uuPosition != -1 {
		a := eq.approx[uuPosition]
		if err := try(uuPosition, uu/a, a < 0); err != nil {
			return propagated, err
		}
	}
	return propagated, nil
}

// Propagate visits watched equations for each newly published bound, following
// the source timestamp gate. The timestamp markers are reset after the pass.
func (s *BoundEquationStore) Propagate() (err error) {
	if err = s.open(); err != nil {
		return err
	}
	toReset := make([]uint32, 0)
	defer func() {
		for _, index := range toReset {
			s.constraints[index].timestamp = 0
		}
	}()
	for s.qhead < len(s.trail) {
		if s.conflict != noEquationPosition {
			break
		}
		event := s.trail[s.qhead]
		v := &s.variables[event.x]
		b := v.upper
		if event.lower {
			b = v.lower
		}
		if b == nil {
			s.qhead++
			continue
		}
		s.tracePropagatingVariable(event.x)
		s.qhead++
		for _, index := range v.watches {
			c := &s.constraints[index]
			if b.timestamp < c.timestamp {
				continue
			}
			if c.timestamp == 0 {
				toReset = append(toReset, index)
			}
			c.timestamp = s.timestamp
			if _, err = s.propagateEquation(index); err != nil {
				return err
			}
		}
	}
	return nil
}
