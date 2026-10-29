package polynomial

// Source: src/math/polynomial/polynomial.cpp:3318-3338;
// src/util/mpzzp.h:set; src/util/mpz.cpp:big_set.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// This receipt covers both actual SET allocation and the reached finite-field
// normalization leaf, so either source child remains attributable to this call.
type polynomialConstantCoefficientScope struct {
	invocation        uint64
	active, executing bool
	copy              unumeral.IntegerCopyAllocationReceipt
	normalize         unumeral.IntegerNormalizeInvocationReceipt
	sourceCause       error
}
type polynomialConstantCoefficientInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialConstantCoefficientScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesConstantCoefficientInvocation(r *polynomialConstantCoefficientInvocationReceipt) bool {
	return r != nil && r.manager == m && r.frame == &m.constantCoefficientInvocation && r.invocation != 0 && r.invocation == r.frame.invocation && r.frame.active
}
func (m *MultivariatePolynomialCore) cleanupConstantCoefficientInvocation(r *polynomialConstantCoefficientInvocationReceipt, abort bool) error {
	if !m.matchesConstantCoefficientInvocation(r) {
		return nil
	}
	s := r.frame
	if s.executing {
		return ErrPolynomialPending
	}
	if e := m.modular.CleanupPNormalizeInvocation(&s.normalize, abort); e != nil {
		return e
	}
	if !abort {
		return nil
	}
	if e := m.integer.CleanupCopyAllocationInvocation(&s.copy); e != nil {
		return e
	}
	*s = polynomialConstantCoefficientScope{invocation: s.invocation, normalize: s.normalize, sourceCause: s.sourceCause}
	return nil
}

func (m *MultivariatePolynomialCore) constantCoefficientInvocationSourceCause(r *polynomialConstantCoefficientInvocationReceipt) error {
	if r == nil || r.manager != m || r.frame != &m.constantCoefficientInvocation || r.invocation == 0 || r.invocation != r.frame.invocation {
		return nil
	}
	return r.frame.sourceCause
}
