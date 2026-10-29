package polynomial

// Source: src/math/polynomial/polynomial.cpp:6364-6388;
// src/util/mpzzp.h power. Actual allocation roots of the reached power leaf.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

type polynomialSubstitutionPowerNumeric struct {
	owner *MultivariatePolynomialCore
	roots unumeral.NumericAllocationRoots
	hooks unumeral.NumericInvocationHooks
}

func (s *polynomialSubstitutionPowerNumeric) NumericBound(executing bool) bool {
	m := s.owner
	return m != nil && m.self == m && s == &m.substitution.scope.powerNumeric && m.substitution.scope.invocation != 0 && s.hooks.Generation == m.substitution.scope.invocation && (!executing || m.substitution.scope.executing)
}
func (s *polynomialSubstitutionPowerNumeric) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.owner.integer != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.roots
}
