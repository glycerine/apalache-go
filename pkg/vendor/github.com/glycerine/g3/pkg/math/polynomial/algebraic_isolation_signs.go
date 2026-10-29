package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2549-2623,2883-2885.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

// ext2_var2num reports every variable present. A read consults the original
// mapping once, borrowing its assigned cell or the same fallback cell.
type algebraicAllUnassigned struct {
	base  PolynomialAlgebraicEvaluation
	value *Algebraic
}

func (v *algebraicAllUnassigned) Contains(uint32) (bool, error) { return true, nil }
func (v *algebraicAllUnassigned) Algebraic(x uint32) (*Algebraic, error) {
	has, e := v.base.Contains(x)
	if e != nil {
		return nil, e
	}
	if has {
		return v.base.Algebraic(x)
	}
	return v.value, nil
}

type algebraicIsolationSignsScope struct {
	traceVariables                                    PolynomialVariableDegrees
	traceVariablesConstructed                         bool
	traceRunning                                      bool
	sample                                            Algebraic
	calls                                             polynomialAlgebraicEvaluationScope
	pm                                                *MultivariatePolynomialManager
	active, running, finishing, constructed, borrowed bool
	sourceStop                                        error
}

func (m *AlgebraicRootCore) isolationSignsChildSourceCause() error {
	s := &m.isolationSigns
	if e := m.rootTraceSourceCause(); e != nil {
		return e
	}
	if e := s.calls.childSourceCause(); e != nil {
		return e
	}
	if m.signEvaluationCaller != s {
		return nil
	}
	if m.signResultant.active && m.signResultant.sourceStop != nil {
		return m.signResultant.sourceStop
	}
	if m.signInterval.active && m.signInterval.sourceStop != nil {
		return m.signInterval.sourceStop
	}
	if m.signSubstitution.active && m.signSubstitution.sourceStop != nil {
		return m.signSubstitution.sourceStop
	}
	if m.rationalSign.active {
		r := &m.rationalSign
		if pm := r.evaluation; pm != nil && pm.evaluation.rationalCaller == r && pm.evaluation.sourceCause != nil {
			return pm.evaluation.sourceCause
		}
		if r.sourceStop != nil {
			return r.sourceStop
		}
	}
	return nil
}
func (m *AlgebraicRootCore) finishIsolationSigns(owning bool) (cause error) {
	s := &m.isolationSigns
	if s.running || s.finishing {
		return ErrAlgebraicPending
	}
	if !s.active && !s.traceVariablesConstructed {
		return nil
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if !owning {
		if s.sourceStop != nil {
			return s.sourceStop
		}
		if e := m.isolationSignsChildSourceCause(); e != nil {
			s.sourceStop = e
			return e
		}
	}
	if e := m.cleanupIsolationSignsTraceVariables(owning); e != nil {
		return e
	}
	if s.constructed {
		if owning {
			l, e := m.locationCleanup(&s.sample)
			if e != nil {
				return e
			}
			if l != nil {
				if e = m.destroyCell(&s.sample, l, true); e != nil {
					return e
				}
			}
		} else if e := m.Del(&s.sample); e != nil {
			s.sourceStop = e
			return e
		}
		s.constructed = false
	}
	s.active = false
	if s.borrowed {
		s.pm.algebraicIsolationBorrowers--
		s.borrowed = false
	}
	return nil
}

// IsolateMultivariateRootsWithSigns appends the sign on each open interval to
// signs, after isolating roots. It refines roots to source precision two and
// may refine assigned values during sign evaluation. An error preserves any
// appended signs and partial roots; CleanupRetained releases stopped locals.
func (m *AlgebraicRootCore) IsolateMultivariateRootsWithSigns(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation, roots *AlgebraicVector, signs *[]int) (cause error) {
	if e := m.check(); e != nil {
		return e
	}
	if pm == nil || values == nil || roots == nil || signs == nil {
		return arena.ErrConfig
	}
	if e := pm.ready(); e != nil {
		return e
	}
	if roots.core != m {
		return ErrAlgebraicOwner
	}
	if e := roots.check(); e != nil {
		return e
	}
	s := &m.isolationSigns
	if s.active || m.rootConstruction.active || m.multivariateIsolationRunning {
		return ErrAlgebraicPending
	}
	if pm.algebraicIsolationBorrowers == math.MaxUint32 {
		return arena.ErrSlotLimit
	}
	*s = algebraicIsolationSignsScope{pm: pm, active: true, running: true, borrowed: true}
	s.calls.vm = m
	pm.algebraicIsolationBorrowers++
	defer func() { s.running = false; cause = errors.Join(cause, m.finishIsolationSigns(false)) }()
	if e := m.isolateMultivariateRootsEntry(pm, p, values, roots, s); e != nil {
		return e
	}
	count := len(roots.values)
	if count == 0 {
		zero := Algebraic{}
		mapping := algebraicAllUnassigned{base: values, value: &zero}
		sign, e := m.evalSignForIsolationSigns(pm, p, &mapping, s)
		if e != nil {
			return e
		}
		*signs = append(*signs, sign)
		return nil
	}
	if e := m.traceIsolationSignsRoots(pm, p, values, roots); e != nil {
		return e
	}
	for i := 0; i < count; i++ {
		if e := m.RefineToPrecision(&roots.values[i], 2); e != nil {
			return e
		}
	}
	s.constructed = true
	if e := m.isolationSignsInteger(&roots.values[0], false); e != nil {
		return e
	}
	if e := m.traceIsolationSignsSample(); e != nil {
		return e
	}
	if e := m.appendIsolationSign(pm, p, values, signs, 2602); e != nil {
		return e
	}
	for i := 1; i < count; i++ {
		previous := m.polynomialCaller
		m.polynomialCaller = &s.calls
		e := m.Select(&roots.values[i-1], &roots.values[i], &s.sample)
		m.polynomialCaller = previous
		if e != nil {
			return e
		}
		if e = m.appendIsolationSign(pm, p, values, signs, 2612); e != nil {
			return e
		}
	}
	if e := m.isolationSignsInteger(&roots.values[count-1], true); e != nil {
		return e
	}
	return m.appendIsolationSign(pm, p, values, signs, 2620)
}
func (m *AlgebraicRootCore) isolationSignsInteger(a *Algebraic, upper bool) error {
	s := &m.isolationSigns
	previous := m.polynomialCaller
	m.polynomialCaller = &s.calls
	defer func() { m.polynomialCaller = previous }()
	if upper {
		return m.IntGT(a, &s.sample)
	}
	return m.IntLT(a, &s.sample)
}
func (m *AlgebraicRootCore) appendIsolationSign(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation, signs *[]int, line int32) error {
	s := &m.isolationSigns
	mapping := algebraicAllUnassigned{base: values, value: &s.sample}
	sign, e := m.evalSignForIsolationSigns(pm, p, &mapping, s)
	if e != nil {
		return e
	}
	if e = algebraicIsolationAssert(sign != 0, line, "s != 0"); e != nil {
		return e
	}
	*signs = append(*signs, sign)
	return nil
}
