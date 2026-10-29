package polynomial

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"io"
	"strconv"
)

// Source: factorization.cpp:428. The matrix and published caller factors are
// already constructed, but the later gcd/div/v_k vectors do not yet exist.
func (q *upolyBerlekampMatrix) traceIrreducible(f UPolynomialView, fs *UPolynomialFactors, p uint32) error {
	return q.traceOutcome(f, fs, p, 428)
}

// Source: factorization.cpp:498 adds the actual total degree after factors.
func (q *upolyBerlekampMatrix) traceSplitOutcome(f UPolynomialView, fs *UPolynomialFactors, p uint32) error {
	return q.traceOutcome(f, fs, p, 498)
}
func (q *upolyBerlekampMatrix) traceOutcome(f UPolynomialView, fs *UPolynomialFactors, p uint32, line int) error {
	tags := q.m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization") {
		return nil
	}
	if q.displaying || q.numeric.executing || q.numeric.cleaning {
		return ErrUPolynomialCleanupPending
	}
	q.displaying = true
	defer func() { q.displaying = false }()
	if e := q.traceBorrow.bindManagers([3]*UPolynomialManager{q.m}, tags); e != nil {
		return e
	}
	defer q.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization", "zp_factor_square_free_berlekamp", "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			caption := "upolynomial::factor_square_free_berlekamp("
			if line == 498 {
				caption = "polynomial::factor("
			}
			_ = linearWriteText(out, caption)
			var issued upolyDisplayInvocationReceipt
			e := q.m.displayIntoWithReceipt(f, factorizationTraceStream{out}, "x", false, &issued)
			if q.m.matchesDisplayReceipt(&issued) {
				q.entryTrace = issued
			}
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, ", ")
			_ = linearWriteText(out, strconv.FormatUint(uint64(p), 10))
			_ = linearWriteText(out, ") => ")
			if e := fs.displayIntoWithReceipt(factorizationTraceStream{out}, &q.outcomeFactors); e != nil {
				computation = e
				return e
			}
			if line == 498 {
				_ = linearWriteText(out, " of degree ")
				degree, e := fs.TotalDegree()
				if e != nil {
					computation = e
					return e
				}
				_ = linearWriteText(out, strconv.FormatUint(uint64(degree), 10))
			}
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}
