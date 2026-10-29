package sat

// Source: src/sat/sat_solver.cpp:2620–2756,2818–2841; src/sat/sat_simplifier.h:190–194.

import (
	"github.com/glycerine/g3/pkg/arena"
)

// Actual disjoint clause-vector owner ranges during unsafe-exception compaction.
// The current source cohort is not copied. Kept [0,out), unprocessed [in,end),
// and a current deletion stage describe the actual remaining scalar ownership.
type satSolverUserGC struct {
	in, out, end, drainPrefix, drainSuffix uint32
	learned, running, active               bool
}

func (s *Solver) userGCVector() *[]StoredClauseRef {
	if s.userGC.learned {
		return &s.Lifecycle.learned
	}
	return &s.Lifecycle.Original
}
func (s *Solver) cleanupUserGC() error {
	g := &s.userGC
	if !g.active {
		return nil
	}
	vec := s.userGCVector()
	if uint64(g.end) > uint64(len(*vec)) {
		return arena.ErrBounds
	}
	for g.drainPrefix < g.out {
		c := (*vec)[g.drainPrefix]
		if e := s.Clauses.Delete(c); e != nil {
			return e
		}
		g.drainPrefix++
	}
	if g.drainSuffix < g.in {
		g.drainSuffix = g.in
	}
	for g.drainSuffix < g.end {
		c := (*vec)[g.drainSuffix]
		if e := s.Clauses.Delete(c); e != nil {
			return e
		}
		g.drainSuffix++
	}
	*vec = (*vec)[:0]
	g.active = false
	s.Lifecycle.userContinuation = nil
	return nil
}
func (s *Solver) gcLit(learned bool, l Literal) (err error) {
	g := &s.userGC
	*g = satSolverUserGC{learned: learned, running: true, active: true}
	vec := s.userGCVector()
	g.end = uint32(len(*vec))
	s.Lifecycle.userContinuation = s
	defer func() {
		g.running = false
		if err == nil {
			g.active = false
			s.Lifecycle.userContinuation = nil
		}
	}()
	for g.in < uint32(len(*vec)) {
		c := (*vec)[g.in]
		yes, e := s.Clauses.ContainsLiteral(c, l)
		if e != nil {
			return e
		}
		if !yes {
			yes, e = s.Clauses.ContainsLiteral(c, l.Negated())
			if e != nil {
				return e
			}
		}
		if yes {
			if e = s.Lifecycle.detachClause(c); e != nil {
				return e
			}
			if e = s.Lifecycle.delClause(c); e != nil {
				return e
			}
		} else {
			(*vec)[g.out] = c
			g.out++
		}
		g.in++
	}
	*vec = (*vec)[:g.out]
	return nil
}
func (s *Solver) gcBin(l Literal) error {
	for _, r := range s.Variables.watches.lists {
		store := s.Variables.watches.store
		n, e := store.Size(r)
		if e != nil {
			return e
		}
		out := uint32(0)
		for i := uint32(0); i < n; i++ {
			w, e := store.Read(r, i)
			if e != nil {
				return e
			}
			skip := false
			if w.Binary() {
				a, e := w.Literal()
				if e != nil {
					return e
				}
				skip = a.variable() == l.variable()
			}
			if !skip {
				if e = store.Write(r, out, w); e != nil {
					return e
				}
				out++
			}
		}
		if e = store.Shrink(r, out); e != nil {
			return e
		}
	}
	return nil
}
func (s *Solver) maxVarClauses(cs []StoredClauseRef, v uint32) (uint32, error) {
	for _, c := range cs {
		n, e := s.Clauses.Size(c)
		if e != nil {
			return v, e
		}
		for i := uint32(0); i < n; i++ {
			l, e := s.Clauses.Literal(c, i)
			if e != nil {
				return v, e
			}
			if uint32(l.variable()) > v {
				v = uint32(l.variable())
			}
		}
	}
	return v, nil
}
func (s *Solver) maxVarBins(learned bool, v uint32) (uint32, error) {
	s.userBinClauses = s.userBinClauses[:0]
	for idx, r := range s.Variables.watches.lists {
		l := Literal(idx).Negated()
		n, e := s.Variables.watches.store.Size(r)
		if e != nil {
			return v, e
		}
		for i := uint32(0); i < n; i++ {
			w, e := s.Variables.watches.store.Read(r, i)
			if e != nil {
				return v, e
			}
			if !w.Binary() {
				continue
			}
			if !learned {
				yes, e := w.Learned()
				if e != nil {
					return v, e
				}
				if yes {
					continue
				}
			}
			l2, e := w.Literal()
			if e != nil {
				return v, e
			}
			if l.index() > l2.index() {
				continue
			}
			s.userBinClauses = append(s.userBinClauses, newSATSolverBinPair(l, l2))
		}
	}
	for _, c := range s.userBinClauses {
		if uint32(c.Literal1().variable()) > v {
			v = uint32(c.Literal1().variable())
		}
		if uint32(c.Literal2().variable()) > v {
			v = uint32(c.Literal2().variable())
		}
	}
	return v, nil
}
func (s *Solver) gcVar(v uint32) error {
	if v > 0 {
		w, e := s.maxVarClauses(s.Lifecycle.learned, v-1)
		if e != nil {
			return e
		}
		w, e = s.maxVarClauses(s.Lifecycle.Original, w)
		if e != nil {
			return e
		}
		w, e = s.maxVarBins(true, w)
		if e != nil {
			return e
		}
		w, e = s.maxVarBins(false, w)
		if e != nil {
			return e
		}
		v, e = s.converter.MaxVar(w)
		if e != nil {
			return e
		}
		for i := uint32(0); ; i++ {
			n, e := s.Variables.trail.Size(s.Variables.trailRef)
			if e != nil {
				return e
			}
			if i >= n {
				break
			}
			l, e := s.Variables.trailRead(i)
			if e != nil {
				return e
			}
			if uint32(l.variable()) > w {
				w = uint32(l.variable())
			}
		}
		if w+1 > v {
			v = w + 1
		}
	}
	n, e := s.Variables.numVarsChecked()
	if e != nil {
		return e
	}
	if v >= n {
		return nil
	}
	for i := v; ; i++ {
		n, e := s.Variables.numVarsChecked()
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		if e = s.Variables.queue.DelVar(BoolVar(i)); e != nil {
			return e
		}
		if e = s.probing.resetCache(MakeLiteral(BoolVar(i), true)); e != nil {
			return e
		}
		if e = s.probing.resetCache(MakeLiteral(BoolVar(i), false)); e != nil {
			return e
		}
	}
	x := s.Variables
	if e = x.watches.Shrink(2 * v); e != nil {
		return e
	}
	if uint64(2*v) > uint64(len(x.assignment)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(x.assignment[2*v:])
	x.assignment = x.assignment[:2*v]
	if e = x.justification.shrink(v); e != nil {
		return e
	}
	if uint64(v) > uint64(len(x.decision)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(x.decision[v:])
	x.decision = x.decision[:v]
	if uint64(v) > uint64(len(x.eliminated)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(x.eliminated[v:])
	x.eliminated = x.eliminated[:v]
	if uint64(v) > uint64(len(x.external)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(x.external[v:])
	x.external = x.external[:v]
	if uint64(v) > uint64(len(x.activity.values)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(x.activity.values[v:])
	x.activity.values = x.activity.values[:v]
	if uint64(v) > uint64(len(x.level)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(x.level[v:])
	x.level = x.level[:v]
	if uint64(v) > uint64(len(x.mark)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(x.mark[v:])
	x.mark = x.mark[:v]
	if uint64(2*v) > uint64(len(x.literalMark)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(x.literalMark[2*v:])
	x.literalMark = x.literalMark[:2*v]
	if uint64(v) > uint64(len(x.phase)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(x.phase[v:])
	x.phase = x.phase[:v]
	if uint64(v) > uint64(len(x.previousPhase)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(x.previousPhase[v:])
	x.previousPhase = x.previousPhase[:v]
	if uint64(v) > uint64(len(x.assignedSinceGC)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(x.assignedSinceGC[v:])
	x.assignedSinceGC = x.assignedSinceGC[:v]
	if e = x.elimTodo.Reset(); e != nil {
		return e
	}
	if e = s.simplifier.subTodo.Reset(); e != nil {
		return e
	}
	return s.simplifier.subBinTodo.Reset()
}
func (s *Solver) UserPush() error {
	if e := s.open(); e != nil {
		return e
	}
	v, e := s.Variables.mkVar(true, false)
	if e != nil {
		return e
	}
	lit := MakeLiteral(v, false)
	if e = s.conflict.userScopeLiterals.Push(uint32(lit)); e != nil {
		return e
	}
	s.traceUserPush(lit)
	return nil
}
func (s *Solver) UserPop(scopes uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	if e := s.popToBase(); e != nil {
		return e
	}
	s.traceUserPopDisplay()
	for scopes > 0 {
		n, e := s.conflict.userScopeLiterals.Size()
		if e != nil {
			return e
		}
		raw, e := s.conflict.userScopeLiterals.Read(n - 1)
		if e != nil {
			return e
		}
		l := Literal(raw)
		if e = s.conflict.userScopeLiterals.Shrink(n - 1); e != nil {
			return e
		}
		for _, lit := range [2]Literal{l, l.Negated()} {
			list, e := s.Variables.watches.Get(lit.index())
			if e != nil {
				return e
			}
			if e = list.SetEnd(0); e != nil {
				return e
			}
		}
		if e = s.gcLit(true, l); e != nil {
			return e
		}
		if e = s.gcLit(false, l); e != nil {
			return e
		}
		if e = s.gcBin(l); e != nil {
			return e
		}
		s.traceUserPopGC(l)
		scopes--
		for i := uint32(0); ; i++ {
			n, e := s.Variables.trail.Size(s.Variables.trailRef)
			if e != nil {
				return e
			}
			if i >= n {
				break
			}
			a, e := s.Variables.trailRead(i)
			if e != nil {
				return e
			}
			if a == l {
				s.traceUserPopTrail()
				if e = s.Lifecycle.unassignVars(i); e != nil {
					return e
				}
				break
			}
		}
		if e = s.gcVar(uint32(l.variable())); e != nil {
			return e
		}
	}
	s.Variables.qhead = 0
	_, e := s.Lifecycle.propagate(false)
	return e
}
