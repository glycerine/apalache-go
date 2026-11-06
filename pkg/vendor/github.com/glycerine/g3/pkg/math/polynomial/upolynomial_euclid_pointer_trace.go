package polynomial

import (
	"fmt"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"strconv"
)

// Source: upolynomial.cpp:833-834, after both actual input copies.
// Pointer text names native Go backing storage, as in exact_div's record.
func (m *UPolynomialManager) traceEuclidPointers(a, b UPolynomialView, B *upolyVectorState) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("upolynomial") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	var computation error
	_ = tags.EmitConditional("upolynomial", "euclid_gcd", "src/math/polynomial/upolynomial.cpp", 833, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		for i, v := range [2]UPolynomialView{a, b} {
			if i == 0 {
				_ = linearWriteText(out, "sz1: ")
			} else {
				_ = linearWriteText(out, ", sz2: ")
			}
			_ = linearWriteText(out, strconv.FormatUint(uint64(v.count), 10))
			if i == 0 {
				_ = linearWriteText(out, ", p1: ")
			} else {
				_ = linearWriteText(out, ", p2: ")
			}
			p, e := m.read(v, 0)
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, fmt.Sprintf("%p", p))
		}
		_ = linearWriteText(out, "\nB.size(): ")
		_ = linearWriteText(out, strconv.FormatUint(uint64(B.size), 10))
		_ = linearWriteText(out, ", B.c_ptr(): ")
		values := m.values(B)
		var first *unumeral.Int
		if len(values) != 0 {
			first = &values[0]
		}
		_ = linearWriteText(out, fmt.Sprintf("%p", first))
		_ = linearWriteText(out, "\n")
		return nil
	})
	return computation
}
