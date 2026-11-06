package polynomial

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Source: upolynomial_factorization.cpp:256,375. These records precede every
// lexical object in the corresponding finite-field factorization body.
func (m *UPolynomialManager) traceFactorFieldEntry(q *upolyFactorFieldScope, f UPolynomialView, p uint32, line int) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	name, caption := "zp_square_free_factor", "polynomial::square_free_factor("
	if line == 375 {
		name, caption = "zp_factor", "polynomial::factor("
	}
	var computation error
	_ = tags.EmitConditional("polynomial::factorization", name, "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, caption)
			if e := m.traceFactorFieldPolynomial(out, f, &q.polynomial); e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, ") over Z_")
			_ = linearWriteText(out, strconv.FormatUint(uint64(p), 10))
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}

func (m *UPolynomialManager) traceFactorFieldPolynomial(out io.Writer, f UPolynomialView, receipt *upolyDisplayInvocationReceipt) error {
	return m.displayIntoWithReceipt(f, factorizationTraceStream{out}, "x", false, receipt)
}

// Source: upolynomial_factorization.cpp:268-273. The factor constant has
// already been published and T_0 has already been made monic.
func (m *UPolynomialManager) traceSquareFreeInitial(q *upolyFactorFieldScope, fs *UPolynomialFactors, t0 UPolynomialView, p uint32) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "zp_square_free_factor", "src/math/polynomial/upolynomial_factorization.cpp", 268,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "Initial factors: ")
			if e := fs.displayIntoWithReceipt(factorizationTraceStream{out}, &q.factors); e != nil {
				computation = e
				return e
			}
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, "R.<x> = GF(")
			_ = linearWriteText(out, strconv.FormatUint(uint64(p), 10))
			_ = linearWriteText(out, ")['x']")
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, "T_0 = ")
			if e := m.traceFactorFieldPolynomial(out, t0, &q.polynomial); e != nil {
				computation = e
				return e
			}
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}

// Source: upolynomial_factorization.cpp:285-366. Each call receives the
// polynomial state from the exact source point following its computation.
func (m *UPolynomialManager) traceSquareFreeState(q *upolyFactorFieldScope, line int, first, second string, value UPolynomialView) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "zp_square_free_factor", "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			if first != "" {
				_ = linearWriteText(out, first)
				_ = diagnostic.WriteTraceEndl(out)
			}
			_ = linearWriteText(out, second)
			if e := m.traceFactorFieldPolynomial(out, value, &q.polynomial); e != nil {
				computation = e
				return e
			}
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}

func (m *UPolynomialManager) traceSquareFreeKZero() error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "zp_square_free_factor", "src/math/polynomial/upolynomial_factorization.cpp", 285,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "k = 0")
			return diagnostic.WriteTraceEndl(out)
		})
	return nil
}

func (m *UPolynomialManager) traceSquareFreeFactor(q *upolyFactorFieldScope, value UPolynomialView, multiplicity uint32) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "zp_square_free_factor", "src/math/polynomial/upolynomial_factorization.cpp", 352,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "Factor: (")
			if e := m.traceFactorFieldPolynomial(out, value, &q.polynomial); e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, ")^")
			_ = linearWriteText(out, strconv.FormatUint(uint64(multiplicity), 10))
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}

// Source: upolynomial_factorization.cpp:369,399. Both result records observe
// the caller's fully published factor collection.
func (m *UPolynomialManager) traceFactorFieldOutcome(q *upolyFactorFieldScope, f UPolynomialView, fs *UPolynomialFactors, line int) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	name, caption := "zp_square_free_factor", "polynomial::square_free_factor("
	if line == 399 {
		name, caption = "zp_factor", "polynomial::factor("
	}
	var computation error
	_ = tags.EmitConditional("polynomial::factorization", name, "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, caption)
			if e := m.traceFactorFieldPolynomial(out, f, &q.polynomial); e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, ") => ")
			if e := fs.displayIntoWithReceipt(factorizationTraceStream{out}, &q.factors); e != nil {
				computation = e
				return e
			}
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}
