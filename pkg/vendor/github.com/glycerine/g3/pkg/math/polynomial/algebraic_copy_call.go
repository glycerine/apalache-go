package polynomial

// Source: algebraic_numbers.cpp:463-474,509-539; basic_interval.h:118-130.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

type algebraicCopyCallScope struct {
	owner                *AlgebraicRootCore
	invocation           uint64
	running, cleaning    bool
	coefficients, bounds unumeral.IntegerCopyAllocationReceipt
	roots                unumeral.NumericAllocationRoots
	hooks                unumeral.NumericInvocationHooks
	borrow               upolyFactorTraceBorrow
	bodyCause            error
}

func (s *algebraicCopyCallScope) valid() bool {
	return s.owner != nil && s.owner.self == s.owner && s == &s.owner.copyCall && !s.owner.closed
}
func (s *algebraicCopyCallScope) callback() bool {
	return (s.running || s.cleaning) && s.borrow.executing && s.borrow.callback()
}
func (s *algebraicCopyCallScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || (s.running || s.cleaning) && s.borrow.executing && s.owner.config.Polynomials.factorTrace == &s.borrow && !s.borrow.callback() && s.owner.config.Polynomials.callbackDepth == 0)
}
func (s *algebraicCopyCallScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.owner.config.Rationals.Integers() != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.roots
}
func (s *algebraicCopyCallScope) context() unumeral.NumericAllocationContext {
	return unumeral.NumericAllocationContext{}.WithCaller(&s.hooks)
}
func (s *algebraicCopyCallScope) pending() bool {
	if !s.valid() {
		return false
	}
	z := s.owner.config.Rationals.Integers()
	return z.MatchesCopyAllocationInvocation(&s.coefficients) || z.MatchesCopyAllocationInvocation(&s.bounds) || unumeral.NumericAllocationPending(&s.roots)
}
func (m *AlgebraicRootCore) Copy(dst, src *Algebraic) (cause error) {
	if m != nil && m.self == m && m.config.Rationals.Integers().MatchesCopyAllocationInvocation(&m.coefficientCopy) {
		return ErrAlgebraicPending
	}
	if e := m.check(); e != nil {
		return e
	}
	if m.copyCall.running || m.copyCall.cleaning || m.copyCall.pending() {
		return ErrAlgebraicPending
	}
	if _, e := m.locationCleanup(src); e != nil {
		return e
	}
	if _, e := m.locationCleanup(dst); e != nil {
		return e
	}
	if dst == src {
		return nil
	}
	s := &m.copyCall
	if s.invocation == ^uint64(0) {
		return ErrAlgebraicPending
	}
	*s = algebraicCopyCallScope{owner: m, invocation: s.invocation + 1}
	s.hooks = unumeral.NumericInvocationHooks{Generation: s.invocation, Policy: s}
	if e := s.borrow.bindManagers([3]*UPolynomialManager{m.config.Polynomials}, m.config.Rationals.Integers().BorrowTraceTags()); e != nil {
		return e
	}
	s.running = true
	defer func() { s.bodyCause = cause; s.running = false; s.borrow.unbind() }()
	return m.copyCellValue(dst, src)
}
func (m *AlgebraicRootCore) cleanupCopyCall() error {
	s := &m.copyCall
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
	if e := z.CleanupCopyAllocationInvocation(&s.coefficients); e != nil {
		return e
	}
	return s.roots.Cleanup(z)
}
