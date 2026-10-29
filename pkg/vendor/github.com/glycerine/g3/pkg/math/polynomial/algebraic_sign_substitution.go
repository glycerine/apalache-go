package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1948-1960,1996-2011.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicBasicRationals struct{ algebraicOptimisticRationals }

func (v *algebraicBasicRationals) Contains(x uint32) (bool, error) {
	has, e := v.values.Contains(x)
	if e != nil || !has {
		return has, e
	}
	a, e := v.values.Algebraic(x)
	if e != nil {
		return false, e
	}
	l, e := v.owner.location(a)
	return l == nil || l.kind == 0, e
}
func (v *algebraicBasicRationals) Rational(x uint32) (*unumeral.Rational, error) {
	// The source getter asserts the value's basic tag independently of Contains.
	a, e := v.values.Algebraic(x)
	if e != nil {
		return nil, e
	}
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return nil, e
	}
	l, e := v.owner.location(a)
	if e != nil {
		return nil, e
	}
	if enabled && l != nil && l.kind != 0 {
		if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 1956, "v.is_basic()"); e != nil {
			return nil, e
		}
	}
	if l != nil && l.kind != 0 {
		return nil, ErrAlgebraicOwner
	}
	if e = v.owner.traceBasicRationalLookup(x, l); e != nil {
		return nil, e
	}
	// Borrow this reached value without performing a second virtual lookup.
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

type algebraicSignSubstitutionScope struct {
	sourceStop      error
	prime           PolynomialRef
	core            *MultivariatePolynomialCore
	active, running bool
}

func (m *AlgebraicRootCore) cleanupSignSubstitution() error { return m.finishSignSubstitution(true) }
func (m *AlgebraicRootCore) finishSignSubstitution(owning bool) error {
	return m.finishBasicSubstitution(&m.signSubstitution, owning)
}
func (m *AlgebraicRootCore) finishBasicSubstitution(s *algebraicSignSubstitutionScope, owning bool) (cause error) {
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
	c := s.prime.owner
	if c == nil {
		return ErrAlgebraicOwner
	}
	child := s.core != nil && s.core.substitution.scope.algebraicCaller == s
	if child && !owning && s.core.substitution.scope.sourceCause != nil {
		return s.core.substitution.scope.sourceCause
	}
	if child && owning {
		if e := s.core.CleanupSubstitution(); e != nil {
			return e
		}
	}
	if owning {
		if e := s.prime.root.close(c); e != nil {
			return e
		}
	} else {
		if e := s.prime.root.closeValue(c, false); e != nil {
			return e
		}
	}
	s.active = false
	// Even after the callee's lexical locals end, an unsuccessful invocation
	// can still own an unjoined modular-power or factory construction.
	if !owning && (s.prime.root.assigning || s.prime.root.incoming != (Polynomial{}) || child) {
		s.active = true
	}
	return nil
}

// prepareSignPolynomial implements p_prime = substitute(p, var2basic). The
// actual owning reference remains live for the later interval/resultant loop.
// Zero and constant branches are returned here with their source sign value.
func (m *AlgebraicRootCore) prepareSignPolynomial(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation) (result Polynomial, sign int, constant bool, cause error) {
	if e := m.check(); e != nil {
		return result, 0, false, e
	}
	if pm == nil || values == nil {
		return result, 0, false, arena.ErrConfig
	}
	if e := pm.ready(); e != nil {
		return result, 0, false, e
	}
	s := &m.signSubstitution
	if s.active {
		return result, 0, false, ErrAlgebraicPending
	}
	*s = algebraicSignSubstitutionScope{active: true, running: true, prime: PolynomialRef{owner: pm.core}}
	s.prime.self = &s.prime
	defer func() {
		s.running = false
		if cause != nil {
			cause = errors.Join(cause, m.finishSignSubstitution(false))
		}
	}()
	mapping := algebraicBasicRationals{algebraicOptimisticRationals: algebraicOptimisticRationals{owner: m, values: values}}
	substituted, e := pm.core.substituteRationalForAlgebraic(p, &mapping, s)
	if e != nil {
		return result, 0, false, e
	}
	if e = s.prime.Assign(substituted); e != nil {
		return result, 0, false, e
	}
	if e = m.traceSignPolynomial(pm, s.prime.root.value, 2000, "p after eliminating rationals: "); e != nil {
		return result, 0, false, e
	}
	result = s.prime.root.value
	zero, e := pm.core.IsZero(result)
	if e != nil {
		return result, 0, false, e
	}
	if zero {
		return result, 0, true, nil
	}
	constant, e = pm.core.IsConstant(result)
	if e != nil || !constant {
		return result, 0, constant, e
	}
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return result, 0, false, e
	}
	if enabled {
		size, e := pm.core.readWord(result, 8)
		if e != nil {
			return result, 0, false, e
		}
		if size != 1 {
			if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 2009, "size(p_prime) == 1"); e != nil {
				return result, 0, false, e
			}
		}
	}
	coefficient, e := pm.core.coefficientCore(result, 0)
	if e != nil {
		return result, 0, false, e
	}
	sourceSign, e := pm.core.integer.Sign(&coefficient)
	return result, int(sourceSign), true, e
}
