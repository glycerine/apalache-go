package polynomial

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"io"
)

// Source: upolynomial_factorization.cpp:1020-1025, before the primitive
// debug scope. Only accepted display metadata is acquired here; none of the
// later square-free lexical objects has been constructed.
func (s *UPolynomialFactorizationService) traceFactorEntry(f UPolynomialView, receipt *upolyFactorSquareFreeInvocationReceipt) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	if e := s.entryTraceBorrow.bindManagers([3]*UPolynomialManager{s.manager}, tags); e != nil {
		return e
	}
	defer s.entryTraceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "factor_square_free", "src/math/polynomial/upolynomial_factorization.cpp", 1020,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "sage: f = ")
			var issued upolyDisplayInvocationReceipt
			e := s.manager.displayIntoWithReceipt(f, factorizationTraceStream{out}, "x", false, &issued)
			if s.manager.matchesDisplayReceipt(&issued) {
				s.entryTrace = issued
			}
			if receipt != nil && s.manager.matchesDisplayReceipt(&issued) {
				*receipt = upolyFactorSquareFreeInvocationReceipt{service: s, entry: issued}
			}
			if e != nil {
				computation = e
				return e
			}
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, "sage: if (not f.is_squarefree()): print 'Error, f is not square-free'")
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, "sage: print 'Factoring :', f")
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, "sage: print 'Expected factors: ', f.factor()")
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}
