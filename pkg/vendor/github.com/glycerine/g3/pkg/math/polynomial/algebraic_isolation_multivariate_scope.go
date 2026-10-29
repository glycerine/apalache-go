package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2287-2451.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Each recursion owns its actual lexical references and algebraic auxiliaries.
// The stack records their interleaved source construction order, including the
// declared but unused zxi_p reference. It never copies an owning numeric token.
type algebraicMultivariateIsolationFrame struct {
	assignments                   PolynomialAlgebraicEvaluation
	owner                         *AlgebraicRootCore
	child                         *algebraicMultivariateIsolationFrame
	pm                            *MultivariatePolynomialManager
	substitution                  algebraicSignSubstitutionScope
	refs                          [9]PolynomialRef // q, p_y, c0/c, c1, q2, z_p, xi_p, zxi_p, product
	values                        [2]Algebraic     // a0/a, a1
	stack                         [12]uint8
	depth                         uint8
	toPolynomial                  polynomialToPolynomialInvocationReceipt
	resultant                     polynomialGCDInvocationReceipt
	coefficient                   polynomialCoefficientInvocationReceipt
	append                        polynomialCheapAppendInvocationReceipt
	factory                       polynomialFactoryInvocationReceipt
	variables                     [2]polynomialVariableInvocationReceipt
	arithmetic                    [2]polynomialArithmeticInvocationReceipt
	conversionManager, evaluation *MultivariatePolynomialManager
	calls                         polynomialAlgebraicEvaluationScope
	neg                           algebraicNegInvocationReceipt
	active, running, borrowed     bool
	sourceStop                    error
}

