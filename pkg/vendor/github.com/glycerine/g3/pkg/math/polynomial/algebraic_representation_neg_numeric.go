package polynomial

// Source: algebraic_numbers.cpp:1916; mpz.h:499-507 neg(INT_MIN) allocation.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

type algebraicRepresentationNegNumeric struct {
	owner *AlgebraicRootCore
	roots unumeral.NumericAllocationRoots
	hooks unumeral.NumericInvocationHooks
}

func (s *algebraicRepresentationNegNumeric) NumericBound(executing bool) bool {
	m := s.owner
	return m != nil && m.self == m && !m.closed && s == &m.representationNeg && (!executing || m.bounds.executing)
}
func (s *algebraicRepresentationNegNumeric) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.owner.config.Rationals.Integers() != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.roots
}
