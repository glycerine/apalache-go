package polynomial

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"io"
	"strconv"
)

func (q *upolyBerlekampMatrix) traceSplitRank(rank uint32) error {
	return q.traceSplitRecord(432, rank, UPolynomialView{})
}
func (q *upolyBerlekampMatrix) traceNullVector() error {
	return q.traceSplitRecord(443, 0, UPolynomialView{})
}
func (q *upolyBerlekampMatrix) traceSplitOperands(current UPolynomialView) error {
	return q.traceSplitRecord(482, 0, current)
}

// Source: factorization.cpp:432,443-448,482-486. Each operand is evaluated
// where the literal source inserts it, including after an ostream failure.
func (q *upolyBerlekampMatrix) traceSplitRecord(line int, rank uint32, current UPolynomialView) error {
	tags := q.m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
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
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "zp_factor_square_free_berlekamp", "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			if line == 432 {
				_ = linearWriteText(out, "upolynomial::factor_square_free_berlekamp(): computing factors, expecting ")
				_ = linearWriteText(out, strconv.FormatUint(uint64(rank), 10))
			} else if line == 443 {
				_ = linearWriteText(out, "null vector: ")
				for j := uint32(0); j < q.size; j++ {
					a, e := q.m.read(q.m.stateView(&q.split.vector), j)
					if e != nil {
						computation = e
						return e
					}
					text, e := q.m.modular.String(a)
					if e != nil {
						computation = e
						return e
					}
					_ = linearWriteText(out, text)
					_ = linearWriteText(out, " ")
				}
			} else {
				for i, label := range []string{"current_factor = ", "gcd_norm = ", "div = "} {
					_ = linearWriteText(out, label)
					view := current
					if i == 1 {
						view = q.m.stateView(&q.split.gcd)
					} else if i == 2 {
						view = q.m.stateView(&q.split.div)
					}
					var issued upolyDisplayInvocationReceipt
					e := q.m.displayIntoWithReceipt(view, factorizationTraceStream{out}, "x", false, &issued)
					if q.m.matchesDisplayReceipt(&issued) {
						q.entryTrace = issued
					}
					if e != nil {
						computation = e
						return e
					}
					if i < 2 {
						_ = diagnostic.WriteTraceEndl(out)
					}
				}
			}
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}
