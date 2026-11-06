package sat

type satAIGTernaryKey [3]Literal
type satAIGUse struct {
	third  Literal
	clause int
}

func satAIGTriple(a, b, c Literal) satAIGTernaryKey {
	if a > b {
		a, b = b, a
	}
	if b > c {
		b, c = c, b
	}
	if a > b {
		a, b = b, a
	}
	return satAIGTernaryKey{a, b, c}
}
func satAIGTernaryIndex(clauses []satLUTClause) (map[satAIGTernaryKey]int, map[[2]Literal][]satAIGUse) {
	ts := make(map[satAIGTernaryKey]int)
	uses := make(map[[2]Literal][]satAIGUse)
	for i, c := range clauses {
		if len(c.Literals) != 3 {
			continue
		}
		a, b, d := c.Literals[0], c.Literals[1], c.Literals[2]
		ts[satAIGTriple(a, b, d)] = i
		uses[satAIGPair(a, b)] = append(uses[satAIGPair(a, b)], satAIGUse{d, i})
		uses[satAIGPair(a, d)] = append(uses[satAIGPair(a, d)], satAIGUse{b, i})
		uses[satAIGPair(d, b)] = append(uses[satAIGPair(d, b)], satAIGUse{a, i})
	}
	return ts, uses
}
