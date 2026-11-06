package polynomial

// Source: src/math/polynomial/polynomial.cpp:796-801.

import "io"

func (m *MonomialManager) traceLeakedMonomials() error {
	tags := m.monomialTraceTags()
	if !tags.ShouldTrace("polynomial") {
		return nil
	}
	if m.managerTraceRunning {
		return ErrMonomialPending
	}
	m.managerTraceRunning = true
	defer func() { m.managerTraceRunning = false }()
	var operandErr error
	_ = tags.EmitConditional("polynomial", "~monomial_manager", "src/math/polynomial/polynomial.cpp", 796,
		func() bool { return m.table.entries.Len() != 0 }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("monomials leaked\n")
			for _, entry := range m.table.entries.All() {
				if entry.key.kind != monomialObjectKey || entry.key.slot == 0 || uint64(entry.key.slot) > uint64(len(m.locations)) {
					operandErr = ErrMonomialPending
					return operandErr
				}
				value := Monomial{m.origin, entry.key.slot, m.locations[entry.key.slot-1].generation}
				operandErr = m.displayIntoCore(value, sink, false)
				if operandErr != nil {
					return operandErr
				}
				_ = sink.WriteLiteral("\n")
			}
			return nil
		})
	return operandErr
}
