package sat

func satXORClauseParity(c []Literal) bool {
	p := false
	for _, l := range c {
		p = p != (!l.sign())
	}
	return p
}
func satXORPositions(c []Literal) map[BoolVar]uint32 {
	p := make(map[BoolVar]uint32, len(c))
	for i, l := range c {
		p[l.variable()] = uint32(i)
	}
	return p
}
