package polynomial

// Source: algebraic_numbers.cpp:395-397; mpz.h:499-507 neg(INT_MIN).

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// This owner is reached only by normalize_coeffs' negative-leading branch.
// Its roots retain actual failed primitive allocations, not a copy of the cell.
type algebraicCellNegNumeric struct {
	self       *algebraicCellNegNumeric
	owner      *AlgebraicRootCore
	roots      unumeral.NumericAllocationRoots
	hooks      unumeral.NumericInvocationHooks
	borrow     upolyFactorTraceBorrow
	running    bool
	cause      error
	invocation uint64
}

func (s *algebraicCellNegNumeric) NumericBound(executing bool) bool {
	m := s.owner
	return s.self == s && m != nil && m.self == m && !m.closed && s == &m.cellNeg && (!executing || s.running && s.borrow.executing && m.config.Polynomials.factorTrace == &s.borrow && !s.borrow.callback() && m.config.Polynomials.callbackDepth == 0)
}
func (s *algebraicCellNegNumeric) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.owner.config.Rationals.Integers() != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.roots
}
func (s *algebraicCellNegNumeric) pending() bool { return unumeral.NumericAllocationPending(&s.roots) }
func (m *AlgebraicRootCore) negateCellCoefficients(v UPolynomialView) error {
	s := &m.cellNeg
	if s.running || s.pending() || s.invocation == ^uint64(0) {
		return ErrAlgebraicPending
	}
	if s.self == nil {
		s.self = s
		s.owner = m
		s.hooks.Policy = s
	}
	if !s.NumericBound(false) {
		return ErrAlgebraicOwner
	}
	z := m.config.Rationals.Integers()
	if e := s.borrow.bindManagers([3]*UPolynomialManager{m.config.Polynomials}, z.BorrowTraceTags()); e != nil {
		return e
	}
	s.invocation++
	s.hooks.Generation = s.invocation
	s.cause = nil
	s.running = true
	defer func() { s.running = false; s.borrow.unbind() }()
	s.cause = m.config.Polynomials.negViewInPlaceWithNumericContext(v, unumeral.NumericAllocationContext{}.WithCaller(&s.hooks))
	return s.cause
}
func (m *AlgebraicRootCore) cleanupCellNeg() error {
	s := &m.cellNeg
	if s.running {
		return ErrAlgebraicPending
	}
	if s.self == nil {
		return nil
	}
	if !s.NumericBound(false) {
		return ErrAlgebraicOwner
	}
	return s.roots.Cleanup(m.config.Rationals.Integers())
}
