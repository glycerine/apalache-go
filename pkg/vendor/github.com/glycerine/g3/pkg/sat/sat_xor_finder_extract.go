package sat

func (x *satXORFinder) extract(c []Literal, pos map[BoolVar]uint32, n uint32, parity bool) bool {
	if !satLUTDistinct(c) || !satLUTSubset(c, pos) {
		return false
	}
	if uint32(len(c)) == n && satXORClauseParity(c) != parity {
		return false
	}
	present := make([]bool, n)
	var mask uint32
	x.missing = x.missing[:0]
	for _, l := range c {
		p := pos[l.variable()]
		present[p] = true
		if !l.sign() {
			mask |= 1 << p
		}
	}
	for i, yes := range present {
		if !yes {
			x.missing = append(x.missing, uint32(i))
		}
	}
	return x.expand(mask, n, parity)
}
func (x *satXORFinder) extractBinary(b satLUTBinary, pos map[BoolVar]uint32, n uint32, parity bool) bool {
	return x.extract([]Literal{b.First, b.Second}, pos, n, parity)
}
