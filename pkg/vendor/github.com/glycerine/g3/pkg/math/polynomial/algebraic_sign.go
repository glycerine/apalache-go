package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1979-2188,3041-3043.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// EvalSign follows the rational, interval and exact resultant phases. It may
// refine mapped algebraic values or replace one with a discovered rational.
// Like the source, the rational phase returns qm().sign (the signed inline
// numerator for small values); the interval phase returns -1, 0 or 1.
func (m *AlgebraicRootCore) EvalSign(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation) (int, error) {
	return m.evalSignForIsolationSigns(pm, p, values, nil)
}
func (m *AlgebraicRootCore) evalSignForIsolationSigns(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation, caller *algebraicIsolationSignsScope) (int, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	if pm == nil || values == nil {
		return 0, arena.ErrConfig
	}
	if m.signEvaluationRunning || m.signResultant.active || m.signInterval.active || m.signSubstitution.active || m.rationalSign.active {
		return 0, ErrAlgebraicPending
	}
	if e := pm.ready(); e != nil {
		return 0, e
	}
	m.signEvaluationManager = pm
	defer func() { m.signEvaluationManager = nil }()
	if e := m.traceSignPolynomial(pm, p, 1981, "evaluating sign of: "); e != nil {
		return 0, e
	}
	m.signEvaluationCaller = caller
	m.signEvaluationRunning = true
	defer func() { m.signEvaluationRunning = false }()
	for {
		sign, all, e := m.tryRationalSign(pm, p, values)
		if e != nil || all {
			return sign, e
		}
		sign, restart, e := m.evalSignAfterRationals(pm, p, values)
		if e != nil || !restart {
			return sign, e
		}
	}
}
func (m *AlgebraicRootCore) evalSignAfterRationals(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation) (sign int, restart bool, cause error) {
	defer func() {
		if e := m.finishSignResultant(false); e != nil {
			cause = errors.Join(cause, e)
			return
		}
		if e := m.finishSignInterval(false); e != nil {
			cause = errors.Join(cause, e)
			return
		}
		cause = errors.Join(cause, m.finishSignSubstitution(false))
	}()
	prime, sign, constant, e := m.prepareSignPolynomial(pm, p, values)
	if e != nil || constant {
		return sign, false, e
	}
	sign, determined, restart, e := m.intervalSignPrefix(pm, prime, values)
	if e != nil || determined || restart {
		return sign, restart, e
	}
	return m.exactSignResultant(pm, prime, values)
}
