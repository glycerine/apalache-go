package numeral

// RationalPowerInvocationReceipt is stamped only at an accepted RAW power
// entry. It owns no operand and cannot authorize another generation's cleanup.
type RationalPowerInvocationReceipt struct {
	manager     *RationalManager
	generation  uint64
	executing   bool
	sourceCause error
}

func (m *RationalManager) MatchesRawPowerInvocation(r *RationalPowerInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.generation != 0 && m.RawArithmetic.PowerGeneration == r.generation && m.RawArithmetic.PowerActive
}
func (m *RationalManager) RawPowerInvocationSourceCause(r *RationalPowerInvocationReceipt) error {
	if r != nil && r.manager == m && r.generation != 0 {
		return r.sourceCause
	}
	return nil
}
func (m *RationalManager) CleanupRawPowerInvocation(r *RationalPowerInvocationReceipt) error {
	if r != nil && r.manager == m && r.executing {
		return ErrNumericPending
	}
	if !m.MatchesRawPowerInvocation(r) {
		return nil
	}
	return m.CleanupRawGroup(RationalRawPower)
}
