package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1933-1949,1979-1997.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// PolynomialAlgebraicEvaluation borrows the actual source assignment cells.
// A null Algebraic value represents zero; a nil pointer is an invalid lookup.
type PolynomialAlgebraicEvaluation interface {
	Contains(uint32) (bool, error)
	Algebraic(uint32) (*Algebraic, error)
}

var errAlgebraicOptimisticNonbasic = errors.New("algebraic optimistic mapping encountered a root cell")

type algebraicOptimisticRationals struct {
	owner  *AlgebraicRootCore
	values PolynomialAlgebraicEvaluation
	// These are borrowed numeral snapshots of encoded algebraic cells, not
	// duplicated owning values. Stable per-variable addresses survive recursion.
	borrowed map[uint32]*algebraicRecord
}

func (v *algebraicOptimisticRationals) Manager() (*unumeral.RationalManager, error) {
	return v.owner.config.Rationals, nil
}
func (v *algebraicOptimisticRationals) Contains(x uint32) (bool, error) { return v.values.Contains(x) }
func (v *algebraicOptimisticRationals) Rational(x uint32) (*unumeral.Rational, error) {
	a, e := v.values.Algebraic(x)
	if e != nil {
		return nil, e
	}
	l, e := v.owner.location(a)
	if e != nil {
		return nil, e
	}
	if l != nil && l.kind != 0 {
		return nil, errAlgebraicOptimisticNonbasic
	}
	if v.borrowed == nil {
		v.borrowed = make(map[uint32]*algebraicRecord)
	}
	r := v.borrowed[x]
	if r == nil {
		r = &algebraicRecord{}
		v.borrowed[x] = r
	}
	if l == nil {
		r.value = v.owner.zero
	} else {
		*r, e = v.owner.load(l)
		if e != nil {
			return nil, e
		}
	}
	return &r.value, nil
}

type algebraicRationalSignScope struct {
	sourceStop      error
	value           unumeral.Rational
	evaluation      *MultivariatePolynomialManager
	active, running bool
	valueOwned      bool
}

func (m *AlgebraicRootCore) cleanupRationalSign() error {
	return m.finishRationalSign(true)
}
func (m *AlgebraicRootCore) finishRationalSign(owning bool) (cause error) {
	s := &m.rationalSign
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
	// This pointer is published only at the actual recursive engine entry.
	// An older active engine refusal gives this parent no cleanup authority.
	if c := s.evaluation; owning && c != nil && c.evaluation.rationalCaller == s && c.evaluation.active {
		if e := c.CleanupEvaluation(); e != nil {
			return e
		}
	}
	if s.valueOwned {
		if e := m.config.Rationals.Del(&s.value); e != nil {
			return e
		}
		s.valueOwned = false
	}
	s.active = false
	if c := s.evaluation; !owning && c != nil && c.evaluation.rationalCaller == s && c.evaluation.active {
		s.active = true
	}
	return nil
}

// tryRationalSign is the source optimistic branch. Only its private nonbasic
// sentinel requests the later interval/resultant path; all other errors escape.
func (m *AlgebraicRootCore) tryRationalSign(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation) (sign int, allBasic bool, cause error) {
	if e := m.check(); e != nil {
		return 0, false, e
	}
	if pm == nil || values == nil {
		return 0, false, arena.ErrConfig
	}
	s := &m.rationalSign
	if s.active {
		return 0, false, ErrAlgebraicPending
	}
	*s = algebraicRationalSignScope{value: unumeral.NewRational(), active: true, running: true, valueOwned: true}
	defer func() {
		s.running = false
		// A source child destructor stop precedes the older scoped rational.
		if c := s.evaluation; c != nil && c.evaluation.rationalCaller == s && c.evaluation.sourceCause != nil {
			cause = errors.Join(cause, c.evaluation.sourceCause)
			return
		}
		cause = errors.Join(cause, m.finishRationalSign(false))
	}()
	mapping := algebraicOptimisticRationals{owner: m, values: values}
	e := pm.evaluateRationalForAlgebraic(p, &mapping, &s.value, s)
	if errors.Is(e, errAlgebraicOptimisticNonbasic) {
		return 0, false, nil
	}
	if e != nil {
		return 0, false, e
	}
	if e = m.traceSignRational(); e != nil {
		return 0, false, e
	}
	valueSign, e := m.config.Rationals.Sign(&s.value)
	return int(valueSign), true, e
}
