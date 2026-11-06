package interval

// Source: src/math/interval/mod_interval.h:54-57,195-201.

func (v *ModularInterval) Equal(other *ModularInterval) (bool, error) {
	if v == nil || other == nil || v.size != other.size || v.small != other.small {
		return false, ErrExtendedDomain
	}
	if v.small {
		return v.lowerSmall == other.lowerSmall && v.upperSmall == other.upperSmall && v.tight == other.tight, nil
	}
	return v.lowerLarge.Cmp(other.lowerLarge) == 0 && v.upperLarge.Cmp(other.upperLarge) == 0 && v.tight == other.tight, nil
}
