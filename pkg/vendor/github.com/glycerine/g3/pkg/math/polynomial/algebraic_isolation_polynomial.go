package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:635-643.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type algebraicIsolationConversionScope struct {
	refinable       upolyRefinableInvocationReceipt
	caller          *algebraicMultivariateIsolationFrame
	factor          upolyFactorFrontendInvocationReceipt
	receipt         polynomialFactorNumeralVectorReceipt
	active, running bool
	scratchStarted  bool
}

func (m *AlgebraicRootCore) cleanupIsolationConversion() error {
	return m.cleanupIsolationConversionScope(&m.isolationConversion)
}
func (m *AlgebraicRootCore) cleanupIsolationConversionScope(s *algebraicIsolationConversionScope) error {
	if s.running {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	if e := m.config.Polynomials.cleanupRefinableReceipt(&s.refinable); e != nil {
		return e
	}
	if r := &s.factor; r.service != nil {
		if e := r.service.cleanupFactorFrontendInvocation(r, false); e != nil {
			return e
		}
		if e := r.service.cleanupFactorFrontendInvocation(r, true); e != nil {
			return e
		}
	}
	if s.scratchStarted {
		for _, v := range []*BinaryRationalVector{m.uppers, m.lowers, m.roots} {
			if v == nil {
				continue
			}
			if e := v.CleanupManagerReset(); e != nil {
				return e
			}
			if e := v.CleanupDrop(); e != nil {
				return e
			}
			v.state.dropPending = true
			v.state.dropCursor, v.state.dropTo = 0, 0
			if e := upolyBinaryFinishDrop(v.manager, &v.state, false); e != nil {
				return e
			}
		}
		s.scratchStarted = false
	}
	if pm := s.receipt.manager; pm != nil {
		if e := pm.cleanupFactorNumeralVector(&s.receipt, false); e != nil {
			return e
		}
		if e := pm.cleanupFactorNumeralVector(&s.receipt, true); e != nil {
			return e
		}
	}
	s.active = false
	return nil
}

// IsolatePolynomialRoots is the source univariate polynomial-ref overload.
// Zero leaves the output untouched; otherwise conversion writes the actual
// persistent m_isolate_tmp1 before invoking the existing UP root isolator.
func (m *AlgebraicRootCore) IsolatePolynomialRoots(pm *MultivariatePolynomialManager, p Polynomial, out *AlgebraicVector) error {
	return m.isolatePolynomialRootsForMultivariate(pm, p, out, nil)
}
func (m *AlgebraicRootCore) isolatePolynomialRootsForMultivariate(pm *MultivariatePolynomialManager, p Polynomial, out *AlgebraicVector, caller *algebraicMultivariateIsolationFrame) error {
	if e := m.check(); e != nil {
		return e
	}
	if pm == nil || out == nil {
		return arena.ErrConfig
	}
	if out.core != m {
		return ErrAlgebraicOwner
	}
	if e := out.check(); e != nil {
		return e
	}
	if e := pm.ready(); e != nil {
		return e
	}
	s := &m.isolationConversion
	if m.rootConstruction.active || s.active || s.running {
		return ErrAlgebraicPending
	}
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled {
		univariate, e := pm.core.IsUnivariate(p)
		if e != nil {
			return e
		}
		if !univariate {
			if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 636, "is_univariate(p)"); e != nil {
				return e
			}
		}
	}
	if e = m.tracePolynomialInput(pm, p, false); e != nil {
		return e
	}
	zero, e := pm.core.IsZero(p)
	if e != nil || zero {
		return e
	}
	*s = algebraicIsolationConversionScope{caller: caller, active: true, running: true}
	defer func() { s.running = false }()
	up := m.config.Polynomials
	if m.isolateTmp1.slot == 0 {
		m.isolateTmp1, e = up.NewPolynomial()
		if e != nil {
			return e
		}
	}
	target, e := up.descriptor(m.isolateTmp1)
	if e != nil {
		return e
	}
	if e = pm.toNumeralVectorIntoSource(p, 0, false, up, &target.state, &s.receipt); e != nil {
		return e
	}
	view, e := up.wholeView(m.isolateTmp1)
	if e != nil {
		return e
	}
	e = m.isolateRootsForPolynomial(view, out, s)
	if e == nil {
		s.active = false
	}
	return e
}
