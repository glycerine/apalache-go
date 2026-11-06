package interval

// Source: src/math/interval/mod_interval.h:76-117,187-193.

func (v *ModularInterval) Intersect(other, result *ModularInterval) (bool, error) {
	if v == nil || other == nil || result == nil || v.size != other.size || v.small != other.small {
		return false, ErrExtendedDomain
	}
	result.size = v.size
	if v.small {
		return intersectModularSmall(v, other, result), nil
	}
	return intersectModularLarge(v, other, result), nil
}

func copyModularInterval(dst, src *ModularInterval) {
	dst.size = src.size
	if src.small {
		dst.setSmall(src.lowerSmall, src.upperSmall, src.tight)
	} else {
		dst.setLarge(src.lowerLarge, src.upperLarge, src.tight)
	}
}
