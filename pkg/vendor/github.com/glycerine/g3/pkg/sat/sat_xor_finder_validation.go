package sat

func satXORLiteralsValue(xs []Literal, assignment uint32) bool {
	p := false
	for _, l := range xs {
		v := assignment&(1<<uint32(l.variable())) != 0
		if l.sign() {
			v = !v
		}
		p = p != v
	}
	return p
}
