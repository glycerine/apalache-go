package interval

// Source: src/math/interval/mod_interval.h:59-74,218-221.

func (v *ModularInterval) Implies(other *ModularInterval) (bool, error) {
	if v == nil || other == nil || v.size != other.size || v.small != other.small {
		return false, ErrExtendedDomain
	}
	if other.IsFull() {
		return true, nil
	}
	if v.IsFull() {
		return false, nil
	}
	if v.small {
		return impliesSmall(v, other), nil
	}
	return impliesLarge(v, other), nil
}

func impliesSmall(a, b *ModularInterval) bool {
	if a.IsWrapped() {
		return b.IsWrapped() && a.upperSmall <= b.upperSmall && a.lowerSmall >= b.lowerSmall
	}
	if b.IsWrapped() {
		return a.upperSmall <= b.upperSmall || a.lowerSmall >= b.lowerSmall
	}
	return a.lowerSmall >= b.lowerSmall && a.upperSmall <= b.upperSmall
}
func impliesLarge(a, b *ModularInterval) bool {
	if a.IsWrapped() {
		return b.IsWrapped() && a.upperLarge.Cmp(b.upperLarge) <= 0 && a.lowerLarge.Cmp(b.lowerLarge) >= 0
	}
	if b.IsWrapped() {
		return a.upperLarge.Cmp(b.upperLarge) <= 0 || a.lowerLarge.Cmp(b.lowerLarge) >= 0
	}
	return a.lowerLarge.Cmp(b.lowerLarge) >= 0 && a.upperLarge.Cmp(b.upperLarge) <= 0
}
