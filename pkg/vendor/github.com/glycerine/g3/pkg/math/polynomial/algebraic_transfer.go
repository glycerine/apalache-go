package polynomial

// Source: algebraic_numbers.cpp:213-215,358-360,2803-2805,2839-2841.

// Reset follows source reset, including Del's destruction order and failures.
func (m *AlgebraicRootCore) Reset(a *Algebraic) error { return m.Del(a) }

// Swap transfers the two existing cells. It does not copy or normalize their
// numerals, refine intervals, or change the ownership of the cells themselves.
func (m *AlgebraicRootCore) Swap(a, b *Algebraic) error {
	if _, e := m.location(a); e != nil {
		return e
	}
	if _, e := m.location(b); e != nil {
		return e
	}
	if m.negationProtectsCaller(a) || m.negationProtectsCaller(b) {
		return ErrAlgebraicPending
	}
	*a, *b = *b, *a
	return nil
}
