package arith

// Source: src/tactic/arith/bound_propagator.cpp:270-343,430-462,760-781,865-900.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// equationBound owns one rational value. Earlier bounds remain on the trail
// until the owner closes.
type equationBound struct {
	value     numeral.Rational
	approx    float64
	strict    bool
	x         uint32
	lower     bool
	timestamp uint32
	prev      *equationBound
}

func (s *BoundEquationStore) AssertLower(x uint32, k *numeral.Rational, strict bool) (bool, error) {
	return s.assertBound(x, k, strict, true, false)
}

func (s *BoundEquationStore) AssertUpper(x uint32, k *numeral.Rational, strict bool) (bool, error) {
	return s.assertBound(x, k, strict, false, false)
}

func (s *BoundEquationStore) assertBound(x uint32, k *numeral.Rational, strict, lower, derived bool) (accepted bool, err error) {
	if err = s.open(); err != nil {
		return false, err
	}
	if uint64(x) >= uint64(len(s.variables)) || !s.variables[x].active || k == nil {
		return false, arena.ErrBounds
	}
	m := s.manager
	working := numeral.NewRational()
	defer func() { err = errors.Join(err, m.Del(&working)) }()
	if err = m.Copy(&working, k); err != nil {
		return false, err
	}
	v := &s.variables[x]
	if v.integer {
		integer, e := m.IsInt(&working)
		if e != nil {
			return false, e
		}
		if lower {
			if integer && strict {
				if err = m.Inc(&working); err != nil {
					return false, err
				}
			} else if !integer {
				if err = m.Ceil(&working, &working); err != nil {
					return false, err
				}
			}
		} else {
			if integer && strict {
				if err = m.Dec(&working); err != nil {
					return false, err
				}
			} else if !integer {
				if err = m.Floor(&working, &working); err != nil {
					return false, err
				}
			}
		}
		strict = false
	}
	if err = s.traceBoundValue("bound_propagator_detail", lower, x, &working, strict, false); err != nil {
		return false, err
	}
	old := v.upper
	if lower {
		old = v.lower
	}
	if old != nil {
		cmp, e := m.Compare(&working, &old.value)
		if e != nil {
			return false, e
		}
		if (lower && cmp < 0) || (!lower && cmp > 0) || (cmp == 0 && (old.strict || !strict)) {
			if derived {
				s.traceBoundFalseAlarm(lower)
				s.falseAlarms++
			}
			return false, nil
		}
	}
	if derived {
		if lower {
			if err = s.traceBoundValue("bound_propagator_derived", lower, x, &working, strict, true); err != nil {
				return false, err
			}
		}
		s.propagations++
		if !lower {
			if err = s.traceBoundValue("bound_propagator_derived", lower, x, &working, strict, true); err != nil {
				return false, err
			}
		}
	}
	approx, e := m.Float64(&working)
	if e != nil {
		return false, e
	}
	if err = s.traceBoundApprox(lower, x, &working, approx, false); err != nil {
		return false, err
	}
	if lower {
		approx = 0.000001 * math.Floor(approx*1000000+0.0000001)
	} else {
		approx = 0.000001 * math.Ceil(approx*1000000-0.0000001)
	}
	if err = s.traceBoundApprox(lower, x, &working, approx, true); err != nil {
		return false, err
	}
	b := &equationBound{value: numeral.NewRational(), approx: approx,
		strict: strict, x: x, lower: lower, timestamp: s.timestamp, prev: old}
	if err = m.Copy(&b.value, &working); err != nil {
		return false, errors.Join(err, m.Del(&b.value))
	}
	s.timestamp++
	if lower {
		v.lower = b
		v.lowerRefinements++
	} else {
		v.upper = b
		v.upperRefinements++
	}
	s.trail = append(s.trail, b)
	if s.conflict == noEquationPosition && v.lower != nil && v.upper != nil {
		cmp, e := m.Compare(&v.lower.value, &v.upper.value)
		if e != nil {
			return true, e
		}
		if cmp > 0 || (cmp == 0 && (v.lower.strict || v.upper.strict)) {
			s.conflict = x
			s.traceBoundConflict(x)
		}
	}
	return true, nil
}

func (s *BoundEquationStore) ConflictVar() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.conflict, nil
}

// Lower and Upper copy a current exact bound to the caller's owned rational.
func (s *BoundEquationStore) Lower(x uint32, out *numeral.Rational) (found, strict bool, timestamp uint32, err error) {
	return s.copyBound(x, out, true)
}

func (s *BoundEquationStore) Upper(x uint32, out *numeral.Rational) (found, strict bool, timestamp uint32, err error) {
	return s.copyBound(x, out, false)
}

func (s *BoundEquationStore) copyBound(x uint32, out *numeral.Rational, lower bool) (found, strict bool, timestamp uint32, err error) {
	if err = s.open(); err != nil {
		return
	}
	if uint64(x) >= uint64(len(s.variables)) || out == nil {
		err = arena.ErrBounds
		return
	}
	b := s.variables[x].upper
	if lower {
		b = s.variables[x].lower
	}
	if b == nil {
		return
	}
	err = s.manager.Copy(out, &b.value)
	return err == nil, b.strict, b.timestamp, err
}

// PolynomialBound is the concrete rational get_bound instantiation. Missing
// required bounds reset out and report an unbounded result.
func (s *BoundEquationStore) PolynomialBound(as []numeral.Rational, xs []uint32, lower bool, out *numeral.Rational) (found, strict bool, err error) {
	if err = s.open(); err != nil {
		return
	}
	if len(as) != len(xs) || out == nil {
		err = arena.ErrInvalidSlot
		return
	}
	if err = s.manager.Reset(out); err != nil {
		return
	}
	for i, x := range xs {
		zero, e := s.manager.IsZero(&as[i])
		if e != nil {
			err = e
			return
		}
		if zero {
			continue
		}
		if uint64(x) >= uint64(len(s.variables)) {
			err = arena.ErrBounds
			return
		}
		negative, e := s.manager.IsNeg(&as[i])
		if e != nil {
			err = e
			return
		}
		b := s.variables[x].lower
		if negative == lower {
			b = s.variables[x].upper
		}
		if b == nil {
			err = s.manager.Reset(out)
			return
		}
		strict = strict || b.strict
		if err = s.manager.AddMul(out, &as[i], &b.value, out); err != nil {
			return
		}
	}
	return true, strict, nil
}
