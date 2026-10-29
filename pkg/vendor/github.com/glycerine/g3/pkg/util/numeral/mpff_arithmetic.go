package numeral

// Source MPFF arithmetic/control.

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (m *FastFloatManager) minSig(a *FastFloat) bool {
	s := m.sig(a)
	if s.read(m.precision-1) != 0x80000000 {
		return false
	}
	s.size--
	return ffIsZero(s)
}
func (m *FastFloatManager) setMinSig(a *FastFloat) {
	s := m.sig(a)
	ffZero(s)
	s.write(m.precision-1, 0x80000000)
}
func (m *FastFloatManager) setMaxSig(a *FastFloat) {
	s := m.sig(a)
	for i := uint32(0); i < s.size; i++ {
		s.write(i, ^uint32(0))
	}
}
func (m *FastFloatManager) SetPlusEpsilon(a *FastFloat) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.allocate(a); e != nil {
		return e
	}
	a.sign(false)
	a.exponent = -2147483648
	m.setMinSig(a)
	return m.debugCheck(a)
}
func (m *FastFloatManager) SetMinusEpsilon(a *FastFloat) error {
	if e := m.SetPlusEpsilon(a); e != nil {
		return e
	}
	a.sign(true)
	return m.debugCheck(a)
}
func (m *FastFloatManager) SetMax(a *FastFloat) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.allocate(a); e != nil {
		return e
	}
	a.sign(false)
	a.exponent = 2147483647
	m.setMaxSig(a)
	return m.debugCheck(a)
}
func (m *FastFloatManager) SetMin(a *FastFloat) error {
	if e := m.SetMax(a); e != nil {
		return e
	}
	a.sign(true)
	return m.debugCheck(a)
}
func (m *FastFloatManager) bigExponent(a *FastFloat, e int64) error {
	if e > 2147483647 {
		if a.negative() {
			if m.toPlus {
				return m.SetMin(a)
			}
		} else {
			if !m.toPlus {
				return m.SetMax(a)
			}
		}
		return ErrFastFloatOverflow
	}
	if a.negative() {
		if m.toPlus {
			return m.Reset(a)
		}
		return m.SetMinusEpsilon(a)
	}
	if m.toPlus {
		return m.SetPlusEpsilon(a)
	}
	return m.Reset(a)
}
func (m *FastFloatManager) exponent64(a *FastFloat, e int64) error {
	if e > 2147483647 || e < -2147483648 {
		return m.bigExponent(a, e)
	}
	a.exponent = int32(e)
	return nil
}
func (m *FastFloatManager) incSigView(s ffLimbView, e *int64) {
	if !ffInc(s) {
		s.write(m.precision-1, 0x80000000)
		*e++
	}
}
func (m *FastFloatManager) incSig(a *FastFloat) error {
	if !ffInc(m.sig(a)) {
		m.sig(a).write(m.precision-1, 0x80000000)
		if a.exponent == 2147483647 {
			return ErrFastFloatOverflow
		}
		a.exponent++
	}
	return nil
}
func (m *FastFloatManager) decSig(a *FastFloat) error {
	if SourceExtendedGCDDebug && !(a.index() != 0 && !(a.exponent == -2147483648 && m.minSig(a))) {
		return ErrFastFloatInvariant
	}
	s := m.sig(a)
	for i := uint32(0); i < m.precision-1; i++ {
		w := s.read(i) - 1
		s.write(i, w)
		if w != ^uint32(0) {
			return nil
		}
	}
	w := s.read(m.precision-1) - 1
	s.write(m.precision-1, w)
	if w < 0x80000000 {
		s.write(m.precision-1, ^uint32(0))
		a.exponent--
	}
	return nil
}
func (m *FastFloatManager) Next(a *FastFloat) error {
	if e := m.check(a); e != nil {
		return e
	}
	var e error
	if a.index() == 0 {
		e = m.SetPlusEpsilon(a)
	} else if a.negative() && a.exponent == -2147483648 && m.minSig(a) {
		e = m.Reset(a)
	} else if !a.negative() {
		e = m.incSig(a)
	} else {
		e = m.decSig(a)
	}
	if e != nil {
		return e
	}
	return m.debugCheck(a)
}
func (m *FastFloatManager) Prev(a *FastFloat) error {
	if e := m.check(a); e != nil {
		return e
	}
	var e error
	if a.index() == 0 {
		e = m.SetMinusEpsilon(a)
	} else if !a.negative() && a.exponent == -2147483648 && m.minSig(a) {
		e = m.Reset(a)
	} else if !a.negative() {
		e = m.decSig(a)
	} else {
		e = m.incSig(a)
	}
	if e != nil {
		return e
	}
	return m.debugCheck(a)
}
func (m *FastFloatManager) addSub(sub bool, a, b, c *FastFloat) (err error) {
	if e := m.triple(a, b, c); e != nil {
		return e
	}
	m.traceAddSubOuter(a, b, c, sub, true)
	defer func() {
		if err == nil {
			m.traceAddSubOuter(a, b, c, sub, false)
		}
	}()
	if a.index() == 0 {
		if e := m.Copy(c, b); e != nil {
			return e
		}
		if sub {
			return m.Neg(c)
		}
		return nil
	}
	if b.index() == 0 {
		return m.Copy(c, a)
	}
	m.traceAddSubInner(a, b, c, sub, true)
	if e := m.allocate(c); e != nil {
		return e
	}
	x, y := a, b
	sa, sb := a.negative(), b.negative()
	if sub {
		sb = !sb
	}
	if a.exponent < b.exponent {
		x, y = b, a
		sa, sb = sb, sa
	}
	ea, eb := x.exponent, y.exponent
	sx, sy := m.sig(x), m.sig(y)
	if ea > eb {
		shift := uint32(ea) - uint32(eb)
		v := m.buffer(0, m.precision)
		ffShift(sy, v, shift, false)
		if sb != m.toPlus && ffHasLow(sy, shift) {
			if !ffInc(v) {
				return ErrFastFloatInvariant
			}
		}
		sy = v
	}
	if sa == sb {
		c.sign(sa)
		r := m.buffer(1, m.precision+1)
		ffAdd(sx, sy, r)
		lz := ffNLZ(r)
		if SourceExtendedGCDDebug && !(lz >= 31 && lz < 32*(m.precision+1)) {
			return ErrFastFloatInvariant
		}
		dest := m.sig(c)
		if lz == 32 {
			c.exponent = ea
			ffCopy(r, dest)
		} else if lz == 31 {
			e := int64(ea) + 1
			inc := c.negative() != m.toPlus && ffHasLow(r, 1)
			ffShift(r, dest, 1, false)
			if inc {
				m.incSigView(dest, &e)
			}
			if x := m.exponent64(c, e); x != nil {
				return x
			}
		} else {
			if SourceExtendedGCDDebug && !(lz > 32 && lz < 32*(m.precision+1)) {
				return ErrFastFloatInvariant
			}
			lz -= 32
			ffShift(r, dest, lz, true)
			if e := m.exponent64(c, int64(ea)-int64(lz)); e != nil {
				return e
			}
		}
	} else {
		dest := m.sig(c)
		var borrow uint32
		if ffCompare(sx, sy) < 0 {
			c.sign(sb)
			borrow = ffSub(sy, sx, dest)
		} else {
			c.sign(sa)
			borrow = ffSub(sx, sy, dest)
		}
		if SourceExtendedGCDDebug && !(borrow == 0) {
			return ErrFastFloatInvariant
		}
		lz := ffNLZ(dest)
		if lz == m.bits {
			if e := m.Reset(c); e != nil {
				return e
			}
		}
		if lz > 0 && lz != m.bits {
			ffShift(dest, dest, lz, true)
			if e := m.exponent64(c, int64(ea)-int64(lz)); e != nil {
				return e
			}
		} else if lz == 0 {
			c.exponent = ea
		}
	}
	m.traceAddSubInner(a, b, c, sub, false)
	return m.debugCheck(c)
}
func (m *FastFloatManager) Add(a, b, c *FastFloat) error { return m.addSub(false, a, b, c) }
func (m *FastFloatManager) Sub(a, b, c *FastFloat) error { return m.addSub(true, a, b, c) }
func (m *FastFloatManager) Inc(a *FastFloat) error       { return m.Add(a, &m.one, a) }
func (m *FastFloatManager) Dec(a *FastFloat) error       { return m.Sub(a, &m.one, a) }
func (m *FastFloatManager) Mul(a, b, c *FastFloat) (err error) {
	if e := m.triple(a, b, c); e != nil {
		return e
	}
	m.traceMulOuter(a, b, c, true)
	defer func() {
		if err == nil {
			m.traceMulOuter(a, b, c, false)
		}
	}()
	if a.index() == 0 || b.index() == 0 {
		return m.Reset(c)
	}
	if e := m.allocate(c); e != nil {
		return e
	}
	m.traceMulEntry(a, b)
	c.sign(a.negative() != b.negative())
	exp := int64(a.exponent) + int64(b.exponent)
	r := m.buffer(0, 2*m.precision)
	ffMul(m.sig(a), m.sig(b), r)
	lz := ffNLZ(r)
	if SourceExtendedGCDDebug && !(lz <= m.bits) {
		return ErrFastFloatInvariant
	}
	m.traceMulLeading(lz)
	shift := m.bits - lz
	inc := c.negative() != m.toPlus && ffHasLow(r, shift)
	m.traceMulRounding(c, r, shift, inc)
	exp += int64(shift)
	dest := m.sig(c)
	ffShift(r, dest, shift, false)
	if inc {
		m.incSigView(dest, &exp)
	}
	if e := m.exponent64(c, exp); e != nil {
		return e
	}
	m.traceMulResult(c)
	return m.debugCheck(c)
}
func (m *FastFloatManager) toBuffer(i int, a *FastFloat, extension, shift bool) {
	b := m.buffer(i, 2*m.precision)
	s := m.sig(a)
	if shift {
		ffShift(s, b, m.bits, true)
	} else {
		if !extension {
			b.size = m.precision
		}
		ffCopy(s, b)
	}
}
func (m *FastFloatManager) Div(a, b, c *FastFloat) (err error) {
	if e := m.triple(a, b, c); e != nil {
		return e
	}
	if b.index() == 0 {
		return ErrFastFloatDivisionByZero
	}
	m.traceDivOuter(a, b, c, true)
	defer func() {
		if err == nil {
			m.traceDivOuter(a, b, c, false)
		}
	}()
	if a.index() == 0 {
		return m.Reset(c)
	}
	two, _ := m.IsTwo(b)
	if two {
		if e := m.Copy(c, a); e != nil {
			return e
		}
		if e := m.exponent64(c, int64(a.exponent)-1); e != nil {
			return e
		}
		return m.debugCheck(c)
	}
	if e := m.allocate(c); e != nil {
		return e
	}
	c.sign(a.negative() != b.negative())
	exp := int64(a.exponent) - int64(b.exponent) - int64(m.bits)
	m.toBuffer(0, a, true, true)
	ffCopy(m.sig(b), m.buffer(3, m.precision))
	if SourceExtendedGCDDebug && !(!ffIsZero(m.buffer(0, 2*m.precision)) && !ffIsZero(m.sig(b)) && ffNLZ(m.buffer(0, 2*m.precision)) == 0) {
		return ErrFastFloatInvariant
	}
	av := limbView{external: m.buffers[0], size: 2 * m.precision}
	bv := limbView{external: m.buffers[3], size: m.precision}
	qv := limbView{external: m.buffers[1], size: m.precision + 1}
	rv := limbView{external: m.buffers[2], size: m.precision}
	if _, e := m.natural.divide(av, bv, qv, rv); e != nil {
		return e
	}
	q, r := m.buffer(1, m.precision+1), m.buffer(2, m.precision)
	m.traceDivQuotient(q)
	lz := ffNLZ(q)
	if SourceExtendedGCDDebug && !(!ffIsZero(q) && lz < q.size*32) {
		return ErrFastFloatInvariant
	}
	actual := q.size*32 - lz
	dest := m.sig(c)
	inc := false
	if actual > m.bits {
		shift := actual - m.bits
		inc = c.negative() != m.toPlus && (ffHasLow(q, shift) || !ffIsZero(r))
		exp += int64(shift)
		ffShift(q, dest, shift, false)
	} else {
		inc = c.negative() != m.toPlus && !ffIsZero(r)
		if actual < m.bits {
			shift := m.bits - actual
			exp -= int64(shift)
			ffShift(q, dest, shift, true)
		} else {
			ffCopy(q, dest)
		}
	}
	if inc {
		m.incSigView(dest, &exp)
	}
	if e := m.exponent64(c, exp); e != nil {
		return e
	}
	return m.debugCheck(c)
}
func (m *FastFloatManager) InvInPlace(a *FastFloat) error { return m.Div(&m.one, a, a) }
func (m *FastFloatManager) Inv(a, c *FastFloat) error {
	if e := m.Copy(c, a); e != nil {
		return e
	}
	return m.InvInPlace(c)
}
func (m *FastFloatManager) roundInteger(a *FastFloat, ceil bool) (err error) {
	if e := m.check(a); e != nil {
		return e
	}
	if a.exponent >= 0 {
		return nil
	}
	m.traceRoundInteger(a, ceil, true)
	defer func() {
		if err == nil {
			m.traceRoundInteger(a, ceil, false)
		}
	}()
	if a.exponent <= -int32(m.bits) {
		if a.negative() != ceil {
			if ceil {
				err = m.SetInt32(a, 1)
			} else {
				err = m.SetInt32(a, -1)
			}
		} else {
			err = m.Reset(a)
		}
		if err != nil {
			return err
		}
		return m.debugCheck(a)
	}
	s := m.sig(a)
	k := uint32(-a.exponent)
	if a.negative() != ceil && ffHasLow(s, k) {
		ffShift(s, s, k, false)
		if !ffInc(s) {
			return ErrFastFloatInvariant
		}
		lz := ffNLZ(s)
		if SourceExtendedGCDDebug && !(lz == k || lz == k-1) {
			return ErrFastFloatInvariant
		}
		if lz != k {
			ffShift(s, s, k-1, true)
			a.exponent++
		} else {
			ffShift(s, s, k, true)
		}
	} else {
		ffShift(s, s, k, false)
		ffShift(s, s, k, true)
	}
	return m.debugCheck(a)
}
func (m *FastFloatManager) FloorInPlace(a *FastFloat) error { return m.roundInteger(a, false) }
func (m *FastFloatManager) CeilInPlace(a *FastFloat) error  { return m.roundInteger(a, true) }
func (m *FastFloatManager) Floor(a, c *FastFloat) error {
	if e := m.Copy(c, a); e != nil {
		return e
	}
	return m.FloorInPlace(c)
}
func (m *FastFloatManager) Ceil(a, c *FastFloat) error {
	if e := m.Copy(c, a); e != nil {
		return e
	}
	return m.CeilInPlace(c)
}
func (m *FastFloatManager) Power(a *FastFloat, p uint32, b *FastFloat) (err error) {
	if e := m.pair(a, b); e != nil {
		return e
	}
	if e := m.debugCheck(a); e != nil {
		return e
	}
	original := a
	tags := m.mulTraceTags()
	m.powerExecuting = true
	defer func() {
		if cleanup := m.cleanupPowerWork(); err == nil {
			err = cleanup
		}
		if err == nil {
			m.tracePower(original, p, b)
		}
		if !m.powerWorkPending {
			if cleanup := m.cleanupPowerTrace(); err == nil {
				err = cleanup
			}
		}
		m.powerExecuting = false
	}()
	if a == b && tags.ShouldTrace("mpff_trace") {
		m.powerTrace = FastFloat{}
		m.powerTracePending = true
		if err = m.Copy(&m.powerTrace, a); err != nil {
			return err
		}
		original = &m.powerTrace
	}
	if a.index() == 0 {
		if SourceExtendedGCDDebug && !(p != 0) {
			return ErrFastFloatInvariant
		}
		return m.Reset(b)
	}
	if p == 0 {
		return m.SetInt32(b, 1)
	}
	if p == 1 {
		return m.Copy(b, a)
	}
	if p == 2 {
		return m.Mul(a, a, b)
	}
	if p <= 8 && a != b {
		if e := m.Copy(b, a); e != nil {
			return e
		}
		for i := p - 1; i > 0; i-- {
			if e := m.Mul(a, b, b); e != nil {
				return e
			}
		}
		return m.debugCheck(b)
	}
	if m.minSig(a) {
		if e := m.allocate(b); e != nil {
			return e
		}
		b.sign(p%2 != 0 && a.negative())
		exp := int64(a.exponent) * int64(p)
		if exp > 2147483647 || exp < -2147483648 {
			return ErrFastFloatOverflow
		}
		exp += int64((m.bits - 1) * (p - 1))
		if exp > 2147483647 || exp < -2147483648 {
			return ErrFastFloatOverflow
		}
		m.setMinSig(b)
		b.exponent = int32(exp)
		return m.debugCheck(b)
	}
	m.powerWork = FastFloat{}
	m.powerWorkPending = true
	if e := m.Copy(&m.powerWork, a); e != nil {
		return e
	}
	if e := m.SetInt32(b, 1); e != nil {
		return e
	}
	mask := uint32(1)
	for mask <= p {
		if mask == 0 {
			return ErrFastFloatPowerDomain
		}
		if mask&p != 0 {
			if e := m.Mul(b, &m.powerWork, b); e != nil {
				return e
			}
		}
		if e := m.Mul(&m.powerWork, &m.powerWork, &m.powerWork); e != nil {
			return e
		}
		mask <<= 1
	}
	return m.debugCheck(b)
}

// Conversion/setup bounded source vectors report arena bounds separately.
var _ = arena.ErrBounds
