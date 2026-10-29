package polynomial

// Source: src/math/polynomial/polynomial.cpp:362-367.

import "io"

func (m *MonomialManager) tracePredecessorResult(a Monomial, x, y, i uint32) error {
	if m.queryTraceRunning {
		return ErrMonomialPending
	}
	tags := m.monomialTraceTags()
	if !tags.ShouldTrace("poly_bug") {
		return nil
	}
	m.queryTraceRunning = true
	defer func() { m.queryTraceRunning = false }()
	var operandErr error
	_ = tags.EmitConditional("poly_bug", "max_smaller_than", "src/math/polynomial/polynomial.cpp", 362,
		func() bool {
			if y == ^uint32(0) {
				return true
			}
			v, e := m.varCore(a, i)
			operandErr = e
			return e == nil && v > y
		}, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("m: ")
			operandErr = m.DisplayInto(a, sink, false)
			if operandErr != nil {
				return operandErr
			}
			_ = sink.WriteLiteral("\nx: ")
			_ = sink.WriteUnsigned(x)
			_ = sink.WriteLiteral("\ny: ")
			_ = sink.WriteUnsigned(y)
			_ = sink.WriteLiteral("\ni: ")
			_ = sink.WriteUnsigned(i)
			_ = sink.WriteLiteral("\nget_var(i): ")
			v, e := m.varCore(a, i)
			if e != nil {
				operandErr = e
				return e
			}
			_ = sink.WriteUnsigned(v)
			return sink.WriteLiteral("\n")
		})
	return operandErr
}
