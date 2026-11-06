package sat

import "sort"

func (a *satAIGCuts) flushRoots() error {
	if len(a.roots) == 0 {
		return nil
	}
	mapping := make([]Literal, len(a.nodes))
	for i := range mapping {
		mapping[i] = MakeLiteral(BoolVar(i), false)
	}
	for i := len(a.roots) - 1; i >= 0; i-- {
		x := a.roots[i]
		r := mapping[x.root.variable()]
		if x.root.sign() {
			r = r.Negated()
		}
		mapping[x.variable] = r
	}
	for v := range a.nodes {
		if mapping[v] != MakeLiteral(BoolVar(v), false) {
			a.nodes[v] = nil
			if e := a.cuts[v].Reset(a.onCutDel); e != nil {
				return e
			}
			continue
		}
		kept := a.nodes[v][:0]
		for _, n := range a.nodes[v] {
			valid := true
			changed := false
			for j := uint32(0); j < n.size; j++ {
				p := n.offset + j
				lit := a.literals[p]
				r := mapping[lit.variable()]
				if lit.sign() {
					r = r.Negated()
				}
				if r != lit {
					a.literals[p] = r
					changed = true
				}
				if r.variable() == BoolVar(v) {
					valid = false
				}
			}
			if valid {
				if changed && (n.isAnd() || n.isXor()) {
					sort.Slice(a.literals[n.offset:n.offset+n.size], func(i, j int) bool { return a.literals[int(n.offset)+i] < a.literals[int(n.offset)+j] })
				}
				kept = append(kept, n)
			}
		}
		a.nodes[v] = kept
	}
	for _, cs := range a.cuts {
		for i := 0; i < len(cs.cuts); {
			bad := false
			for j := uint32(0); j < cs.cuts[i].size; j++ {
				v := cs.cuts[i].elems[j]
				if mapping[v] != MakeLiteral(BoolVar(v), false) {
					bad = true
					break
				}
			}
			if bad {
				if e := cs.EvictIndex(a.onCutDel, uint32(i)); e != nil {
					return e
				}
			} else {
				i++
			}
		}
	}
	a.roots = nil
	return nil
}
