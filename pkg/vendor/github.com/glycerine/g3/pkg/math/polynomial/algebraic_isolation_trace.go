package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:549,568,589,594,607.

import (
	"io"
	"strconv"
)

func (m *AlgebraicRootCore) traceIsolationPolynomial(p UPolynomialView, squareFree bool) error {
	line, function := 568, "isolate_roots"
	if squareFree {
		line, function = 549, "factor"
	}
	return m.emitRootTrace(function, line, func(w io.Writer) error {
		out := upolyRemainderTraceStream{w}
		if e := m.config.Polynomials.displayIntoWithReceipt(p, out, "x", false, &m.rootTrace.univariate); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}
func (m *AlgebraicRootCore) traceIsolationFactor(index, degree uint32) error {
	return m.emitRootTrace("isolate_roots", 589, func(w io.Writer) error {
		out := algebraicPredicateTraceStream{w}
		return out.AppendCString("factor " + strconv.FormatUint(uint64(index), 10) + " degree: " + strconv.FormatUint(uint64(degree), 10) + "\n")
	})
}
func (m *AlgebraicRootCore) traceIsolationLinear() error {
	return m.emitRootTrace("isolate_roots", 594, func(w io.Writer) error {
		return (algebraicPredicateTraceStream{w}).AppendCString("linear polynomial...\n")
	})
}
func (m *AlgebraicRootCore) traceIsolatedBasicRoots(count uint32) error {
	return m.emitRootTrace("isolate_roots", 607, func(w io.Writer) error {
		return (algebraicPredicateTraceStream{w}).AppendCString("isolated roots: " + strconv.FormatUint(uint64(count), 10) + "\n")
	})
}
