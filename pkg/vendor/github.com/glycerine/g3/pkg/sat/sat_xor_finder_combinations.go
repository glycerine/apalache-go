package sat

func (x *satXORFinder) set(mask uint32) { x.combination |= 1 << mask }
func (x *satXORFinder) covered(n uint32, parity bool) bool {
	for a := uint32(0); a < uint32(1)<<n; a++ {
		if x.parity[n][a] == parity && x.combination&(1<<a) == 0 {
			return false
		}
	}
	return true
}
func (x *satXORFinder) expand(mask uint32, n uint32, parity bool) bool {
	for k := uint32(0); k < uint32(1)<<uint32(len(x.missing)); k++ {
		m := mask
		for i, p := range x.missing {
			if k&(1<<uint32(i)) != 0 {
				m |= 1 << p
			}
		}
		x.set(m)
	}
	return x.covered(n, parity)
}
