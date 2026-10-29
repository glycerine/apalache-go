package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:217-240,350-356,2807-2828.

// The pointer-tag dispatch establishes each reached accessor assertion. Its
// predicate cannot fail here, but the source still evaluates the runtime switch.
func algebraicQueryTagAssertionSwitch() error {
	_, e := predicateAssertionsEnabled()
	return e
}

func (m *AlgebraicRootCore) IsPos(a *Algebraic) (bool, error) {
	return m.querySign(a, false)
}
func (m *AlgebraicRootCore) IsNeg(a *Algebraic) (bool, error) {
	return m.querySign(a, true)
}

// Source qm().is_pos/is_neg reaches only mpz.m_val. The open binary interval
// specialization reaches lower.m_num.m_val >= 0 or upper.m_num.m_val <= 0
// (basic_interval.h:177,182; mpbq.h:98-99), with no exponent or opposite bound.
func (m *AlgebraicRootCore) querySign(a *Algebraic, negative bool) (bool, error) {
	l, e := m.location(a)
	if e != nil {
		return false, e
	}
	if l == nil || l.kind == 0 {
		// basic_value assertion at cpp236, including the canonical null zero.
		if e := algebraicQueryTagAssertionSwitch(); e != nil {
			return false, e
		}
		if l == nil {
			return false, nil
		}
		// The nonnull branch also reaches anum::to_basic at h372.
		if e := algebraicQueryTagAssertionSwitch(); e != nil {
			return false, e
		}
		word, e := m.config.Allocator.ReadUint32(l.raw, 0)
		if e != nil {
			return false, e
		}
		if negative {
			return int32(word) < 0, nil
		}
		return int32(word) > 0, nil
	}
	// anum::to_algebraic at h373, then the actually borrowed interval owner.
	if e := algebraicQueryTagAssertionSwitch(); e != nil {
		return false, e
	}
	if _, e := m.config.Intervals.Bounds(); e != nil {
		return false, e
	}
	offset := uint64(24)
	if negative {
		offset = 40
	}
	word, e := m.config.Allocator.ReadUint32(l.raw, offset)
	if e != nil {
		return false, e
	}
	if negative {
		return int32(word) <= 0, nil
	}
	return int32(word) >= 0, nil
}

// Degree preserves source zero/basic/root dispatch; it does not minimize a
// polynomial or refine an interval.
func (m *AlgebraicRootCore) Degree(a *Algebraic) (uint32, error) {
	l, e := m.location(a)
	if e != nil || l == nil {
		return 0, e
	}
	if l.kind == 0 {
		return 1, nil
	}
	// This dispatch already establishes !is_basic(), but the SASSERT in
	// anum::to_algebraic (algebraic_numbers.h:373) still reads its switch.
	if e := algebraicQueryTagAssertionSwitch(); e != nil {
		return 0, e
	}
	// Only m_p_sz is reached. Polynomial handles, flags and interval fields
	// are independent source members, not prerequisites for this scalar read.
	size, e := m.config.Allocator.ReadUint32(l.raw, 20)
	if e != nil {
		return 0, e
	}
	return size - 1, nil
}
