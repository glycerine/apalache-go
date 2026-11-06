package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1321-1343, concrete config_mpfx.

func (n *FixedNode) IsUnbounded(variable uint32) (bool, error) {
	lower, err := n.bound(variable, true)
	if err != nil {
		return false, err
	}
	upper, err := n.bound(variable, false)
	if err != nil {
		return false, err
	}
	return lower == nil && upper == nil, nil
}

func (n *FixedNode) IsZero(variable uint32) (bool, error) {
	lower, err := n.bound(variable, true)
	if err != nil {
		return false, err
	}
	upper, err := n.bound(variable, false)
	if err != nil {
		return false, err
	}
	if lower == nil || upper == nil {
		return false, nil
	}
	lz, err := n.numbers.IsZero(lower.value)
	if err != nil {
		return false, err
	}
	if !lz {
		return false, nil
	}
	uz, err := n.numbers.IsZero(upper.value)
	return uz && !lower.open && !upper.open, err
}

func (n *FixedNode) IsUpperZero(variable uint32) (bool, error) {
	upper, err := n.bound(variable, false)
	if err != nil {
		return false, err
	}
	if upper == nil {
		return false, nil
	}
	z, err := n.numbers.IsZero(upper.value)
	return z && !upper.open, err
}

func (n *FixedNode) ConflictingBounds(variable uint32) (bool, error) {
	lower, err := n.bound(variable, true)
	if err != nil {
		return false, err
	}
	upper, err := n.bound(variable, false)
	if err != nil {
		return false, err
	}
	if lower == nil || upper == nil {
		return false, nil
	}
	less, err := n.numbers.Less(upper.value, lower.value)
	if err != nil || less {
		return less, err
	}
	if !lower.open && !upper.open {
		return false, nil
	}
	return n.numbers.Equal(upper.value, lower.value)
}
