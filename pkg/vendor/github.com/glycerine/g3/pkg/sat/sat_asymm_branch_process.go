package sat

// Source: src/sat/sat_asymm_branch.cpp:123–204; src/sat/sat_solver.h:199–218.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"

	"github.com/glycerine/g3/pkg/arena"
)

func (a *satAsymmBranch) process(c StoredClauseRef) (keep bool, err error) {
	if a == nil || a.self != a {
		return false, ErrSATOwner
	}
	if a.closed || a.closing || (!a.running && (a.interrupted || a.sorting)) {
		return false, arena.ErrClosed
	}
	l := a.lifecycle
	p := l.propagation
	v := p.variables
	cl := p.clauses
	a.traceProcessing(c)
	var trail uint32
	var e error
	if satStorageSourceDebug {
		if v.scopeLevel != 0 {
			return false, ErrSATAssertion
		}
		if e = a.trailInvariant(); e != nil {
			return false, e
		}
		trail, e = v.trail.Size(v.trailRef)
		if e != nil {
			return false, e
		}
		if p.inconsistent {
			return false, ErrSATAssertion
		}
	}

	sz, e := cl.Size(c)
	if e != nil {
		return false, e
	}
	if satStorageSourceDebug && sz == 0 {
		return false, ErrSATAssertion
	}
	for i := uint32(0); i < sz; i++ {
		x, e := cl.Literal(c, i)
		if e != nil {
			return false, e
		}
		val, e := v.valueLiteral(x)
		if e != nil {
			return false, e
		}
		if val == lbool.True {
			if e = l.detachClause(c); e != nil {
				return false, e
			}
			return false, l.delClause(c)
		}
	}
	// Lexical scoped_detach is constructed before push. On exceptional exit the
	// source reattaches but does not pop the solver scope. Preserve that prefix.
	if e = l.detachClause(c); e != nil {
		return false, e
	}
	a.detachedClause = c
	a.detachActive = true
	defer func() {
		if a.detachActive {
			if e := l.attachClause(c); e != nil {
				a.cleanupCause = e
				if err == nil {
					err = e
				} else {
					err = errors.Join(err, e)
				}
			} else {
				a.detachActive = false
				a.detachedClause = StoredClauseRef{}
			}
		}
	}()
	if e = l.push(); e != nil {
		return false, e
	}
	i := uint32(0)
	for ; i < sz-1; i++ {
		x, e := cl.Literal(c, i)
		if e != nil {
			return false, e
		}
		if satStorageSourceDebug && p.inconsistent {
			return false, ErrSATAssertion
		}
		a.traceAssign(x.Negated())
		if e = p.assign(x.Negated(), satJustification{}); e != nil {
			return false, e
		}
		if _, e = p.propagateCore(false); e != nil {
			return false, e
		}
		if p.inconsistent {
			break
		}
	}
	if e = l.pop(1); e != nil {
		return false, e
	}
	if satStorageSourceDebug {
		if p.inconsistent || v.scopeLevel != 0 {
			return false, ErrSATAssertion
		}
		n, e := v.trail.Size(v.trailRef)
		if e != nil {
			return false, e
		}
		if n != trail {
			return false, ErrSATAssertion
		}
		if e = a.trailInvariant(); e != nil {
			return false, e
		}
	}
	if i == sz-1 {
		return true, nil
	}
	newSize := i + 1
	if satStorageSourceDebug && (newSize < 1 || newSize >= sz) {
		return false, ErrSATAssertion
	}
	a.traceReduced(c, newSize)
	j := uint32(0)
	for i = 0; i < newSize; i++ {
		x, e := cl.Literal(c, i)
		if e != nil {
			return false, e
		}
		val, e := v.valueLiteral(x)
		if e != nil {
			return false, e
		}
		switch val {
		case lbool.Undef:
			if e = cl.SetLiteral(c, j, x); e != nil {
				return false, e
			}
			j++
		case lbool.False:
		case lbool.True:
			return false, ErrSATAssertion
		}
	}
	a.elimLiterals += sz - j
	switch j {
	case 0:
		p.setConflict(satJustification{}, DefaultLiteral())
		a.discardedClause = c
		return false, nil
	case 1:
		x, e := cl.Literal(c, 0)
		if e != nil {
			return false, e
		}
		a.traceUnit(x)
		if e = p.assign(x, satJustification{}); e != nil {
			return false, e
		}
		if _, e = p.propagateCore(false); e != nil {
			return false, e
		}
		if e = l.delClause(c); e != nil {
			return false, e
		}
		a.detachActive = false
		a.detachedClause = StoredClauseRef{}
		if satStorageSourceDebug && !p.inconsistent {
			if e = a.trailInvariant(); e != nil {
				return false, e
			}
		}
		return false, nil
	case 2:
		if satStorageSourceDebug {
			for k := uint32(0); k < 2; k++ {
				x, e := cl.Literal(c, k)
				if e != nil {
					return false, e
				}
				val, e := v.valueLiteral(x)
				if e != nil {
					return false, e
				}
				if val != lbool.Undef {
					return false, ErrSATAssertion
				}
			}
		}
		x, e := cl.Literal(c, 0)
		if e != nil {
			return false, e
		}
		y, e := cl.Literal(c, 1)
		if e != nil {
			return false, e
		}
		if e = l.mkBinClause(x, y, false); e != nil {
			return false, e
		}
		if e = l.delClause(c); e != nil {
			return false, e
		}
		a.detachActive = false
		a.detachedClause = StoredClauseRef{}
		if e = a.trailInvariant(); e != nil {
			return false, e
		}
		return false, nil
	default:
		if e = cl.Shrink(c, j); e != nil {
			return false, e
		}
		if e = a.trailInvariant(); e != nil {
			return false, e
		}
		return true, nil
	}
}