func (s *algebraicMultivariateIsolationFrame) constructRef(i int, pm *MultivariatePolynomialManager) {
	s.refs[i] = PolynomialRef{owner: pm.core}
	s.refs[i].self = &s.refs[i]
	s.stack[s.depth] = uint8(i + 1)
	s.depth++
}
func (s *algebraicMultivariateIsolationFrame) constructValue(i int) {
	s.values[i] = Algebraic{}
	s.stack[s.depth] = uint8(i + 10)
	s.depth++
}
func (m *AlgebraicRootCore) newMultivariateIsolationFrame(pm *MultivariatePolynomialManager) (*algebraicMultivariateIsolationFrame, error) {
	if pm.algebraicIsolationBorrowers == math.MaxUint32 {
		return nil, arena.ErrSlotLimit
	}
	i := m.multivariateIsolationDepth
	if int(i) == len(m.multivariateIsolationFrames) {
		if e := m.config.Rationals.Budget().Reserve(uint64(reflect.TypeOf(algebraicMultivariateIsolationFrame{}).Size())); e != nil {
			return nil, e
		}
		m.multivariateIsolationFrames = append(m.multivariateIsolationFrames, &algebraicMultivariateIsolationFrame{})
	}
	s := m.multivariateIsolationFrames[i]
	if s.active {
		return nil, ErrAlgebraicPending
	}
	*s = algebraicMultivariateIsolationFrame{owner: m, pm: pm, active: true, running: true, borrowed: true}
	if i > 0 {
		m.multivariateIsolationFrames[i-1].child = s
	}
	s.calls.vm = m
	pm.algebraicIsolationBorrowers++
	m.multivariateIsolationDepth++
	return s, nil
}
func algebraicIsolationAssert(ok bool, line int32, expression string) error {
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled && !ok {
		return diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", line, expression)
	}
	return nil
}
func (s *algebraicMultivariateIsolationFrame) childSourceCause() error {
	if e := s.owner.rootTraceSourceCause(); e != nil {
		return e
	}
	if s.child != nil && s.child.active && s.child.sourceStop != nil {
		return s.child.sourceStop
	}
	if c := &s.owner.isolationConversion; c.caller == s && c.active {
		if r := &c.factor; r.service != nil && r.service.matchesFactorFrontendHistory(r) && r.service.frontend.receipt == *r && r.service.frontend.sourceCause != nil {
			return r.service.frontend.sourceCause
		}
	}
	if e := s.calls.childSourceCause(); e != nil {
		return e
	}
	if pm := s.evaluation; pm != nil && pm.algebraicEvaluation.isolationCaller == s && pm.algebraicEvaluation.active && pm.algebraicEvaluation.sourceCause != nil {
		return pm.algebraicEvaluation.sourceCause
	}
	if pm := s.conversionManager; pm != nil && pm.conversion.isolationCaller == s && pm.conversion.sourceCause != nil {
		return pm.conversion.sourceCause
	}
	if c := s.substitution.core; c != nil && c.substitution.scope.algebraicCaller == &s.substitution && c.substitution.scope.sourceCause != nil {
		return c.substitution.scope.sourceCause
	}
	if e := s.pm.toPolynomialInvocationSourceCause(&s.toPolynomial); e != nil {
		return e
	}
	if e := s.pm.gcdInvocationSourceCause(&s.resultant); e != nil {
		return e
	}
	if e := s.pm.core.coefficientInvocationSourceCause(&s.coefficient); e != nil {
		return e
	}
	for i := range s.arithmetic {
		if r := &s.arithmetic[i]; r.manager != nil {
			if e := r.manager.arithmeticInvocationSourceCause(r); e != nil {
				return e
			}
		}
	}
	if s.owner.matchesNegation(&s.neg) && s.neg.frame.sourceCause != nil {
		return s.neg.frame.sourceCause
	}
	return nil
}
func (m *AlgebraicRootCore) finishMultivariateIsolation(s *algebraicMultivariateIsolationFrame, owning bool) (cause error) {
	if !s.active {
		return nil
	}
	if s.running {
		return ErrAlgebraicPending
	}
	if !owning {
		if s.sourceStop != nil {
			return s.sourceStop
		}
		if e := s.childSourceCause(); e != nil {
			s.sourceStop = e
			return e
		}
	}
	defer func() {
		if !owning && cause != nil && s.sourceStop == nil {
			s.sourceStop = cause
		}
	}()
	if owning {
		if pm := s.evaluation; pm != nil && pm.algebraicEvaluation.isolationCaller == s {
			if e := pm.CleanupAlgebraicEvaluation(); e != nil {
				return e
			}
		}
		if pm := s.conversionManager; pm != nil && pm.conversion.isolationCaller == s {
			if e := pm.CleanupConversion(); e != nil {
				return e
			}
		}
	}
	c := s.pm.core
	if e := s.pm.cleanupGCDInvocation(&s.resultant, false); e != nil {
		return e
	}
	if e := s.pm.cleanupToPolynomialInvocation(&s.toPolynomial, false); e != nil {
		return e
	}
	if e := c.cleanupCoefficientInvocation(&s.coefficient, false); e != nil {
		return e
	}
	for i := len(s.arithmetic) - 1; i >= 0; i-- {
		if r := &s.arithmetic[i]; r.manager != nil {
			if e := r.manager.cleanupArithmeticInvocation(r, false); e != nil {
				return e
			}
		}
	}
	for s.depth > 0 {
		id := s.stack[s.depth-1]
		if id == 12 {
			if e := m.finishBasicSubstitution(&s.substitution, owning); e != nil {
				return e
			}
		} else if id >= 10 {
			a := &s.values[id-10]
			if owning {
				l, e := m.locationCleanup(a)
				if e != nil {
					return e
				}
				if l != nil {
					if e = m.destroyCell(a, l, true); e != nil {
						return e
					}
				}
			} else if e := m.Del(a); e != nil {
				return e
			}
		} else {
			r := &s.refs[id-1]
			var e error
			if owning {
				e = r.root.close(r.owner)
			} else {
				e = r.root.closeValue(r.owner, false)
			}
			if e != nil {
				return e
			}
		}
		s.depth--
	}
	if owning {
		if e := m.finishBasicSubstitution(&s.substitution, true); e != nil {
			return e
		}
		for i := range s.refs {
			r := &s.refs[i]
			if r.owner != nil {
				if e := r.root.abort(r.owner); e != nil {
					return e
				}
			}
		}
		if e := s.pm.cleanupGCDInvocation(&s.resultant, true); e != nil {
			return e
		}
		if e := s.pm.cleanupToPolynomialInvocation(&s.toPolynomial, true); e != nil {
			return e
		}
		if e := c.cleanupCoefficientInvocation(&s.coefficient, true); e != nil {
			return e
		}
		if e := c.cleanupFactoryInvocation(&s.factory, true); e != nil {
			return e
		}
		if e := c.cheap.cleanupAppendInvocation(&s.append, true); e != nil {
			return e
		}
		for i := len(s.arithmetic) - 1; i >= 0; i-- {
			if r := &s.arithmetic[i]; r.manager != nil {
				if e := r.manager.cleanupArithmeticInvocation(r, true); e != nil {
					return e
				}
			}
		}
		for i := len(s.variables) - 1; i >= 0; i-- {
			if r := &s.variables[i]; r.manager != nil {
				if e := r.manager.cleanupVariableInvocation(r, true); e != nil {
					return e
				}
			}
		}
		s.active = false
		s.assignments = nil
		if s.borrowed {
			s.pm.algebraicIsolationBorrowers--
			s.borrowed = false
		}
	}
	return nil
}
func (m *AlgebraicRootCore) cleanupMultivariateIsolation() error {
	if m.multivariateIsolationRunning {
		return ErrAlgebraicPending
	}
	for i := len(m.multivariateIsolationFrames) - 1; i >= 0; i-- {
		if e := m.finishMultivariateIsolation(m.multivariateIsolationFrames[i], true); e != nil {
			return e
		}
	}
	return nil
}
func (s *algebraicMultivariateIsolationFrame) evaluate(p Polynomial, out *Algebraic) (cause error) {
	m := s.owner
	previous := m.polynomialCaller
	m.polynomialCaller = &s.calls
	defer func() { m.polynomialCaller = previous }()
	return s.pm.evaluateAlgebraicForIsolation(p, m, s.assignments, out, s)
}

func (s *algebraicMultivariateIsolationFrame) finishBody(cause error) error {
	s.running = false
	e := s.owner.finishMultivariateIsolation(s, false)
	if e != nil {
		return errors.Join(cause, e)
	}
	if cause == nil {
		return s.owner.finishMultivariateIsolation(s, true)
	}
	return cause
}
