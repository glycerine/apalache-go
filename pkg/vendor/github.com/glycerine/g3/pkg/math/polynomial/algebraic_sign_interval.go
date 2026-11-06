package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1965-1976,2013-2068.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicRootIntervals struct {
	owner    *AlgebraicRootCore
	values   PolynomialAlgebraicEvaluation
	borrowed map[uint32]*algebraicRecord
}

func (v *algebraicRootIntervals) Manager() (*unumeral.BinaryIntervalManager, error) {
	return v.owner.config.Intervals, nil
}
func (v *algebraicRootIntervals) Contains(x uint32) (bool, error) {
	has, e := v.values.Contains(x)
	if e != nil || !has {
		return has, e
	}
	a, e := v.values.Algebraic(x)
	if e != nil {
		return false, e
	}
	l, e := v.owner.location(a)
	return l != nil && l.kind == 1, e
}
func (v *algebraicRootIntervals) Interval(x uint32) (*unumeral.BinaryInterval, error) {
	a, e := v.values.Algebraic(x)
	if e != nil {
		return nil, e
	}
	if e = v.owner.assertBinaryRoot(a, 1973, "!v.is_basic()"); e != nil {
		return nil, e
	}
	l, e := v.owner.location(a)
	if e != nil {
		return nil, e
	}
	if l == nil || l.kind != 1 {
		return nil, ErrAlgebraicOwner
	}
	if v.borrowed == nil {
		v.borrowed = make(map[uint32]*algebraicRecord)
	}
	r := v.borrowed[x]
	if r == nil {
		r = &algebraicRecord{}
		v.borrowed[x] = r
	}
	*r, e = v.owner.load(l)
	if e != nil {
		return nil, e
	}
	return &r.interval, nil
}

type algebraicSignIntervalScope struct {
	sourceStop                   error
	interval                     unumeral.BinaryInterval
	evaluation                   *MultivariatePolynomialManager
	active, running, constructed bool
}

func (m *AlgebraicRootCore) cleanupSignInterval() error { return m.finishSignInterval(true) }
func (m *AlgebraicRootCore) finishSignInterval(owning bool) (cause error) {
	s := &m.signInterval
	if s.running {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	if !owning && s.sourceStop != nil {
		return s.sourceStop
	}
	if !owning {
		if e := m.rootTraceSourceCause(); e != nil {
			s.sourceStop = e
			return e
		}
	}
	defer func() {
		if !owning && cause != nil && s.sourceStop == nil {
			s.sourceStop = cause
		}
	}()
	child := s.evaluation != nil && s.evaluation.evaluation.intervalCaller == s
	if child && !owning && s.evaluation.evaluation.sourceCause != nil {
		return s.evaluation.evaluation.sourceCause
	}
	if child && owning && s.evaluation.evaluation.active {
		if e := s.evaluation.CleanupEvaluation(); e != nil {
			return e
		}
	}
	if s.constructed {
		if e := m.config.Intervals.Del(&s.interval); e != nil {
			return e
		}
		s.constructed = false
	}
	s.active = !owning && child && s.evaluation.evaluation.active
	return nil
}

// intervalSignPrefix is the source interval/refinement phase. An unresolved
// result leaves ri live for the exact resultant phase; restart reports that an
// input became rational and the source outer loop must start again.
func (m *AlgebraicRootCore) intervalSignPrefix(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation) (sign int, determined, restart bool, cause error) {
	if e := m.check(); e != nil {
		return 0, false, false, e
	}
	if pm == nil || values == nil {
		return 0, false, false, arena.ErrConfig
	}
	if e := pm.ready(); e != nil {
		return 0, false, false, e
	}
	s := &m.signInterval
	if s.active {
		return 0, false, false, ErrAlgebraicPending
	}
	*s = algebraicSignIntervalScope{active: true, running: true}
	defer func() {
		s.running = false
		if cause != nil || determined || restart {
			cause = errors.Join(cause, m.finishSignInterval(false))
		}
	}()
	xs := &m.evalSignVariables
	if !xs.constructed {
		variableEmbedded(xs, pm.core.memory, true)
	}
	if e := pm.Vars(p, xs); e != nil {
		return 0, false, false, e
	}
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return 0, false, false, e
	}
	if enabled && len(xs.values) == 0 {
		if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 2017, "!xs.empty()"); e != nil {
			return 0, false, false, e
		}
	}
	mapping := algebraicRootIntervals{owner: m, values: values}
	s.constructed = true
	for {
		if e = m.checkpoint(); e != nil {
			return 0, false, false, e
		}
		if e = pm.evaluateIntervalForAlgebraic(p, &mapping, &s.interval, s); e != nil {
			return 0, false, false, e
		}
		if e = m.traceSignInterval(false); e != nil {
			return 0, false, false, e
		}
		contains, e := m.config.Intervals.ContainsZero(&s.interval)
		if e != nil {
			return 0, false, false, e
		}
		if !contains {
			positive, e := m.config.Intervals.IsPos(&s.interval)
			if e != nil {
				return 0, false, false, e
			}
			if positive {
				return 1, true, false, nil
			}
			return -1, true, false, nil
		}
		refined := false
		for _, x := range xs.values {
			d, enabled, e := algebraicAssertionSwitch()
			if e != nil {
				return 0, false, false, e
			}
			if enabled {
				has, e := values.Contains(x)
				if e != nil {
					return 0, false, false, e
				}
				if !has {
					if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 2032, "x2v.contains(x)"); e != nil {
						return 0, false, false, e
					}
				}
			}
			a, e := values.Algebraic(x)
			if e != nil {
				return 0, false, false, e
			}
			if e = m.assertBinaryRoot(a, 2034, "!v.is_basic()"); e != nil {
				return 0, false, false, e
			}
			l, e := m.location(a)
			if e != nil {
				return 0, false, false, e
			}
			if l == nil || l.kind != 1 {
				return 0, false, false, ErrAlgebraicOwner
			}
			r, e := m.load(l)
			if e != nil {
				return 0, false, false, e
			}
			magnitude, e := m.magnitude(r.interval.BorrowLower(), r.interval.BorrowUpper())
			if e != nil {
				return 0, false, false, e
			}
			if magnitude > m.minMagnitude || m.zeroAccuracy > 0 && magnitude > m.zeroAccuracy {
				refinable, e := m.refine(a)
				if e != nil {
					return 0, false, false, e
				}
				if !refinable {
					if e = m.traceSignLiteral(2057, "restarting some algebraic_cell became basic\n"); e != nil {
						return 0, false, false, e
					}
					return 0, false, true, nil
				}
				if e = m.traceSignLiteral(2043, "refined algebraic interval\n"); e != nil {
					return 0, false, false, e
				}
				if e = m.assertBinaryRoot(a, 2044, "!v.is_basic()"); e != nil {
					return 0, false, false, e
				}
				refined = true
			}
		}
		if !refined {
			break
		}
	}
	if m.zeroAccuracy > 0 {
		return 0, true, false, nil
	}
	return 0, false, false, nil
}
