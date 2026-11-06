package interval

// Source: src/math/interval/mod_interval.h:76-117, uint64 instantiation.

func intersectModularSmall(a, b, r *ModularInterval) bool {
	equal, _ := a.Equal(b)
	if a.IsFull() || equal {
		copyModularInterval(r, b)
		return true
	}
	if b.IsFull() {
		copyModularInterval(r, a)
		return true
	}
	if a.IsWrapped() {
		if b.IsWrapped() {
			if a.upperSmall >= b.lowerSmall {
				copyModularInterval(r, b)
			} else if b.upperSmall >= a.lowerSmall {
				copyModularInterval(r, a)
			} else {
				r.setSmall(maxU64(a.lowerSmall, b.lowerSmall), minU64(a.upperSmall, b.upperSmall), false)
			}
			return true
		}
		return intersectModularSmall(b, a, r)
	}
	if b.IsWrapped() {
		if a.upperSmall < b.lowerSmall && a.lowerSmall > b.upperSmall {
			return false
		}
		if a.upperSmall >= b.lowerSmall && a.lowerSmall <= b.upperSmall {
			copyModularInterval(r, b)
		} else if a.upperSmall >= b.lowerSmall {
			r.setSmall(b.lowerSmall, a.upperSmall, false)
		} else {
			r.setSmall(a.lowerSmall, minU64(a.upperSmall, b.upperSmall), false)
		}
		return true
	}
	if a.lowerSmall > b.upperSmall || a.upperSmall < b.lowerSmall {
		return false
	}
	r.setSmall(maxU64(a.lowerSmall, b.lowerSmall), minU64(a.upperSmall, b.upperSmall), a.tight && b.tight)
	return true
}
func minU64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
func maxU64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
