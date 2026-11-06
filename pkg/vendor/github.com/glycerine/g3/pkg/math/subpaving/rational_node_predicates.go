package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1321-1343, concrete config_mpq.

func (n *RationalNode) IsZero(variable uint32) (bool, error) {
	lo, hi, err := n.Bounds(variable)
	if err != nil || lo == nil || hi == nil {
		return false, err
	}
	lz, err := n.numbers.IsZero(&lo.value)
	if err != nil || !lz {
		return false, err
	}
	hz, err := n.numbers.IsZero(&hi.value)
	return hz && !lo.open && !hi.open, err
}

func (n *RationalNode) IsUpperZero(variable uint32) (bool, error) {
	_, hi, err := n.Bounds(variable)
	if err != nil || hi == nil {
		return false, err
	}
	z, err := n.numbers.IsZero(&hi.value)
	return z && !hi.open, err
}
