package numeral

// Source MPFF conversions, internal32.

// Selected source MPZ digit setter over a concrete ephemeral owner-indexed view.
// Retains existing MPZ storage staging; no temporary unbudgeted limb slice.
func ffSetInteger(z *IntegerManager, a *Int, v ffLimbView) error {
	if e := z.check(a); e != nil {
		return e
	}
	for v.size > 0 && v.read(v.size-1) == 0 {
		v.size--
	}
	if v.size == 0 {
		return z.Reset(a)
	}
	if v.size == 1 {
		return z.SetUint32(a, v.read(0))
	}
	ref := a.Cell
	if ref == 0 || z.Cell(ref).Capacity < v.size {
		var e error
		ref, e = z.Allocate(max(v.size, 6), v.size)
		if e != nil {
			return e
		}
	}
	z.ResizeCell(ref, v.size)
	for i := uint32(0); i < v.size; i++ {
		z.writeDigit(ref, i, v.read(i))
	}
	old := a.Cell
	a.Cell = ref
	z.PublishCell(ref)
	a.Val = 1
	if old != 0 && old != ref {
		if e := z.free(old); e != nil {
			return e
		}
	}
	return nil
}
func (m *FastFloatManager) decompose(z *IntegerManager, a *Int) (bool, error) {
	if e := m.resize(&m.setBuffer, 0); e != nil {
		return false, e
	}
	size := uint32(1)
	if a.Cell != 0 {
		size = z.Cell(a.Cell).Size
	}
	for i := uint32(0); i < size; i++ {
		v := uint32(0)
		if a.Cell == 0 {
			n := int64(a.Val)
			if n < 0 {
				n = -n
			}
			v = uint32(n)
		} else {
			v = z.Digit(a.Cell, i)
		}
		if e := m.resize(&m.setBuffer, i+1); e != nil {
			return false, e
		}
		m.write(&m.setBuffer, i, v)
	}
	return a.Val < 0, nil
}
func (m *FastFloatManager) SetInteger(a *FastFloat, z *IntegerManager, v *Int) error {
	if e := m.check(a); e != nil {
		return e
	}
	if z == nil {
		return ErrFastFloatOwner
	}
	if e := z.check(v); e != nil {
		return e
	}
	m.traceSetIntegerInput(z, v)
	ok, e := z.IsInt64(v)
	if e != nil {
		return e
	}
	if ok {
		n, e := z.Int64(v)
		if e != nil {
			return e
		}
		m.traceSetIntegerSigned(n)
		if e = m.SetInt64(a, n); e != nil {
			return e
		}
		m.traceSetIntegerResult(a)
		return m.debugCheck(a)
	}
	ok, e = z.IsUint64(v)
	if e != nil {
		return e
	}
	if ok {
		n, e := z.Uint64(v)
		if e != nil {
			return e
		}
		m.traceSetIntegerUnsigned()
		if e = m.SetUint64(a, n); e != nil {
			return e
		}
		m.traceSetIntegerResult(a)
		return m.debugCheck(a)
	}
	if e = m.allocate(a); e != nil {
		return e
	}
	negative, e := m.decompose(z, v)
	if e != nil {
		return e
	}
	a.sign(negative)
	for uint32(len(m.setBuffer)) < m.precision {
		if e = m.resize(&m.setBuffer, uint32(len(m.setBuffer))+1); e != nil {
			return e
		}
		m.write(&m.setBuffer, uint32(len(m.setBuffer))-1, 0)
	}
	w := ffLimbView{m: m, v: &m.setBuffer, size: uint32(len(m.setBuffer))}
	m.traceSetIntegerWords(w)
	lz := ffNLZ(w)
	if SourceExtendedGCDDebug && !(lz != 0) {
		return ErrFastFloatInvariant
	}
	ffShift(w, w, lz, true)
	s := m.sig(a)
	j := w.size
	for i := m.precision; i > 0; {
		i--
		j--
		s.write(i, w.read(j))
	}
	a.exponent = int32(j*32 - lz)
	if a.negative() != m.toPlus {
		for j > 0 {
			j--
			if w.read(j) != 0 {
				if e = m.incSig(a); e != nil {
					return e
				}
			}
		}
	}
	m.traceSetIntegerResult(a)
	return m.debugCheck(a)
}
func (m *FastFloatManager) SetRational(a *FastFloat, qm *RationalManager, v *Rational) (err error) {
	if e := m.check(a); e != nil {
		return e
	}
	if qm == nil {
		return ErrFastFloatOwner
	}
	if e := qm.check(v); e != nil {
		return e
	}
	m.rationalNum, m.rationalDen = FastFloat{}, FastFloat{}
	m.rationalPending = true
	m.rationalCursor = 0
	m.rationalExecuting = true
	defer func() {
		if cleanup := m.cleanupSetRationalTemporaries(); err == nil {
			err = cleanup
		}
		m.rationalExecuting = false
	}()
	if e := m.SetInteger(&m.rationalNum, qm.Z, &v.Num); e != nil {
		return e
	}
	e := func() error {
		old := m.toPlus
		m.toPlus = !old
		defer func() { m.toPlus = old }()
		return m.SetInteger(&m.rationalDen, qm.Z, &v.Den)
	}()
	if e != nil {
		return e
	}
	return m.Div(&m.rationalNum, &m.rationalDen, a)
}
func (m *FastFloatManager) Significand(a *FastFloat, z *IntegerManager, out *Int) error {
	if e := m.check(a); e != nil {
		return e
	}
	if z == nil {
		return ErrFastFloatOwner
	}
	return ffSetInteger(z, out, m.sig(a))
}
func (m *FastFloatManager) ToInteger(a *FastFloat, z *IntegerManager, out *Int) error {
	if e := m.check(a); e != nil {
		return e
	}
	if z == nil {
		return ErrFastFloatOwner
	}
	if e := z.check(out); e != nil {
		return e
	}
	if SourceExtendedGCDDebug {
		ok, e := m.IsInteger(a)
		if e != nil {
			return e
		}
		if !ok {
			return ErrFastFloatInvariant
		}
	}
	var e error
	exp := a.exponent
	if exp < 0 {
		if SourceExtendedGCDDebug && exp <= -int32(m.bits) {
			return ErrFastFloatInvariant
		}
		m.toBuffer(0, a, false, false)
		// Source negates an int32 here. INT_MIN is undefined signed
		// arithmetic; retain its completed buffer copy before diagnosing it.
		if exp == -2147483648 {
			return ErrFastFloatDomain
		}
		v := m.buffer(0, m.precision)
		ffShift(v, v, uint32(-exp), false)
		if e = ffSetInteger(z, out, v); e != nil {
			return e
		}
	} else {
		if e = ffSetInteger(z, out, m.sig(a)); e != nil {
			return e
		}
		if exp > 0 {
			if e = m.multiplyToIntegerPower(z, out, uint32(exp)); e != nil {
				return e
			}
		}
	}
	if a.negative() {
		return z.Neg(out)
	}
	return nil
}

