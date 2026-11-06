package sat

func (f *satAIGFinder) implies(a, b Literal) bool {
	return f.big.connected(a, b) || f.binaries[satAIGPair(a.Negated(), b)]
}
