package polynomial

// Source: polynomial.cpp display, reached by algebraic_numbers.cpp:668 TRACE.

type polynomialDisplayInvocationReceipt struct {
	frame      *polynomialDisplayFrame
	generation uint64
}

func (m *MultivariatePolynomialCore) displayIntoCoreWithReceipt(p Polynomial, sink PolynomialTextSink, star bool, receipt *polynomialDisplayInvocationReceipt) error {
	previous := m.display.receipt
	m.display.receipt = receipt
	defer func() { m.display.receipt = previous }()
	return m.displayIntoCore(p, sink, star)
}
func (m *MultivariatePolynomialCore) matchesDisplayReceipt(r *polynomialDisplayInvocationReceipt) bool {
	if m == nil || m.self != m || r == nil || r.frame == nil || r.frame.owner != m || r.generation == 0 || r.generation != r.frame.generation {
		return false
	}
	for _, issued := range m.display.frames {
		if issued == r.frame {
			return true
		}
	}
	return false
}
func (m *MultivariatePolynomialCore) displayReceiptPending(r *polynomialDisplayInvocationReceipt) bool {
	return m.matchesDisplayReceipt(r) && r.frame.active
}
func (m *MultivariatePolynomialCore) displayReceiptSourceCause(r *polynomialDisplayInvocationReceipt) error {
	if m.matchesDisplayReceipt(r) {
		return r.frame.sourceCause
	}
	return nil
}
func (m *MultivariatePolynomialCore) cleanupDisplayReceipt(r *polynomialDisplayInvocationReceipt) error {
	if !m.matchesDisplayReceipt(r) {
		return nil
	}
	return m.cleanupDisplayFrame(r.frame)
}

func (m *MultivariatePolynomialCore) displayPolynomialDirectWithReceipt(p Polynomial, sink PolynomialTextSink, star bool, receipt *polynomialDisplayInvocationReceipt) error {
	previous := m.display.receipt
	m.display.receipt = receipt
	defer func() { m.display.receipt = previous }()
	return m.displayPolynomialDirect(p, sink, star)
}
