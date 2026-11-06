package interval

// Source: src/math/interval/mod_interval.h:76-117, rational instantiation.

import "math/big"

func intersectModularLarge(a, b, r *ModularInterval) bool {
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
			if a.upperLarge.Cmp(b.lowerLarge) >= 0 {
				copyModularInterval(r, b)
			} else if b.upperLarge.Cmp(a.lowerLarge) >= 0 {
				copyModularInterval(r, a)
			} else {
				r.setLarge(maxBig(a.lowerLarge, b.lowerLarge), minBig(a.upperLarge, b.upperLarge), false)
			}
			return true
		}
		return intersectModularLarge(b, a, r)
	}
	if b.IsWrapped() {
		if a.upperLarge.Cmp(b.lowerLarge) < 0 && a.lowerLarge.Cmp(b.upperLarge) > 0 {
			return false
		}
		if a.upperLarge.Cmp(b.lowerLarge) >= 0 && a.lowerLarge.Cmp(b.upperLarge) <= 0 {
			copyModularInterval(r, b)
		} else if a.upperLarge.Cmp(b.lowerLarge) >= 0 {
			r.setLarge(b.lowerLarge, a.upperLarge, false)
		} else {
			r.setLarge(a.lowerLarge, minBig(a.upperLarge, b.upperLarge), false)
		}
		return true
	}
	if a.lowerLarge.Cmp(b.upperLarge) > 0 || a.upperLarge.Cmp(b.lowerLarge) < 0 {
		return false
	}
	r.setLarge(maxBig(a.lowerLarge, b.lowerLarge), minBig(a.upperLarge, b.upperLarge), a.tight && b.tight)
	return true
}
func minBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return a
	}
	return b
}
func maxBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) > 0 {
		return a
	}
	return b
}
