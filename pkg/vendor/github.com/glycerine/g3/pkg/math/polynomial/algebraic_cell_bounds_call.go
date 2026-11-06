package polynomial

// Source: algebraic_numbers.cpp:411,462,495; basic_interval.h:118-121.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

type algebraicCellBoundsCallScope struct {
	owner             *AlgebraicRootCore
	invocation        uint64
	running, cleaning bool
	bounds            unumeral.IntegerCopyAllocationReceipt
	roots             unumeral.NumericAllocationRoots
	hooks             unumeral.NumericInvocationHooks
	borrow            upolyFactorTraceBorrow
	bodyCause         error
}

func (s *algebraicCellBoundsCallScope) valid() bool {
	return s.owner != nil && s.owner.self == s.owner && s == &s.owner.cellBoundsCall && !s.owner.closed
}
func (s *algebraicCellBoundsCallScope) callback() bool {
	return (s.running || s.cleaning) && s.borrow.executing && s.borrow.callback()
}
func (s *algebraicCellBoundsCallScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || (s.running || s.cleaning) && s.borrow.executing && s.owner.config.Polynomials.factorTrace == &s.borrow && !s.borrow.callback() && s.owner.config.Polynomials.callbackDepth == 0)
}
func (s *algebraicCellBoundsCallScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.owner.config.Rationals.Integers() != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.roots
}
func (s *algebraicCellBoundsCallScope) context() unumeral.NumericAllocationContext {
	return unumeral.NumericAllocationContext{}.WithCaller(&s.hooks)
}
func (s *algebraicCellBoundsCallScope) pending() bool {
	if !s.valid() {
		return false
	}
	z := s.owner.config.Rationals.Integers()
	return z.MatchesCopyAllocationInvocation(&s.bounds) || unumeral.NumericAllocationPending(&s.roots)
}
func (m *AlgebraicRootCore) setCellBounds(dst *unumeral.BinaryInterval, lower, upper *unumeral.BinaryRational) (cause error) {
	s := &m.cellBoundsCall
	if s.running || s.cleaning || s.pending() || s.invocation == ^uint64(0) {
		return ErrAlgebraicPending
	}
	*s = algebraicCellBoundsCallScope{owner: m, invocation: s.invocation + 1}
	s.hooks = unumeral.NumericInvocationHooks{Generation: s.invocation, Policy: s}
	if e := s.borrow.bindManagers([3]*UPolynomialManager{m.config.Polynomials}, m.config.Rationals.Integers().BorrowTraceTags()); e != nil {
		return e
	}
	s.running = true
	defer func() { s.bodyCause = cause; s.running = false; s.borrow.unbind() }()
	return m.config.Intervals.WithAllocationContext(s.context()).SetBoundsTracked(dst, lower, upper, &s.bounds)
}
func (m *AlgebraicRootCore) cleanupCellBoundsCall() error {
	s := &m.cellBoundsCall
	if s.running || s.cleaning {
		return ErrAlgebraicPending
	}
	if !s.pending() {
		return nil
	}
	if e := s.borrow.bindManagers([3]*UPolynomialManager{m.config.Polynomials}, m.config.Rationals.Integers().BorrowTraceTags()); e != nil {
		return e
	}
	s.cleaning = true
	defer func() { s.cleaning = false; s.borrow.unbind() }()
	z := m.config.Rationals.Integers()
	if e := z.CleanupCopyAllocationInvocation(&s.bounds); e != nil {
		return e
	}
	return s.roots.Cleanup(z)
}
