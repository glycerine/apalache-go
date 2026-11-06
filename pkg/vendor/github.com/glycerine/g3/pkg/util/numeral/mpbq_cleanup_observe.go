package numeral

// Source saved-interval destructors read magnitude and swap old bounds after
// completed debug locals, even when an unjoined RAW allocation remains.

func (m *BinaryRationalManager) IsNonNegForCleanup(a *BinaryRational) (bool, error) {
	if e := m.checkCleanup(a); e != nil {
		return false, e
	}
	return m.Z.IsNonNeg(&a.Num)
}
func (m *BinaryRationalManager) IsNonPosForCleanup(a *BinaryRational) (bool, error) {
	if e := m.checkCleanup(a); e != nil {
		return false, e
	}
	return m.Z.IsNonPos(&a.Num)
}
func (m *BinaryRationalManager) MagnitudeUpperForCleanup(a *BinaryRational) (int32, error) {
	if e := m.checkCleanup(a); e != nil {
		return 0, e
	}
	return m.magnitudeUpper(a)
}
func (m *BinaryIntervalManager) SwapForCleanup(a, b *BinaryInterval) error {
	if e := m.checkCleanup(a); e != nil {
		return e
	}
	if e := m.checkCleanup(b); e != nil {
		return e
	}
	a.lower, b.lower = b.lower, a.lower
	a.upper, b.upper = b.upper, a.upper
	return nil
}
