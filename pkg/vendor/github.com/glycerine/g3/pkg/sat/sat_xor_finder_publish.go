package sat

func satXORPublication(c []Literal, parity bool) []Literal {
	out := make([]Literal, len(c))
	for i, l := range c {
		out[i] = MakeLiteral(l.variable(), false)
	}
	if parity == (len(out)%2 == 0) {
		out[0] = out[0].Negated()
	}
	return out
}
