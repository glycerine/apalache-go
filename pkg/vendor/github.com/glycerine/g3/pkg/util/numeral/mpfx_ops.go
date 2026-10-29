package numeral

// Source: src/util/mpfx.cpp:558-650 and src/util/mpfx.h:260-278.

import "math/big"

// Floor rounds an owned fixed-point value toward minus infinity.
func (m *FixedPointManager) Floor(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	m.traceFixedRoundIntegral(a, false, false)
	var q, rem, rounded big.Int
	q.QuoRem(&a.scaled, &m.scale, &rem)
	if rem.Sign() != 0 && a.scaled.Sign() < 0 {
		q.Sub(&q, big.NewInt(1))
	}
	rounded.Mul(&q, &m.scale)
	if err := m.publish(a, &rounded); err != nil {
		// Source clears the fractional words before inc wraps every integer
		// word to zero and reports overflow.
		a.scaled.SetInt64(0)
		a.sourceZeroSlot = true
		a.sourceZeroNegative = true
		return err
	}
	m.traceFixedRoundIntegral(a, false, true)
	return nil
}

// Ceil rounds an owned fixed-point value toward plus infinity.
func (m *FixedPointManager) Ceil(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	m.traceFixedRoundIntegral(a, true, false)
	var q, rem, rounded big.Int
	q.QuoRem(&a.scaled, &m.scale, &rem)
	if rem.Sign() != 0 && a.scaled.Sign() > 0 {
		q.Add(&q, big.NewInt(1))
	}
	rounded.Mul(&q, &m.scale)
	if err := m.publish(a, &rounded); err != nil {
		// The in-place integer-word increment wraps after the source has
		// cleared all fractional words.
		a.scaled.SetInt64(0)
		a.sourceZeroSlot = true
		a.sourceZeroNegative = false
		return err
	}
	m.traceFixedRoundIntegral(a, true, true)
	return nil
}

// Inv uses the manager's current directed rounding for 1/a. The source
// numeral manager invokes division by its retained one numeral.
func (m *FixedPointManager) Inv(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	if a.scaled.Sign() == 0 {
		return ErrFixedPointDivZero
	}
	m.traceFixedInverse(a, false)
	var oneScaled big.Int
	oneScaled.Mul(&m.scale, &m.scale)
	q, err := m.roundMagnitude(&oneScaled, &a.scaled)
	if err != nil {
		return err
	}
	if err := m.publish(a, q); err != nil {
		return err
	}
	m.traceFixedInverse(a, true)
	return nil
}

// Power preserves the source's left-to-right small-power path for distinct
// operands and repeated-square path for aliased or larger powers. Directed
// rounding occurs after each multiplication, not only at the final result.
func (m *FixedPointManager) Power(a *FixedPoint, exponent uint32, out *FixedPoint) (err error) {
	if err := m.pair(a, out); err != nil {
		return err
	}
	input := m.fixedPowerTraceInput(a)
	defer func() {
		if err == nil {
			m.traceFixedPower(input, exponent, out)
		}
	}()
	if a.sourceIsZero() {
		if exponent == 0 {
			return ErrFixedPointDomain
		}
		return m.Reset(out)
	}
	switch exponent {
	case 0:
		return m.SetInt64(out, 1)
	case 1:
		return m.Copy(out, a)
	case 2:
		return m.Mul(a, a, out)
	}
	if exponent <= 8 && a != out {
		if err := m.Copy(out, a); err != nil {
			return err
		}
		for i := uint32(1); i < exponent; i++ {
			if err := m.Mul(a, out, out); err != nil {
				return err
			}
		}
		return nil
	}
	// The source's 32-bit mask loop would wrap to zero for exponents with
	// the high bit set. Refuse that nonterminating input explicitly.
	if exponent >= 1<<31 {
		return ErrFixedPointDomain
	}
	pw, err := m.New()
	if err != nil {
		return err
	}
	defer pw.Close()
	if err := m.Copy(pw, a); err != nil {
		return err
	}
	if err := m.SetInt64(out, 1); err != nil {
		return err
	}
	for mask := uint32(1); mask <= exponent; mask <<= 1 {
		if mask&exponent != 0 {
			if err := m.Mul(out, pw, out); err != nil {
				return err
			}
		}
		// The source squares even after the highest selected bit.
		if err := m.Mul(pw, pw, pw); err != nil {
			return err
		}
	}
	return nil
}
