package sat

import "sort"

func (l *satLUTFinder) Run(clauses []satLUTClause, binaries []satLUTBinary) ([]satLUTClause, error) {
	used := make([]bool, len(clauses))
	removed := make([]bool, len(clauses))
	for size := l.maxSize; size > 2; size-- {
		for ci, c := range clauses {
			if used[ci] || removed[ci] || c.Learned || uint32(len(c.Literals)) != size || !satLUTDistinct(c.Literals) {
				continue
			}
			base := append([]Literal(nil), c.Literals...)
			sort.Slice(base, func(i, j int) bool { return base[i] < base[j] })
			l.vars = l.vars[:0]
			pos := make(map[BoolVar]uint32, size)
			var initial uint32
			for i, x := range base {
				l.vars = append(l.vars, x.variable())
				pos[x.variable()] = uint32(i)
				if x.sign() {
					initial |= 1 << uint32(i)
				}
			}
			l.combination, l.numCombinations = 0, 0
			l.setCombination(initial)
			members := []int{ci}
			used[ci] = true
			found := false
			for j, d := range clauses {
				if j == ci || used[j] || removed[j] || uint32(len(d.Literals)) > size || !satLUTDistinct(d.Literals) {
					continue
				}
				if satLUTFilter(d.Literals)&^satLUTFilter(base) != 0 {
					continue
				}
				if l.extract(d.Literals, pos) {
					if len(d.Literals) == len(base) {
						members = append(members, j)
						used[j] = true
					}
					found = true
					break
				}
				if len(d.Literals) == len(base) {
					members = append(members, j)
					used[j] = true
				}
			}
			if !found {
				for _, b := range binaries {
					pair := []Literal{b.First, b.Second}
					if l.extract(pair, pos) {
						found = true
						break
					}
				}
			}
			if found {
				table, inputs, out := l.convert()
				if l.onLUT != nil {
					l.onLUT(table, inputs, out)
				}
				for _, j := range members {
					removed[j] = true
				}
			}
		}
	}
	result := make([]satLUTClause, 0, len(clauses))
	for i, c := range clauses {
		if !removed[i] {
			result = append(result, c)
		} else {
			used[i] = true
		}
	}
	return result, nil
}
