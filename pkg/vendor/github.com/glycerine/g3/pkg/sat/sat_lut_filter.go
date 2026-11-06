package sat

func satLUTDistinct(c []Literal) bool {
	seen := make(map[BoolVar]bool, len(c))
	for _, l := range c {
		v := l.variable()
		if seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}
func satLUTFilter(c []Literal) uint32 {
	var f uint32
	for _, l := range c {
		f |= uint32(1) << (uint32(l.variable()) & 31)
	}
	return f
}
func satLUTSubset(c []Literal, pos map[BoolVar]uint32) bool {
	for _, l := range c {
		if _, ok := pos[l.variable()]; !ok {
			return false
		}
	}
	return true
}