// The source p scope ends before the final sign change in ToInteger.
func (m *FastFloatManager) multiplyToIntegerPower(z *IntegerManager, out *Int, exp uint32) (err error) {
	m.integerPower = NewInt(2)
	m.integerPowerOwner = z
	m.integerPowerPending = true
	m.integerExecuting = true
	defer func() {
		if cleanup := m.cleanupToIntegerPower(true); err == nil {
			err = cleanup
		}
		m.integerExecuting = false
	}()
	if err = z.Power(&m.integerPower, exp, &m.integerPower); err != nil {
		return err
	}
	return z.Mul(out, &m.integerPower, out)
}
func ffSetRational(qm *RationalManager, out *Rational, v ffLimbView) error {
	if e := qm.check(out); e != nil {
		return e
	}
	if e := ffSetInteger(qm.Z, &out.Num, v); e != nil {
		return e
	}
	return qm.resetDenominator(out)
}
func (m *FastFloatManager) ToRational(a *FastFloat, qm *RationalManager, out *Rational) error {
	if e := m.check(a); e != nil {
		return e
	}
	if qm == nil {
		return ErrFastFloatOwner
	}
	if e := qm.check(out); e != nil {
		return e
	}
	exp := a.exponent
	m.traceToRational(a, exp)
	if exp < 0 && exp > -int32(m.bits) && !ffHasLow(m.sig(a), uint32(-exp)) {
		m.toBuffer(0, a, false, false)
		v := m.buffer(0, m.precision)
		ffShift(v, v, uint32(-exp), false)
		if e := ffSetRational(qm, out, v); e != nil {
			return e
		}
	} else {
		if e := ffSetRational(qm, out, m.sig(a)); e != nil {
			return e
		}
		if exp != 0 {
			if e := m.multiplyToRationalPower(qm, out, exp); e != nil {
				return e
			}
		}
	}
	if a.negative() {
		return qm.Neg(out)
	}
	return nil
}

// The source p scope ends before the final sign change in ToRational.
func (m *FastFloatManager) multiplyToRationalPower(qm *RationalManager, out *Rational, exp int32) (err error) {
	m.rationalPower = NewRational()
	m.rationalPowerOwner = qm
	m.rationalPowerPending = true
	m.rationalPowerCursor = 0
	m.rationalPowerExecuting = true
	defer func() {
		if cleanup := m.cleanupToRationalPower(true); err == nil {
			err = cleanup
		}
		m.rationalPowerExecuting = false
	}()
	if err = qm.SetInt64(&m.rationalPower, 2); err != nil {
		return err
	}
	power := uint32(exp)
	if exp < 0 {
		power = uint32(-int64(exp))
	}
	if err = qm.Power(&m.rationalPower, power, &m.rationalPower); err != nil {
		return err
	}
	if exp < 0 {
		return qm.Div(out, &m.rationalPower, out)
	}
	return qm.Mul(out, &m.rationalPower, out)
}
