package sat

import "github.com/glycerine/g3/pkg/arena"

func (a *satAIGCuts) evalNode(n satAIGNode, env []satCutVal) (satCutVal, error) {
	var r uint64
	switch n.op {
	case satAIGAnd:
		r = ^uint64(0)
		for i := uint32(0); i < n.size; i++ {
			l := a.child(n, i)
			x := env[l.variable()].t
			if l.sign() {
				x = env[l.variable()].f
			}
			r &= x
		}
	case satAIGXor:
		for i := uint32(0); i < n.size; i++ {
			l := a.child(n, i)
			x := env[l.variable()].t
			if l.sign() {
				x = env[l.variable()].f
			}
			r ^= x
		}
	case satAIGITE:
		if n.size != 3 {
			return satCutVal{}, arena.ErrBounds
		}
		c, t, e := a.child(n, 0), a.child(n, 1), a.child(n, 2)
		cv, tv, ev := env[c.variable()].t, env[t.variable()].t, env[e.variable()].t
		if c.sign() {
			cv = env[c.variable()].f
		}
		if t.sign() {
			tv = env[t.variable()].f
		}
		if e.sign() {
			ev = env[e.variable()].f
		}
		r = cv&tv | ^cv&ev
	case satAIGLUT:
		for lane := uint32(0); lane < 64; lane++ {
			var w uint32
			for i := uint32(0); i < n.size; i++ {
				l := a.child(n, i)
				x := env[l.variable()].t
				if l.sign() {
					x = env[l.variable()].f
				}
				w |= uint32(x>>lane&1) << i
			}
			r |= (n.lut >> w) & 1 << lane
		}
	default:
		return satCutVal{}, ErrSATKind
	}
	if n.sign {
		r = ^r
	}
	return satCutVal{r, ^r}, nil
}
func (a *satAIGCuts) Simulate(rounds uint32) ([]satCutVal, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	out := make([]satCutVal, len(a.cuts))
	for i := range out {
		r := uint64(a.random.Next()) | uint64(a.random.Next())<<16 | uint64(a.random.Next())<<32 | uint64(a.random.Next())<<48
		out[i] = satCutVal{r, ^r}
	}
	for k := uint32(0); k < rounds; k++ {
		for v, cs := range a.cuts {
			if cs.Size() <= 1 && len(a.nodes[v]) > 0 && !a.nodes[v][0].isVar() {
				x, e := a.evalNode(a.nodes[v][0], out)
				if e != nil {
					return nil, e
				}
				out[v] = x
			} else if cs.Size() > 1 {
				out[v] = cs.cuts[1+a.random.Next()%(cs.Size()-1)].Eval(out)
			}
		}
	}
	return out, nil
}
