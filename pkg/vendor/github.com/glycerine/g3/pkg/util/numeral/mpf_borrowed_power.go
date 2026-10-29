package numeral

// Source: src/util/mpf.h m_powers2, consumed by theory_fpa.cpp:158,161.

// BorrowPowerOfTwo returns a read-only NUM view owned by this manager's real
// power cache. The caller must not delete or mutate it, or retain it after the
// manager ends. It is the concrete mpz const& cache access, not a copied power.
func (m *ArbitraryFloatManager) BorrowPowerOfTwo(n uint32) (Int, error) {
	if err := m.valueOwnerGuard(); err != nil {
		return Int{}, err
	}
	wrapper, err := m.powerTwoWrapper(n, false)
	if err != nil {
		return Int{}, err
	}
	return wrapper.Capture()
}
