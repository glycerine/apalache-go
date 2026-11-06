package numeral

// Source: src/util/hwf.cpp:577-584 and the live f2n<hwf_manager>
// previous-power lookup in src/util/f2n.h:145.

// PrevPowerOfTwo preserves the hardware manager's source formula. The result
// is the unsigned exponent used by the interval Newton root's rough estimate.
func (m *HardwareFloatManager) PrevPowerOfTwo(a *HardwareFloat) uint32 {
	if !m.IsPos(a) {
		return 0
	}
	exp := m.Exponent(a)
	if exp <= -52 {
		return 0
	}
	return uint32(51 + exp)
}

func (m *HardwareFloatNumeralManager) PrevPowerOfTwo(a *HardwareFloat) (uint32, error) {
	if err := m.check(a); err != nil {
		return 0, err
	}
	return m.core.PrevPowerOfTwo(a), nil
}
