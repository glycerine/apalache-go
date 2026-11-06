package numeral

// Source: src/util/mpq.cpp power, including its final unused square.

// Power preserves source rational repeated squaring, including zero^zero=1.
// Unlike integer Power it has no 0/1/2 fast paths. Nonterminating source mask
// inputs fail only at their actual mask wrap, retaining completed publications.
func (m *RationalManager) Power(a *Rational, p uint32, out *Rational) error {
	return m.powerWithAllocationContext(a, p, out, integerAllocationInvocationContext{})
}
func (m *RationalManager) powerWithAllocationContext(a *Rational, p uint32, out *Rational, context integerAllocationInvocationContext) (_rawErr error) {
	return m.powerWithAllocationContextAndReceipt(a, p, out, context, nil)
}
func (m *RationalManager) powerWithAllocationContextAndReceipt(a *Rational, p uint32, out *Rational, context integerAllocationInvocationContext, receipt *RationalPowerInvocationReceipt) (_rawErr error) {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	if e := m.checkScalarValue(out); e != nil {
		return e
	}
	r := &m.RawArithmetic
	if r.PowerActive {
		return ErrRationalRawArithmeticPending
	}
	generation, e := m.beginRawGeneration()
	if e != nil {
		return e
	}
	r.PowerGeneration = generation
	if c := context.rationalServiceFrame(m.Z); c != nil {
		c.powerGeneration = generation
	}
	r.Power = NewRational()
	r.PowerActive = true
	r.powerEnd = false
	r.powerGuard = false
	r.powerCursor = 0
	r.powerCause = nil
	power := &r.Power
	if receipt != nil {
		*receipt = RationalPowerInvocationReceipt{manager: m, generation: generation, executing: true}
		defer func() {
			if r.PowerGeneration == generation && r.powerEnd && r.powerCause != nil {
				receipt.sourceCause = r.powerCause
			}
			receipt.executing = false
		}()
	}
	defer m.finishRawPower(&_rawErr)
	if e := m.copyWithAllocationContext(power, a, context); e != nil {
		return e
	}
	if e := m.setInt64WithAllocationContext(out, 1, context); e != nil {
		return e
	}
	for mask := uint32(1); mask <= p; mask <<= 1 {
		if mask == 0 {
			return &PowerDomainError{Exponent: p, Reason: PowerMaskOverflow}
		}
		if mask&p != 0 {
			if e := m.mulWithAllocationContext(out, power, out, context); e != nil {
				return e
			}
		}
		if e := m.mulWithAllocationContext(power, power, power, context); e != nil {
			return e
		}
	}
	return m.rawPowerEnd()
}
