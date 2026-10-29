package numeral

// Source MPFF predicates/getters.

func (m *FastFloatManager) Sign(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return a.negative(), nil
}
func (m *FastFloatManager) IsZero(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return a.index() == 0, nil
}
func (m *FastFloatManager) IsPos(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return a.index() != 0 && !a.negative(), nil
}
func (m *FastFloatManager) IsNeg(a *FastFloat) (bool, error) { return m.Sign(a) }
func (m *FastFloatManager) IsNonPos(a *FastFloat) (bool, error) {
	ok, e := m.IsPos(a)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (m *FastFloatManager) IsNonNeg(a *FastFloat) (bool, error) {
	ok, e := m.IsNeg(a)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (m *FastFloatManager) IsAbsOne(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return a.exponent == 1-int32(m.bits) && m.minSig(a), nil
}
func (m *FastFloatManager) IsOne(a *FastFloat) (bool, error) {
	ok, e := m.IsPos(a)
	if e != nil || !ok {
		return false, e
	}
	return m.IsAbsOne(a)
}
func (m *FastFloatManager) IsMinusOne(a *FastFloat) (bool, error) {
	ok, e := m.IsNeg(a)
	if e != nil || !ok {
		return false, e
	}
	return m.IsAbsOne(a)
}
func (m *FastFloatManager) IsTwo(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return !a.negative() && a.exponent == 2-int32(m.bits) && m.minSig(a), nil
}
func (m *FastFloatManager) IsPlusEpsilon(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return !a.negative() && a.exponent == -2147483648 && m.minSig(a), nil
}
func (m *FastFloatManager) IsMinusEpsilon(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return a.negative() && a.exponent == -2147483648 && m.minSig(a), nil
}
func (m *FastFloatManager) IsInteger(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if a.exponent >= 0 {
		return true, nil
	}
	if a.exponent <= -int32(m.bits) {
		return false, nil
	}
	return !ffHasLow(m.sig(a), uint32(-a.exponent)), nil
}
func (m *FastFloatManager) IsInt64(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if a.index() == 0 {
		return true, nil
	}
	mx := -int32(32 * (m.precision - 2))
	if a.exponent < mx {
		return a.exponent > -int32(m.bits) && !ffHasLow(m.sig(a), uint32(-a.exponent)), nil
	}
	if a.exponent == mx {
		return a.negative() && m.minSig(a), nil
	}
	return false, nil
}
func (m *FastFloatManager) IsUint64(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if a.index() == 0 {
		return true, nil
	}
	return !a.negative() && a.exponent <= -int32(32*(m.precision-2)) && a.exponent > -int32(m.bits) && !ffHasLow(m.sig(a), uint32(-a.exponent)), nil
}
func (m *FastFloatManager) magnitude64(a *FastFloat) (uint64, error) {
	if a.index() == 0 {
		return 0, nil
	}
	// The source negates int32 exponent before computing its signed shift.
	// Diagnose actual undefined negation/shift, not finite fractional inputs.
	if a.exponent == -2147483648 {
		return 0, ErrFastFloatDomain
	}
	shift := -int64(a.exponent) - int64(32*(m.precision-2))
	if shift < 0 || shift >= 64 {
		return 0, ErrFastFloatDomain
	}
	s := m.sig(a)
	v := uint64(s.read(m.precision-1))<<32 | uint64(s.read(m.precision-2))
	return v >> uint32(shift), nil
}
func (m *FastFloatManager) Uint64(a *FastFloat) (uint64, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if SourceExtendedGCDDebug {
		ok, e := m.IsUint64(a)
		if e != nil {
			return 0, e
		}
		if !ok {
			return 0, ErrFastFloatInvariant
		}
	}
	return m.magnitude64(a)
}
func (m *FastFloatManager) Int64(a *FastFloat) (int64, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if SourceExtendedGCDDebug {
		ok, e := m.IsInt64(a)
		if e != nil {
			return 0, e
		}
		if !ok {
			return 0, ErrFastFloatInvariant
		}
	}
	v, e := m.magnitude64(a)
	if e != nil {
		return 0, e
	}
	// Source host signed conversion is adapted explicitly to two's-complement
	// int64. The source INT64_MIN special case has this same result.
	if a.negative() {
		return int64(0 - v), nil
	}
	return int64(v), nil
}
func (m *FastFloatManager) Equal(a, b *FastFloat) (bool, error) {
	if e := m.pair(a, b); e != nil {
		return false, e
	}
	if a.index() == 0 || b.index() == 0 {
		return a.index() == 0 && b.index() == 0, nil
	}
	return a.negative() == b.negative() && a.exponent == b.exponent && ffCompare(m.sig(a), m.sig(b)) == 0, nil
}
func (m *FastFloatManager) NotEqual(a, b *FastFloat) (bool, error) {
	ok, e := m.Equal(a, b)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (m *FastFloatManager) Less(a, b *FastFloat) (bool, error) {
	if e := m.pair(a, b); e != nil {
		return false, e
	}
	m.traceLessOperands(a, b)
	if a.index() == 0 {
		result := b.index() != 0 && !b.negative()
		m.traceLessResult(result, false)
		return result, nil
	}
	if b.index() == 0 {
		result := a.negative()
		m.traceLessResult(result, false)
		return result, nil
	}
	if a.negative() != b.negative() {
		result := a.negative()
		m.traceLessResult(result, false)
		return result, nil
	}
	if a.negative() {
		result := b.exponent < a.exponent || (a.exponent == b.exponent && ffCompare(m.sig(b), m.sig(a)) < 0)
		m.traceLessResult(result, true)
		return result, nil
	}
	result := a.exponent < b.exponent || (a.exponent == b.exponent && ffCompare(m.sig(a), m.sig(b)) < 0)
	m.traceLessResult(result, true)
	return result, nil
}
func (m *FastFloatManager) Greater(a, b *FastFloat) (bool, error) { return m.Less(b, a) }
func (m *FastFloatManager) LessEqual(a, b *FastFloat) (bool, error) {
	ok, e := m.Less(b, a)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (m *FastFloatManager) GreaterEqual(a, b *FastFloat) (bool, error) {
	ok, e := m.Less(a, b)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (m *FastFloatManager) IsPowerOfTwo(a *FastFloat) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return a.index() != 0 && !a.negative() && a.exponent > -int32(m.bits) && m.minSig(a), nil
}
func (m *FastFloatManager) PowerOfTwo(a *FastFloat) (bool, uint32, error) {
	ok, e := m.IsPowerOfTwo(a)
	if !ok || e != nil {
		return ok, 0, e
	}
	return true, uint32(int64(a.exponent) + int64(m.bits) - 1), nil
}
func (m *FastFloatManager) PrevPowerOfTwo(a *FastFloat) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if a.index() == 0 || a.negative() || a.exponent <= -int32(m.bits) {
		return 0, nil
	}
	return m.bits + uint32(a.exponent) - 1, nil
}

type FastFloatStats struct {
	HeaderBytes                                                           uint64
	PrecisionWords, Capacity, SignificandCapacity, IDRange, Live, FreeIDs uint32
}

func (m *FastFloatManager) Stats() (FastFloatStats, error) {
	if e := m.open(); e != nil {
		return FastFloatStats{}, e
	}
	return FastFloatStats{m.headerBytes, m.precision, m.capacity, uint32(cap(m.significands)), m.next, m.live, uint32(len(m.freeIDs))}, nil
}
