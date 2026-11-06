package sat

import "github.com/glycerine/g3/pkg/arena"

func (a *satAIGCuts) node2def(f satAIGClauseUpdate, n satAIGNode, r Literal) error {
	if n.sign {
		r = r.Negated()
	}
	switch n.op {
	case satAIGVar:
		return nil
	case satAIGAnd:
		for i := uint32(0); i < n.size; i++ {
			emitSATClause(f, r.Negated(), a.child(n, i))
		}
		cl := make([]Literal, 0, n.size+1)
		for i := uint32(0); i < n.size; i++ {
			cl = append(cl, a.child(n, i).Negated())
		}
		cl = append(cl, r)
		f(cl)
		return nil
	case satAIGITE:
		if n.size != 3 {
			return arena.ErrBounds
		}
		c, t, e := a.child(n, 0), a.child(n, 1), a.child(n, 2)
		emitSATClause(f, r.Negated(), c.Negated(), t)
		emitSATClause(f, r.Negated(), c, e)
		emitSATClause(f, r, c.Negated(), t.Negated())
		emitSATClause(f, r, c, e.Negated())
		return nil
	case satAIGXor:
		if n.size > 10 {
			return arena.ErrBounds
		}
		for bits := uint32(0); bits < uint32(1)<<n.size; bits++ {
			parity := n.size%2 == 1
			cl := make([]Literal, 0, n.size+1)
			for j := uint32(0); j < n.size; j++ {
				lit := a.child(n, j)
				if bits&(1<<j) == 0 {
					lit = lit.Negated()
				} else {
					parity = !parity
				}
				cl = append(cl, lit)
			}
			if parity {
				cl = append(cl, r)
			} else {
				cl = append(cl, r.Negated())
			}
			f(cl)
		}
		return nil
	case satAIGLUT:
		for bits := uint32(0); bits < uint32(1)<<n.size; bits++ {
			cl := make([]Literal, 0, n.size+1)
			for j := uint32(0); j < n.size; j++ {
				lit := a.child(n, j)
				if bits&(1<<j) != 0 {
					lit = lit.Negated()
				}
				cl = append(cl, lit)
			}
			rr := r
			if n.lut&(uint64(1)<<bits) == 0 {
				rr = rr.Negated()
			}
			cl = append(cl, rr)
			f(cl)
		}
		return nil
	default:
		return ErrSATKind
	}
}

func (a *satAIGCuts) Cut2Def(f satAIGClauseUpdate, c satCut, r Literal) error {
	if err := a.open(); err != nil {
		return err
	}
	if f == nil {
		return arena.ErrConfig
	}
	a.cut2def(f, c, r)
	return nil
}
