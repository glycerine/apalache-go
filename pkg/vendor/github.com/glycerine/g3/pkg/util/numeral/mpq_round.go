package numeral

// Source: src/util/mpq.cpp floor/ceil/prev_power_of_two and mpq.h wrappers.

import umemory "github.com/glycerine/g3/pkg/util/memory"

// roundCore retains the source sign-only correction, including unreduced or
// negative-denominator raw values. It captures numerator sign before division
// publishes into a caller that may alias either input component.
func (m *RationalManager) roundCore(a *Rational, out *Int, ceil bool) error {
	return m.roundCoreWithAllocationContext(a, out, ceil, integerAllocationInvocationContext{})
}
func (m *RationalManager) roundCoreWithAllocationContext(a *Rational, out *Int, ceil bool, context integerAllocationInvocationContext) error {
	if isIntegerRational(a) {
		return m.Z.copyFromOwnerWithAllocationContext(out, m.Z, &a.Num, nil, context)
	}
	correct := a.Num.Val < 0
	if ceil {
		correct = a.Num.Val > 0
	}
	if e := m.Z.machineDivWithAllocationContext(&a.Num, &a.Den, out, context); e != nil {
		return e
	}
	if correct {
		one := NewInt(1)
		if ceil {
			return m.Z.addSubWithAllocationContext(out, &one, out, false, context)
		}
		return m.Z.addSubWithAllocationContext(out, &one, out, true, context)
	}
	return nil
}
func (m *RationalManager) roundToInt(a *Rational, out *Int, ceil bool) error {
	return m.roundToIntWithAllocationContext(a, out, ceil, integerAllocationInvocationContext{})
}
func (m *RationalManager) roundToIntWithAllocationContext(a *Rational, out *Int, ceil bool, context integerAllocationInvocationContext) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.Z.check(out); e != nil {
		return e
	}
	return m.roundCoreWithAllocationContext(a, out, ceil, context)
}

// FloorToInt and CeilToInt publish the source Copy/quotient into the caller
// before sign correction. Output may alias either input component; errors
// retain completed source actions rather than restoring the old caller.
func (m *RationalManager) FloorToInt(a *Rational, out *Int) error { return m.roundToInt(a, out, false) }
func (m *RationalManager) CeilToInt(a *Rational, out *Int) error  { return m.roundToInt(a, out, true) }
func (m *RationalManager) round(a, out *Rational, ceil bool) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	if e := m.roundCore(a, &out.Num, ceil); e != nil {
		return e
	}
	return m.resetDenominator(out)
}

// Floor and Ceil are the source rational-output wrappers, with denominator 1.
func (m *RationalManager) Floor(a, out *Rational) error { return m.round(a, out, false) }
func (m *RationalManager) Ceil(a, out *Rational) error  { return m.round(a, out, true) }
func (m *RationalManager) PrevPowerOfTwo(a *Rational) (uint32, error) {
	if m == nil || m.self != m {
		return 0, umemory.ErrRuntimeOwner
	}
	if m.prevPowerPending {
		return 0, ErrRationalLexicalPending
	}
	if e := m.checkOwnerReady(); e != nil {
		return 0, e
	}
	var tmp Int
	var value uint32
	var bodyErr error
	if bodyErr = m.FloorToInt(a, &tmp); bodyErr == nil {
		value, bodyErr = m.Z.PrevPowerOfTwo(&tmp)
	}
	if _, cleanupErr := sourceNumeralDel(m.Z, &tmp); cleanupErr != nil {
		m.prevPowerTemporary = tmp
		m.prevPowerPending = true
		if bodyErr == nil {
			return value, cleanupErr
		}
	}
	return value, bodyErr
}

// CleanupPrevPowerOfTwo continues ownership of the exact scoped integer when
// its source destructor stopped. No second source destructor is invoked.
func (m *RationalManager) CleanupPrevPowerOfTwo() error {
	if m == nil || m.self != m || m.Z == nil {
		return umemory.ErrRuntimeOwner
	}
	if !m.prevPowerPending {
		return nil
	}
	if e := m.Z.Del(&m.prevPowerTemporary); e != nil {
		return e
	}
	m.prevPowerTemporary = Int{}
	m.prevPowerPending = false
	return nil
}
