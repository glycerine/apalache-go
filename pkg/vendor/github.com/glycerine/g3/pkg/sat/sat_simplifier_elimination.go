package sat

// Source: src/sat/sat_simplifier.cpp:1085–1480.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

type satSimplifierVarCost struct{ variable, cost uint32 }

// Charged enclosing cold metadata retains the actual lexical owners on stopped
// cleanup. Costs are concrete source pair records, never copied clause owners.
type satSimplifierElimination struct {
	costs        []satSimplifierVarCost
	vars         satUintColumn
	reportActive bool
}

func (s *satSimplifier) closeElimination() error {
	x := &s.elimination
	s.closeEliminationCosts()
	if x.vars.runtime != nil {
		if e := x.vars.Finalize(); e != nil {
			return e
		}
		x.vars = satUintColumn{}
	}
	if x.reportActive {
		if e := s.elimReport.Stop(); e != nil {
			return e
		}
		x.reportActive = false
		if e := s.emitEliminationReport(); e != nil {
			return e
		}
	}
	return nil
}
func (s *satSimplifier) pushCost(v, c uint32) {
	x := &s.elimination
	x.costs = append(x.costs, satSimplifierVarCost{v, c})
}
func (s *satSimplifier) nonLearnedBinaries(l Literal) (uint32, error) {
	w, e := s.lifecycle.propagation.variables.watches.Get(uint32(l.Negated()))
	if e != nil {
		return 0, e
	}
	it, e := s.watchIterator(w)
	if e != nil {
		return 0, e
	}
	n := uint32(0)
	for i := uint32(0); i < it.end; i++ {
		x, e := satPropagationCursorRead(&it, i)
		if e != nil {
			return 0, e
		}
		if x.Binary() {
			b, e := x.Learned()
			if e != nil {
				return 0, e
			}
			if !b {
				n++
			}
		}
	}
	return n, nil
}
func (s *satSimplifier) eliminationCost(v BoolVar) (uint32, error) {
	pos := Literal(uint32(v) * 2)
	neg := pos.Negated()
	p, e := s.useList.Get(pos)
	if e != nil {
		return 0, e
	}
	n, e := s.useList.Get(neg)
	if e != nil {
		return 0, e
	}
	np, e := p.Size()
	if e != nil {
		return 0, e
	}
	nn, e := n.Size()
	if e != nil {
		return 0, e
	}
	bp, e := s.nonLearnedBinaries(pos)
	if e != nil {
		return 0, e
	}
	bn, e := s.nonLearnedBinaries(neg)
	if e != nil {
		return 0, e
	}
	cost := 2*np*nn + np*bn + nn*bp
	s.traceEliminationZeroCost(v, np, nn, bp, bn, cost)
	return cost, nil
}
func (s *satSimplifier) orderVariables() error {
	defer s.closeEliminationCosts()
	p := s.lifecycle.propagation
	x := &s.elimination
	if e := p.variables.elimTodo.open(); e != nil {
		return e
	}
	for _, v := range p.variables.elimTodo.values {
		external, e := p.variables.IsExternal(BoolVar(v))
		if e != nil {
			return e
		}
		if external {
			continue
		}
		eliminated, e := p.variables.wasEliminated(BoolVar(v))
		if e != nil {
			return e
		}
		if eliminated {
			continue
		}
		value, e := p.variables.valueLiteral(Literal(v * 2))
		if e != nil {
			return e
		}
		if value != lbool.Undef {
			continue
		}
		c, e := s.eliminationCost(BoolVar(v))
		if e != nil {
			return e
		}
		s.pushCost(v, c)
	}
	if e := p.variables.elimTodo.Reset(); e != nil {
		return e
	}
	s.sortEliminationCosts()
	s.traceEliminationSortedCosts(x.costs)
	for _, a := range x.costs {
		if e := x.vars.Push(a.variable); e != nil {
			return e
		}
	}
	// tmp's destructor occurs before order_vars_for_elim returns.
	s.closeEliminationCosts()
	return nil
}
func (s *satSimplifier) collectClauses(l Literal, out *satClauseWrapperVector) (err error) {
	u, e := s.useList.Get(l)
	if e != nil {
		return e
	}
	it, e := u.Begin()
	if e != nil {
		return e
	}
	if e = func() (err error) {
		defer func() {
			if e := it.Close(); e != nil {
				err = errors.Join(err, e)
			}
		}()
		for !it.AtEnd() {
			c, e := it.Current()
			if e != nil {
				return e
			}
			if e = out.Push(newSATClauseWrapper(c)); e != nil {
				return e
			}
			if satStorageSourceDebug {
				w, e := out.Back()
				if e != nil {
					return e
				}
				n, e := w.Size(s.lifecycle.propagation.clauses)
				if e != nil {
					return e
				}
				c, e = it.Current()
				if e != nil {
					return e
				}
				m, e := s.lifecycle.propagation.clauses.Size(c)
				if e != nil {
					return e
				}
				if n != m {
					return ErrSATAssertion
				}
			}
			if e = it.Next(); e != nil {
				return e
			}
		}
		return nil
	}(); e != nil {
		return e
	}
	w, e := s.lifecycle.propagation.variables.watches.Get(uint32(l.Negated()))
	if e != nil {
		return e
	}
	wi, e := s.watchIterator(w)
	if e != nil {
		return e
	}
	for i := uint32(0); i < wi.end; i++ {
		x, e := satPropagationCursorRead(&wi, i)
		if e != nil {
			return e
		}
		if !x.Binary() {
			continue
		}
		learned, e := x.Learned()
		if e != nil {
			return e
		}
		if learned {
			continue
		}
		l2, e := x.Literal()
		if e != nil {
			return e
		}
		if e = out.Push(newSATBinaryWrapper(l, l2)); e != nil {
			return e
		}
		if satStorageSourceDebug {
			b, e := out.Back()
			if e != nil {
				return e
			}
			n, e := b.Size(s.lifecycle.propagation.clauses)
			if e != nil {
				return e
			}
			if n != 2 {
				return ErrSATAssertion
			}
		}
	}
	return nil
}
func (s *satSimplifier) saveClauses(mc satModelConversionEntryRef, cs *satClauseWrapperVector) error {
	n := cs.Size()
	for i := uint32(0); i < n; i++ {
		c, e := cs.Get(i)
		if e != nil {
			return e
		}
		if e = s.lifecycle.propagation.converter.InsertWrapper(mc, s.lifecycle.propagation.clauses, c); e != nil {
			return e
		}
	}
	return nil
}
func (s *satSimplifier) addNonLearnedBinary(a, b Literal) error {
	p := s.lifecycle.propagation
	wa, e := p.variables.watches.Get(uint32(a.Negated()))
	if e != nil {
		return e
	}
	wb, e := p.variables.watches.Get(uint32(b.Negated()))
	if e != nil {
		return e
	}
	ia, e := s.watchIterator(wa)
	if e != nil {
		return e
	}
	for i := uint32(0); i < ia.end; i++ {
		x, e := satPropagationCursorRead(&ia, i)
		if e != nil {
			return e
		}
		if !x.Binary() {
			continue
		}
		l, e := x.Literal()
		if e != nil {
			return e
		}
		if l != b {
			continue
		}
		if e = satPropagationCursorWrite(&ia, i, satSimplifierBinaryRecord(b)); e != nil {
			return e
		}
		ib, e := s.watchIterator(wb)
		if e != nil {
			return e
		}
		found := false
		for j := uint32(0); j < ib.end; j++ {
			x, e := satPropagationCursorRead(&ib, j)
			if e != nil {
				return e
			}
			if !x.Binary() {
				continue
			}
			l, e := x.Literal()
			if e != nil {
				return e
			}
			if l == a {
				if e = satPropagationCursorWrite(&ib, j, satSimplifierBinaryRecord(a)); e != nil {
					return e
				}
				found = true
				break
			}
		}
		s.traceMissingReciprocalBinary(a, b, wa, wb, !found)
		if satStorageSourceDebug && !found {
			return ErrSATAssertion
		}
		return nil
	}
	if e = wa.Push(satSimplifierBinaryRecord(b)); e != nil {
		return e
	}
	return wb.Push(satSimplifierBinaryRecord(a))
}
func (s *satSimplifier) removeBinaries(l Literal) error {
	p := s.lifecycle.propagation
	w, e := p.variables.watches.Get(uint32(l.Negated()))
	if e != nil {
		return e
	}
	it, e := s.watchIterator(w)
	if e != nil {
		return e
	}
	for i := uint32(0); i < it.end; i++ {
		x, e := satPropagationCursorRead(&it, i)
		if e != nil {
			return e
		}
		if !x.Binary() {
			continue
		}
		l2, e := x.Literal()
		if e != nil {
			return e
		}
		other, e := p.variables.watches.Get(uint32(l2.Negated()))
		if e != nil {
			return e
		}
		oi, e := s.watchIterator(other)
		if e != nil {
			return e
		}
		out := uint32(0)
		for j := uint32(0); j < oi.end; j++ {
			y, e := satPropagationCursorRead(&oi, j)
			if e != nil {
				return e
			}
			remove := false
			if y.Binary() {
				q, e := y.Literal()
				if e != nil {
					return e
				}
				if q == l {
					s.traceRemovingBinary(l, q)
					learned, e := y.Learned()
					if e != nil {
						return e
					}
					if _, e = s.subBinTodo.Erase(newSATBinClause(l2, l, learned)); e != nil {
						return e
					}
					remove = true
				}
			}
			if !remove {
				if e = satPropagationCursorWrite(&oi, out, y); e != nil {
					return e
				}
				out++
			}
		}
		if e = p.finishPropagationList(&oi, other, out); e != nil {
			return e
		}
		x, e = satPropagationCursorRead(&it, i)
		if e != nil {
			return e
		}
		learned, e := x.Learned()
		if e != nil {
			return e
		}
		if _, e = s.subBinTodo.Erase(newSATBinClause(l, l2, learned)); e != nil {
			return e
		}
	}
	s.traceCollapsingBinaryWatch(l)
	return w.Finalize()
}
func (s *satSimplifier) removeEliminatedClauses(u satClauseUseListView, l Literal) (err error) {
	it, e := u.Begin()
	if e != nil {
		return e
	}
	defer func() {
		if e := it.Close(); e != nil {
			err = errors.Join(err, e)
		}
	}()
	a := s.lifecycle.propagation.clauses
	for !it.AtEnd() {
		c, e := it.Current()
		if e != nil {
			return e
		}
		if e = it.Next(); e != nil {
			return e
		}
		if satStorageSourceDebug {
			ok, e := a.ContainsLiteral(c, l)
			if e != nil {
				return e
			}
			if !ok {
				return ErrSATAssertion
			}
		}
		if e = a.SetRemoved(c, true); e != nil {
			return e
		}
		if e = s.useList.EraseExcept(c, l); e != nil {
			return e
		}
		if e = s.subTodo.Erase(c); e != nil {
			return e
		}
		s.traceEliminatedClause(c)
		s.needCleanup = true
	}
	return nil
}
func (s *satSimplifier) occurrenceLiteralCount(u satClauseUseListView) (n uint32, err error) {
	it, e := u.Begin()
	if e != nil {
		return 0, e
	}
	defer func() {
		if e := it.Close(); e != nil {
			err = errors.Join(err, e)
		}
	}()
	for !it.AtEnd() {
		c, e := it.Current()
		if e != nil {
			return 0, e
		}
		sz, e := s.lifecycle.propagation.clauses.Size(c)
		if e != nil {
			return 0, e
		}
		n += sz
		if e = it.Next(); e != nil {
			return 0, e
		}
	}
	return n, nil
}
func (s *satSimplifier) tryEliminate(v BoolVar) (bool, error) {
	s.traceEliminationProcessing(v)
	p := s.lifecycle.propagation
	pos := Literal(uint32(v) * 2)
	neg := pos.Negated()
	val, e := p.variables.valueLiteral(pos)
	if e != nil {
		return false, e
	}
	if val != lbool.Undef {
		return false, nil
	}
	bp, e := s.nonLearnedBinaries(pos)
	if e != nil {
		return false, e
	}
	bn, e := s.nonLearnedBinaries(neg)
	if e != nil {
		return false, e
	}
	po, e := s.useList.Get(pos)
	if e != nil {
		return false, e
	}
	no, e := s.useList.Get(neg)
	if e != nil {
		return false, e
	}
	np, e := po.Size()
	if e != nil {
		return false, e
	}
	nn, e := no.Size()
	if e != nil {
		return false, e
	}
	np += bp
	nn += bn
	s.traceEliminationOccurrences(v, np, nn)
	if np >= s.occCutoff && nn >= s.occCutoff {
		return false, nil
	}
	before := bp*2 + bn*2
	n, e := s.occurrenceLiteralCount(po)
	if e != nil {
		return false, e
	}
	before += n
	n, e = s.occurrenceLiteralCount(no)
	if e != nil {
		return false, e
	}
	before += n
	s.traceEliminationBeforeLiterals(v, np, nn, before)
	if np >= s.occRanges[2] && nn >= s.occRanges[2] && before > s.litRanges[2] && uint32(len(s.lifecycle.Original)) > s.clsCutoffs[1] {
		return false, nil
	}
	if np >= s.occRanges[1] && nn >= s.occRanges[1] && before > s.litRanges[1] && uint32(len(s.lifecycle.Original)) > s.clsCutoffs[0] && uint32(len(s.lifecycle.Original)) <= s.clsCutoffs[1] {
		return false, nil
	}
	if np >= s.occRanges[0] && nn >= s.occRanges[0] && before > s.litRanges[0] && uint32(len(s.lifecycle.Original)) <= s.clsCutoffs[0] {
		return false, nil
	}
	if e = s.posCls.Reset(); e != nil {
		return false, e
	}
	if e = s.negCls.Reset(); e != nil {
		return false, e
	}
	if e = s.collectClauses(pos, s.posCls); e != nil {
		return false, e
	}
	if e = s.collectClauses(neg, s.negCls); e != nil {
		return false, e
	}
	s.traceEliminationCollecting()
	after := uint32(0)
	pc, nc := s.posCls.Size(), s.negCls.Size()
	for i := uint32(0); i < pc; i++ {
		for j := uint32(0); j < nc; j++ {
			if e = s.resetNewClause(); e != nil {
				return false, e
			}
			a, e := s.posCls.Get(i)
			if e != nil {
				return false, e
			}
			b, e := s.negCls.Get(j)
			if e != nil {
				return false, e
			}
			ok, e := s.kernel.resolve(a, b, pos, s.literalStore, s.newCls)
			if e != nil {
				return false, e
			}
			if ok {
				s.traceEliminationResolventDetail(a, b)
				after++
				if after > np+nn {
					s.traceEliminationTooMany(after)
					return false, nil
				}
			}
		}
	}
	s.traceEliminationFound(np+nn, after)
	s.kernel.elimCounter -= int32(np*nn + before)
	mc, e := p.converter.Mk(satModelElimVar, v)
	if e != nil {
		return false, e
	}
	if e = s.saveClauses(mc, s.posCls); e != nil {
		return false, e
	}
	if e = s.saveClauses(mc, s.negCls); e != nil {
		return false, e
	}
	if e = p.variables.setEliminated(v, true); e != nil {
		return false, e
	}
	if e = s.removeBinaries(pos); e != nil {
		return false, e
	}
	if e = s.removeBinaries(neg); e != nil {
		return false, e
	}
	if e = s.removeEliminatedClauses(po, pos); e != nil {
		return false, e
	}
	if e = s.removeEliminatedClauses(no, neg); e != nil {
		return false, e
	}
	if e = po.Reset(); e != nil {
		return false, e
	}
	if e = no.Reset(); e != nil {
		return false, e
	}
	s.kernel.elimCounter -= int32(np*nn + before) // source's second identical subtraction
	for i := uint32(0); i < pc; i++ {
		for j := uint32(0); j < nc; j++ {
			if e = s.resetNewClause(); e != nil {
				return false, e
			}
			a, e := s.posCls.Get(i)
			if e != nil {
				return false, e
			}
			b, e := s.negCls.Get(j)
			if e != nil {
				return false, e
			}
			ok, e := s.kernel.resolve(a, b, pos, s.literalStore, s.newCls)
			if e != nil {
				return false, e
			}
			if !ok {
				continue
			}
			s.traceEliminationNewClause(a, b)
			satisfied, e := s.cleanupLiterals(s.newCls)
			if e != nil {
				return false, e
			}
			if satisfied {
				continue
			}
			n, e := s.literalStore.Size(s.newCls)
			if e != nil {
				return false, e
			}
			switch n {
			case 0:
				p.setConflict(satJustification{}, DefaultLiteral())
			case 1:
				a, e := s.literalStore.Read(s.newCls, 0)
				if e != nil {
					return false, e
				}
				if e = s.propagateUnit(a); e != nil {
					return false, e
				}
			case 2:
				s.lifecycle.mkBinary++
				a, e := s.literalStore.Read(s.newCls, 0)
				if e != nil {
					return false, e
				}
				b, e := s.literalStore.Read(s.newCls, 1)
				if e != nil {
					return false, e
				}
				if e = s.addNonLearnedBinary(a, b); e != nil {
					return false, e
				}
				a, e = s.literalStore.Read(s.newCls, 0)
				if e != nil {
					return false, e
				}
				b, e = s.literalStore.Read(s.newCls, 1)
				if e != nil {
					return false, e
				}
				if e = s.backSubsumptionBinary(a, b, false); e != nil {
					return false, e
				}
			default:
				if n == 3 {
					s.lifecycle.mkTernary++
				} else {
					s.lifecycle.mkNary++
				}
				span, e := satSimplifierCapturedSpan(s.literalStore, s.newCls)
				if e != nil {
					return false, e
				}
				c, e := p.clauses.mkClauseSpan(n, span, false)
				s.lifecycle.pendingCandidate = c
				if e != nil {
					return false, e
				}
				if e = s.lifecycle.pushClause(false, c); e != nil {
					return false, e
				}
				s.lifecycle.pendingCandidate = StoredClauseRef{}
				if e = s.useList.Insert(c); e != nil {
					return false, e
				}
				if s.kernel.subCounter > 0 {
					e = s.backSubsumption1(c)
				} else {
					e = s.backSubsumption0(c)
				}
				if e != nil {
					return false, e
				}
			}
			if p.inconsistent {
				return true, nil
			}
		}
	}
	return true, nil
}
func (s *satSimplifier) eliminateVariables() (err error) {
	if !s.elimVars {
		return nil
	}
	s.elimReport = stopwatch.Stopwatch{}
	s.elimReportVars = s.numElimVars
	if e := s.elimReport.Start(); e != nil {
		return e
	}
	s.elimination.reportActive = true
	s.elimination.vars.Bind(s.lifecycle.propagation.memory)
	defer func() {
		if e := s.closeElimination(); e != nil {
			err = errors.Join(err, e)
		}
	}()
	if e := s.orderVariables(); e != nil {
		return e
	}
	n, e := s.elimination.vars.Size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		if e = s.lifecycle.propagation.checkpoint(); e != nil {
			return e
		}
		if s.kernel.elimCounter < 0 {
			return nil
		}
		v, e := s.elimination.vars.Read(i)
		if e != nil {
			return e
		}
		ok, e := s.tryEliminate(BoolVar(v))
		if e != nil {
			return e
		}
		if ok {
			s.numElimVars++
		}
	}
	if e = s.posCls.Finalize(); e != nil {
		return e
	}
	if e = s.negCls.Finalize(); e != nil {
		return e
	}
	if s.newCls != (satSolverLiteralListRef{}) {
		if e = s.literalStore.Free(s.newCls); e != nil {
			return e
		}
		s.newCls = satSolverLiteralListRef{}
	}
	return nil
}

// Operands originate in valid clause wrappers; constructor guard stays reached.
func satSimplifierBinaryRecord(l Literal) satWatched {
	w, _ := newSATBinaryWatch(l, false)
	return w
}

func (s *satSimplifier) resetNewClause() error {
	if s.newCls == (satSolverLiteralListRef{}) {
		var e error
		s.newCls, e = s.literalStore.NewList()
		return e
	}
	return s.literalStore.Shrink(s.newCls, 0)
}

func (s *satSimplifier) closeEliminationCosts() {
	s.elimination.costs = nil
}
func (s *satSimplifier) sortEliminationCosts() {
	x := &s.elimination
	sort.SliceStable(x.costs, func(i, j int) bool { return x.costs[i].cost < x.costs[j].cost })
}
