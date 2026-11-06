package sat

func (x *satXORFinder) Run(clauses []satLUTClause, binaries []satLUTBinary) ([]satLUTClause, error) {
	used := make([]bool, len(clauses))
	removed := make([]bool, len(clauses))
	for size := x.maxSize; size > 2; size-- {
		for ci, c := range clauses {
			if used[ci] || removed[ci] || c.Learned || uint32(len(c.Literals)) != size || !satLUTDistinct(c.Literals) {
				continue
			}
			pos := satXORPositions(c.Literals)
			parity := satXORClauseParity(c.Literals)
			x.combination = 0
			var initial uint32
			for i, l := range c.Literals {
				if !l.sign() {
					initial |= 1 << uint32(i)
				}
			}
			x.set(initial)
			members := []int{ci}
			used[ci] = true
			found := false
			for j, d := range clauses {
				if j == ci || used[j] || removed[j] || uint32(len(d.Literals)) > size {
					continue
				}
				if x.extract(d.Literals, pos, size, parity) {
					if len(d.Literals) == len(c.Literals) {
						members = append(members, j)
						used[j] = true
					}
					found = true
					break
				}
				if len(d.Literals) == len(c.Literals) && satLUTSubset(d.Literals, pos) && satXORClauseParity(d.Literals) == parity {
					members = append(members, j)
					used[j] = true
				}
			}
			if !found {
				for _, b := range binaries {
					if x.extractBinary(b, pos, size, parity) {
						found = true
						break
					}
				}
			}
			if found {
				if x.onXOR != nil {
					if e := x.onXOR(satXORPublication(c.Literals, parity)); e != nil {
						return nil, e
					}
				}
				for _, i := range members {
					removed[i] = true
				}
			}
		}
	}
	out := make([]satLUTClause, 0, len(clauses))
	for i, c := range clauses {
		if !removed[i] {
			out = append(out, c)
		}
	}
	return out, nil
}
