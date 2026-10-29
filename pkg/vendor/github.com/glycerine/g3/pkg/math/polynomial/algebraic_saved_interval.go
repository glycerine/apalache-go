package polynomial

// Source: algebraic_numbers.cpp:897-929; binary/unary selection and signing.

func (m *AlgebraicRootCore) beginSavedInterval(saved *algebraicSavedInterval, target *Algebraic) error {
	if e := m.check(); e != nil {
		return e
	}
	if saved.owned {
		return ErrAlgebraicPending
	}
	*saved = algebraicSavedInterval{target: target, owned: true}
	if e := m.assertBinaryRoot(target, 907, "!num.is_basic()"); e != nil {
		return e
	}
	if e := algebraicQueryTagAssertionSwitch(); e != nil {
		return e
	}
	l, e := m.location(target)
	if e != nil {
		return e
	}
	if l == nil || l.kind != 1 {
		return ErrAlgebraicOwner
	}
	r, e := m.load(l)
	if e != nil {
		return e
	}
	m.savedIntervalExecuting = saved
	defer func() { m.savedIntervalExecuting = nil }()
	if e = m.config.Intervals.CopyWithAllocationReceipt(&saved.value, &r.interval, &saved.copy); e != nil {
		return e
	}
	saved.constructed = true
	return nil
}

func (m *AlgebraicRootCore) finishSavedInterval(saved *algebraicSavedInterval, owning bool) error {
	if m.savedIntervalExecuting != nil {
		return ErrAlgebraicPending
	}
	if !saved.owned {
		return nil
	}
	// A failed constructor never reaches ~save_intervals or its manager DEL.
	// Its partial member storage and unjoined SET belong to explicit abort.
	if !saved.constructed && !owning {
		return nil
	}
	m.savedIntervalExecuting = saved
	defer func() { m.savedIntervalExecuting = nil }()
	if owning {
		if e := m.config.Rationals.Integers().CleanupCopyAllocationInvocation(&saved.copy); e != nil {
			return e
		}
	}
	if saved.constructed && !saved.restored {
		if e := m.restoreBinaryInterval(saved); e != nil {
			return e
		}
	}
	if e := m.config.Intervals.Del(&saved.value); e != nil {
		return e
	}
	saved.constructed = false
	saved.owned = false
	saved.target = nil
	return nil
}
