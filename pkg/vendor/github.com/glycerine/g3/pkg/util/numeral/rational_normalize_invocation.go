package numeral

// RationalNormalizeInvocationReceipt identifies the actual SYNCH RAW mpz
// declared by normalize. Unsynchronized normalization has no such local.
type RationalNormalizeInvocationReceipt struct {
	manager     *RationalManager
	generation  uint64
	executing   bool
	sourceCause error
}

func (m *RationalManager) MatchesRawNormalizeInvocation(r *RationalNormalizeInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.generation != 0 && m.RawArithmetic.normGeneration == r.generation && m.RawArithmetic.NormActive
}
func (m *RationalManager) RawNormalizeInvocationSourceCause(r *RationalNormalizeInvocationReceipt) error {
	if r != nil && r.manager == m && r.generation != 0 {
		return r.sourceCause
	}
	return nil
}
func (m *RationalManager) CleanupRawNormalizeInvocation(r *RationalNormalizeInvocationReceipt) error {
	if r != nil && r.manager == m && r.executing {
		return ErrNumericPending
	}
	if !m.MatchesRawNormalizeInvocation(r) {
		return nil
	}
	return m.CleanupRawGroup(rationalRawNormalize)
}
