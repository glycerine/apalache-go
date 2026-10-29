package polynomial

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (c *upolyFactorSquareFreeScope) tracePending() bool {
	if q := &c.recombinationStep; q.matches() && q.owner == c && q.exactTrace.owner.displayReceiptPending(&q.exactTrace) {
		return true
	}
	return c.trace.pending() || c.tracePolynomial.owner.displayReceiptPending(&c.tracePolynomial)
}
func (c *upolyFactorSquareFreeScope) traceSourceBlocker() error {
	if q := &c.recombinationStep; q.matches() && q.owner == c {
		if e := q.exactTrace.sourceBlocker(); e != nil {
			return e
		}
	}
	if e := c.trace.sourceBlocker(); e != nil {
		return e
	}
	r := &c.tracePolynomial
	if r.owner.displayReceiptPending(r) {
		if e := r.owner.displayReceiptSourceCause(r); e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	return nil
}
func (c *upolyFactorSquareFreeScope) cleanupTrace() error {
	if q := &c.recombinationStep; q.matches() && q.owner == c {
		if q.executing || q.sourceExecuting || q.ownExecuting || q.callDepth != 0 {
			return ErrUPolynomialCleanupPending
		}
		if e := q.exactTrace.cleanupOwned(); e != nil {
			return e
		}
	}
	if e := c.trace.cleanup(); e != nil {
		return e
	}
	r := &c.tracePolynomial
	if r.owner == nil {
		return nil
	}
	return r.owner.cleanupDisplayReceipt(r)
}

// Complete retained source records at cpp1052,1100. The exact accepted
// display receipt, rather than the trace borrow, owns any unfinished child.
func (s *UPolynomialFactorizationService) traceFactorPolynomial(m *UPolynomialManager, f UPolynomialView, line int) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	c := s.scope
	if c == nil || c.self != c {
		return ErrUPolynomialOwner
	}
	if e := c.traceBorrow.bind(c, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "factor_square_free", "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			label, variable := "sage: f_pp = ", "x"
			switch line {
			case 1052:
				label = "sage: f_pp = "
			case 1100:
				_ = linearWriteText(out, "sage: Rp.<x_p> = GF(")
				prime, e := s.manager.modular.String(&c.p)
				if e != nil {
					computation = e
					return e
				}
				_ = linearWriteText(out, prime)
				_ = linearWriteText(out, ")['x_p']")
				_ = diagnostic.WriteTraceEndl(out)
				label, variable = "sage: f_pp_zp = ", "x_p"
			}
			_ = linearWriteText(out, label)
			if e := m.displayIntoWithReceipt(f, factorizationTraceStream{out}, variable, false, &c.tracePolynomial); e != nil {
				computation = e
				return e
			}
			_ = diagnostic.WriteTraceEndl(out)
			switch line {
			case 1052:
				_ = linearWriteText(out, "sage: if (not (f_pp * 1 == f): print 'Error, content computation wrong'")
				_ = diagnostic.WriteTraceEndl(out)
			}
			return nil
		})
	return computation
}

// Source: upolynomial_factorization.cpp:1239-1242,1246-1250. The quotient
// is displayed even after an inexact division, before the source branch.
func (s *UPolynomialFactorizationService) traceRecombinationDivision(line int, exact bool) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	c := s.scope
	if c == nil || c.self != c {
		return ErrUPolynomialOwner
	}
	if e := c.traceBorrow.bind(c, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "factor_square_free", "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			first, second := &c.fpp, &c.trial
			labels := [2]string{"f_pp*lc(f_pp) = ", "trial_factor = "}
			if line == 1246 {
				first, second = &c.trial, &c.quotient
				labels = [2]string{"trial_factor = ", "trial_factor_quo = "}
			}
			for i, v := range [2]*upolyVectorState{first, second} {
				_ = linearWriteText(out, labels[i])
				if e := c.manager.displayIntoWithReceipt(c.manager.stateView(v), factorizationTraceStream{out}, "x", false, &c.tracePolynomial); e != nil {
					computation = e
					return e
				}
				_ = diagnostic.WriteTraceEndl(out)
			}
			if line == 1246 {
				_ = linearWriteText(out, "result = ")
				result := "false"
				if exact {
					result = "true"
				}
				_ = linearWriteText(out, result)
				_ = diagnostic.WriteTraceEndl(out)
			}
			return nil
		})
	return computation
}

// Source: upolynomial_factorization.cpp:1274-1278, after the actual
// publication/rejection branch. scoped_numeral insertion uses raw mpz display.
func (s *UPolynomialFactorizationService) traceRecombinationFactors(fs *UPolynomialFactors) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	c := s.scope
	if c == nil || c.self != c {
		return ErrUPolynomialOwner
	}
	if e := c.traceBorrow.bind(c, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "factor_square_free", "src/math/polynomial/upolynomial_factorization.cpp", 1274,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "factors = ")
			if e := fs.displayIntoWithReceipt(factorizationTraceStream{out}, &c.trace); e != nil {
				computation = e
				return e
			}
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, "f_pp*lc(f_pp) = ")
			if e := c.manager.displayIntoWithReceipt(c.manager.stateView(&c.fpp), factorizationTraceStream{out}, "x", false, &c.tracePolynomial); e != nil {
				computation = e
				return e
			}
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, "lc(f_pp) = ")
			leading, e := c.manager.config.Integer.String(&c.leading)
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, leading)
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}
