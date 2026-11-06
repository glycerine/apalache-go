package polynomial

import (
	"fmt"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"strconv"
)

// Source: upolynomial.cpp:624, before set(sz1,p1,_p1). Pointer text projects
// actual native Go storage; no C++ addresses or replacement tokens are invented.
func (m *UPolynomialManager) traceExactDivPointers(a UPolynomialView) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("factor_bug") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	var computation error
	_ = tags.EmitConditional("factor_bug", "exact_div", "src/math/polynomial/upolynomial.cpp", 624, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		_ = linearWriteText(out, "sz1: ")
		_ = linearWriteText(out, strconv.FormatUint(uint64(a.count), 10))
		_ = linearWriteText(out, " p1: ")
		p, e := m.read(a, 0)
		if e != nil {
			computation = e
			return e
		}
		_ = linearWriteText(out, fmt.Sprintf("%p", p))
		_ = linearWriteText(out, ", _p1.c_ptr(): ")
		var previous *unumeral.Int
		values := m.values(&m.div1)
		if len(values) != 0 {
			previous = &values[0]
		}
		_ = linearWriteText(out, fmt.Sprintf("%p", previous))
		_ = linearWriteText(out, ", _p1.size(): ")
		_ = linearWriteText(out, strconv.FormatUint(uint64(m.div1.size), 10))
		return linearWriteText(out, "\n")
	})
	return computation
}
