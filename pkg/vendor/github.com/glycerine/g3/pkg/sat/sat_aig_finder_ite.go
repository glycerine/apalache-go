package sat

func (f *satAIGFinder) hasTernary(a, b, c Literal, ts map[satAIGTernaryKey]int) (int, bool) {
	if i, ok := ts[satAIGTriple(a, b, c)]; ok {
		return i, true
	}
	if f.implies(b.Negated(), c) || f.implies(a.Negated(), b) || f.implies(a.Negated(), c) {
		return -1, true
	}
	return -1, false
}
func (f *satAIGFinder) tryITE(x, y, z Literal, ci int, ts map[satAIGTernaryKey]int, uses map[[2]Literal][]satAIGUse, used []bool) (bool, error) {
	i1, ok := f.hasTernary(y, z.Negated(), x.Negated(), ts)
	if !ok {
		return false, nil
	}
	for _, p := range uses[satAIGPair(y.Negated(), x)] {
		u := p.third
		i3, yes := f.hasTernary(u.Negated(), x.Negated(), y.Negated(), ts)
		if !yes {
			continue
		}
		used[ci] = true
		if i1 >= 0 {
			used[i1] = true
		}
		used[p.clause] = true
		if i3 >= 0 {
			used[i3] = true
		}
		if err := f.onITE(x.Negated(), y.Negated(), z, u); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
func (f *satAIGFinder) findITEs(clauses []satLUTClause, used []bool) error {
	if f.onITE == nil {
		return nil
	}
	ts, uses := satAIGTernaryIndex(clauses)
	for i, c := range clauses {
		if len(c.Literals) != 3 || used[i] {
			continue
		}
		a, b, d := c.Literals[0], c.Literals[1], c.Literals[2]
		orders := [6][3]Literal{{a, d, b}, {a, b, d}, {b, a, d}, {d, a, b}, {d, b, a}, {b, d, a}}
		for _, o := range orders {
			ok, e := f.tryITE(o[0], o[1], o[2], i, ts, uses, used)
			if e != nil {
				return e
			}
			if ok {
				break
			}
		}
	}
	return nil
}
