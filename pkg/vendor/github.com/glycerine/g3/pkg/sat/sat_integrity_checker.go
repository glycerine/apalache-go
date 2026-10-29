package sat

// Source: src/sat/sat_integrity_checker.cpp; src/sat/sat_integrity_checker.h.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"

	"github.com/glycerine/g3/pkg/arena"
)

// A source stack checker borrows the actual solver member owners. It owns no
// clauses and has no separately allocated header. GC holds the sole frozen count.
type satIntegrityChecker struct {
	lifecycle    *satSolverClauseLifecycle
	gc           *satSolverGC
	debugActions bool
}

func newSATIntegrityChecker(l *satSolverClauseLifecycle, g *satSolverGC) (satIntegrityChecker, error) {
	if e := l.open(); e != nil {
		return satIntegrityChecker{}, e
	}
	if g == nil || g.self != g || g.lifecycle() != l {
		return satIntegrityChecker{}, ErrSATOwner
	}
	if g.closed || g.closing {
		return satIntegrityChecker{}, arena.ErrClosed
	}
	return satIntegrityChecker{lifecycle: l, gc: g}, nil
}
func (c satIntegrityChecker) open() error {
	if c.lifecycle == nil {
		return ErrSATOwner
	}
	return c.lifecycle.open()
}
func (c satIntegrityChecker) literal(cl StoredClauseRef, i uint32) (Literal, error) {
	return c.lifecycle.propagation.clauses.Literal(cl, i)
}
func (c satIntegrityChecker) eliminated(v BoolVar) (bool, error) {
	return c.lifecycle.propagation.variables.wasEliminated(v)
}
func (c satIntegrityChecker) containsTernary(index uint32, a, b Literal) (bool, error) {
	v := c.lifecycle.propagation.variables
	list, e := v.watches.Get(index)
	if e != nil {
		return false, e
	}
	ref, e := list.ref()
	if e != nil {
		return false, e
	}
	w, e := newSATTernaryWatch(a, b)
	if e != nil {
		return false, e
	}
	return v.watches.store.Contains(ref, w)
}
func (c satIntegrityChecker) containsClause(index uint32, cl StoredClauseRef, offset satClauseOffset) (bool, error) {
	p := c.lifecycle.propagation
	list, e := p.variables.watches.Get(index)
	if e != nil {
		return false, e
	}
	ref, e := list.ref()
	if e != nil {
		return false, e
	}
	it, e := p.variables.watches.store.Iter(ref)
	if e != nil {
		return false, e
	}
	for {
		w, ok, e := it.Next()
		if e != nil {
			return false, e
		}
		if !ok {
			break
		}
		if w.Clause() {
			off, e := w.ClauseOffset()
			if e != nil {
				return false, e
			}
			if off == offset {
				if satStorageSourceDebug {
					enabled, e := diagnostic.BorrowProcessDebugContext().AssertionsEnabled()
					if e != nil {
						return false, e
					}
					if enabled {
						blocked, e := w.BlockedLiteral()
						if e != nil {
							return false, e
						}
						yes, e := p.clauses.ContainsLiteral(cl, blocked)
						if e != nil {
							return false, e
						}
						if e = satIntegrityAssertion(yes, c.debugActions, 43, "c.contains(it->get_blocked_literal())"); e != nil {
							return false, e
						}
					}
				}
				return true, nil
			}
		}
	}
	// Source UNREACHABLE has undefined release continuation on GNU; expose a
	// checked assertion error to unbound direct callers. The real cleaner's
	// bound debugger can continue to the following VERIFY after a C action.
	if c.debugActions && satStorageSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		if d.HasAssertionActions() {
			if e := d.UnreachableSource("src/sat/sat_integrity_checker.cpp", 48); e != nil {
				return false, e
			}
			return false, nil
		}
	}
	return false, ErrSATAssertion
}
func (c satIntegrityChecker) verifyClauseWatch(line int32, condition string) error {
	if c.debugActions && satStorageSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		if d.HasAssertionActions() {
			return d.VerifyFailure("src/sat/sat_integrity_checker.cpp", line, condition)
		}
	}
	return ErrSATAssertion
}
func (c satIntegrityChecker) checkClause(cl StoredClauseRef) (bool, error) {
	if e := c.open(); e != nil {
		return false, e
	}
	p := c.lifecycle.propagation
	a := p.clauses
	v := p.variables
	tags := c.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if satStorageSourceDebug {
		enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
		if e != nil {
			return false, e
		}
		if enabled {
			removed, e := a.Removed(cl)
			if e != nil {
				return false, e
			}
			if e = satIntegrityAssertion(!removed, c.debugActions, 53, "!c.was_removed()"); e != nil {
				return false, e
			}
		}
	}
	for i := uint32(0); ; i++ {
		n, e := a.Size(cl)
		if e != nil {
			return false, e
		}
		if i >= n {
			break
		}
		if satStorageSourceDebug {
			enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
			if e != nil {
				return false, e
			}
			if enabled {
				lit, e := a.Literal(cl, i)
				if e != nil {
					return false, e
				}
				vars, e := v.numVarsChecked()
				if e != nil {
					return false, e
				}
				if e = satIntegrityAssertion(uint32(lit.variable()) <= vars, c.debugActions, 55, "c[i].var() <= s.num_vars()"); e != nil {
					return false, e
				}
			}
		}
		if tags.ShouldTrace("sat_bug") && c.gc != nil && c.gc.conflict != nil && c.gc.conflict.solver != nil {
			lit, e := a.Literal(cl, i)
			if e != nil {
				return false, e
			}
			gone, e := c.eliminated(lit.variable())
			if e != nil {
				return false, e
			}
			if gone {
				c.traceEliminatedClause(cl, uint32(lit.variable()))
			}
		}
		if satStorageSourceDebug {
			enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
			if e != nil {
				return false, e
			}
			if enabled {
				lit, e := a.Literal(cl, i)
				if e != nil {
					return false, e
				}
				gone, e := c.eliminated(lit.variable())
				if e != nil {
					return false, e
				}
				if e = satIntegrityAssertion(!gone, c.debugActions, 60, "!s.was_eliminated(c[i].var())"); e != nil {
					return false, e
				}
			}
		}
	}
	if satStorageSourceDebug {
		enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
		if e != nil {
			return false, e
		}
		if enabled {
			yes, e := a.CheckApprox(cl)
			if e != nil {
				return false, e
			}
			if e = satIntegrityAssertion(yes, c.debugActions, 63, "c.check_approx()"); e != nil {
				return false, e
			}
		}
	}
	frozen, e := a.Frozen(cl)
	if e != nil {
		return false, e
	}
	if frozen {
		return true, nil
	}
	n, e := a.Size(cl)
	if e != nil {
		return false, e
	}
	if n == 3 {
		if tags.ShouldTrace("sat_ter_watch_bug") {
			first, e := a.Literal(cl, 0)
			if e != nil {
				return false, e
			}
			second, e := a.Literal(cl, 1)
			if e != nil {
				return false, e
			}
			third, e := a.Literal(cl, 2)
			if e != nil {
				return false, e
			}
			watched, e := c.containsTernary(first.Negated().index(), second, third)
			if e != nil {
				return false, e
			}
			if !watched {
				c.traceMissingTernaryWatch(cl, first.Negated().index())
			}
		}
		// Each VERIFY expression independently reads its source fields, in order.
		for k := uint32(0); k < 3; k++ {
			x, e := a.Literal(cl, k)
			if e != nil {
				return false, e
			}
			first, second := uint32(1), uint32(2)
			if k == 1 {
				first, second = 0, 2
			}
			if k == 2 {
				first, second = 0, 1
			}
			y, e := a.Literal(cl, first)
			if e != nil {
				return false, e
			}
			z, e := a.Literal(cl, second)
			if e != nil {
				return false, e
			}
			yes, e := c.containsTernary(x.Negated().index(), y, z)
			if e != nil {
				return false, e
			}
			if !yes {
				conditions := [...]string{
					"contains_watched(s.get_wlist(~c[0]), c[1], c[2])",
					"contains_watched(s.get_wlist(~c[1]), c[0], c[2])",
					"contains_watched(s.get_wlist(~c[2]), c[0], c[1])",
				}
				if e = c.verifyClauseWatch(int32(73+k), conditions[k]); e != nil {
					return false, e
				}
			}
		}
	} else {
		first, e := a.Literal(cl, 0)
		if e != nil {
			return false, e
		}
		value, e := v.valueLiteral(first)
		if e != nil {
			return false, e
		}
		watchedFalse := value == lbool.False
		if !watchedFalse {
			second, e := a.Literal(cl, 1)
			if e != nil {
				return false, e
			}
			value, e = v.valueLiteral(second)
			if e != nil {
				return false, e
			}
			watchedFalse = value == lbool.False
		}
		if watchedFalse {
			onStack := false
			for i := v.qhead; ; i++ {
				size, e := v.trail.Size(v.trailRef)
				if e != nil {
					return false, e
				}
				if i >= size {
					break
				}
				trail, e := v.trailRead(i)
				if e != nil {
					return false, e
				}
				x, e := a.Literal(cl, 0)
				if e != nil {
					return false, e
				}
				same := trail.variable() == x.variable()
				if !same {
					x, e = a.Literal(cl, 1)
					if e != nil {
						return false, e
					}
					same = trail.variable() == x.variable()
				}
				if same {
					onStack = true
					break
				}
			}
			if !onStack {
				status, e := p.status(cl)
				if e != nil {
					return false, e
				}
				if status != lbool.True {
					for i := uint32(2); ; i++ {
						size, e := a.Size(cl)
						if e != nil {
							return false, e
						}
						if i >= size {
							break
						}
						if tags.ShouldTrace("sat_bug") {
							lit, e := a.Literal(cl, i)
							if e != nil {
								return false, e
							}
							val, e := v.valueLiteral(lit)
							if e != nil {
								return false, e
							}
							if val != lbool.False {
								c.traceResidualClause(cl)
							}
						}
						if satStorageSourceDebug {
							enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
							if e != nil {
								return false, e
							}
							if !enabled {
								continue
							}
							lit, e := a.Literal(cl, i)
							if e != nil {
								return false, e
							}
							val, e := v.valueLiteral(lit)
							if e != nil {
								return false, e
							}
							if e = satIntegrityAssertion(val == lbool.False, c.debugActions, 93, "s.value(c[i]) == l_false"); e != nil {
								return false, e
							}
						}
					}
				}
			}
		}
		for k := uint32(0); k < 2; k++ {
			x, e := a.Literal(cl, k)
			if e != nil {
				return false, e
			}
			offset, e := a.Offset(cl)
			if e != nil {
				return false, e
			}
			yes, e := c.containsClause(x.Negated().index(), cl, offset)
			if e != nil {
				return false, e
			}
			if !yes {
				condition := "contains_watched(s.get_wlist(~c[0]), c, s.get_offset(c))"
				if k != 0 {
					condition = "contains_watched(s.get_wlist(~c[1]), c, s.get_offset(c))"
				}
				if e = c.verifyClauseWatch(int32(99+k), condition); e != nil {
					return false, e
				}
			}
		}
	}
	return true, nil
}
func (c satIntegrityChecker) checkClauseVector(cs []StoredClauseRef) (bool, error) {
	if e := c.open(); e != nil {
		return false, e
	}
	for i := 0; i < len(cs); i++ {
		enabled := false
		if satStorageSourceDebug {
			on, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
			if e != nil {
				return false, e
			}
			enabled = on
		}
		if enabled {
			yes, e := c.checkClause(cs[i])
			if e != nil {
				return false, e
			}
			if e = satIntegrityAssertion(yes, c.debugActions, 107, "check_clause(*(*it))"); e != nil {
				return false, e
			}
		}
	}
	return true, nil
}
func (c satIntegrityChecker) checkClauses() (bool, error) {
	return c.checkClauseVector(c.lifecycle.Original)
}
func (c satIntegrityChecker) checkLearnedClauses() (bool, error) {
	if e := c.open(); e != nil {
		return false, e
	}
	count := uint32(0)
	for _, cl := range c.lifecycle.Original {
		frozen, e := c.lifecycle.propagation.clauses.Frozen(cl)
		if e != nil {
			return false, e
		}
		if frozen {
			count++
		}
	}
	if satStorageSourceDebug {
		enabled, e := diagnostic.BorrowProcessDebugContext().AssertionsEnabled()
		if e != nil {
			return false, e
		}
		if enabled {
			if c.gc == nil || c.gc.self != c.gc || c.gc.lifecycle() != c.lifecycle {
				return false, ErrSATOwner
			}
			if c.gc.closed || c.gc.closing {
				return false, arena.ErrClosed
			}
			if e = satIntegrityAssertion(count == c.gc.numFrozen, c.debugActions, 124, "num_frozen == s.m_num_frozen"); e != nil {
				return false, e
			}
		}
	}
	return c.checkClauseVector(c.lifecycle.learned)
}
func (c satIntegrityChecker) checkAssignment() (bool, error) {
	if e := c.open(); e != nil {
		return false, e
	}
	return true, nil
}
func (c satIntegrityChecker) checkBoolVars() (bool, error) {
	if e := c.open(); e != nil {
		return false, e
	}
	v := c.lifecycle.propagation.variables
	if satStorageSourceDebug {
		enabled, e := diagnostic.BorrowProcessDebugContext().AssertionsEnabled()
		if e != nil {
			return false, e
		}
		if enabled {
			n, e := v.numVarsChecked()
			if e != nil {
				return false, e
			}
			checks := [...]struct {
				got       uint32
				want      uint32
				line      int32
				condition string
			}{
				{uint32(len(v.watches.lists)), n * 2, 133, "s.m_watches.size() == s.num_vars() * 2"},
				{uint32(len(v.assignment)), n * 2, 134, "s.m_assignment.size() == s.num_vars() * 2"},
				{uint32(len(v.literalMark)), n * 2, 135, "s.m_lit_mark.size() == s.num_vars() * 2"},
				{0, n, 136, "s.m_justification.size() == s.num_vars()"},
				{uint32(len(v.decision)), n, 137, "s.m_decision.size() == s.num_vars()"},
				{uint32(len(v.eliminated)), n, 138, "s.m_eliminated.size() == s.num_vars()"},
				{uint32(len(v.external)), n, 139, "s.m_external.size() == s.num_vars()"},
				{uint32(len(v.level)), n, 140, "s.m_level.size() == s.num_vars()"},
				{uint32(len(v.mark)), n, 141, "s.m_mark.size() == s.num_vars()"},
				{0, n, 142, "s.m_activity.size() == s.num_vars()"},
				{uint32(len(v.phase)), n, 143, "s.m_phase.size() == s.num_vars()"},
				{uint32(len(v.previousPhase)), n, 144, "s.m_prev_phase.size() == s.num_vars()"},
				{uint32(len(v.assignedSinceGC)), n, 145, "s.m_assigned_since_gc.size() == s.num_vars()"},
			}
			for i := range checks {
				reached := enabled
				if i != 0 {
					reached, e = diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
					if e != nil {
						return false, e
					}
				}
				if !reached {
					continue
				}
				if checks[i].line == 136 {
					checks[i].got, e = v.justification.size()
				} else if checks[i].line == 142 {
					e = v.activity.open()
					if e == nil {
						checks[i].got = uint32(len(v.activity.values))
					}
				}
				if e != nil {
					return false, e
				}
				if e = satIntegrityAssertion(checks[i].got == checks[i].want, c.debugActions, checks[i].line, checks[i].condition); e != nil {
					return false, e
				}
			}
		}
	}
	for variable := uint32(0); ; variable++ {
		n, e := v.numVarsChecked()
		if e != nil {
			return false, e
		}
		if variable >= n {
			break
		}
		gone, e := v.wasEliminated(BoolVar(variable))
		if e != nil {
			return false, e
		}
		if gone && satStorageSourceDebug {
			for sign := uint32(0); sign < 2; sign++ {
				reached, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
				if e != nil {
					return false, e
				}
				if !reached {
					continue
				}
				list, e := v.watches.Get(variable*2 + sign)
				if e != nil {
					return false, e
				}
				size, e := list.Size()
				if e != nil {
					return false, e
				}
				line := int32(148 + sign)
				condition := "s.get_wlist(literal(v, false)).empty()"
				if sign != 0 {
					condition = "s.get_wlist(literal(v, true)).empty()"
				}
				if e = satIntegrityAssertion(size == 0, c.debugActions, line, condition); e != nil {
					return false, e
				}
			}
		}
	}
	return true, nil
}
func (c satIntegrityChecker) checkWatches() (bool, error) {
	if e := c.open(); e != nil {
		return false, e
	}
	if !satStorageSourceDebug {
		return true, nil
	}
	tags := c.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	p := c.lifecycle.propagation
	v := p.variables
	for index := uint32(0); uint64(index) < uint64(len(v.watches.lists)); index++ {
		literal := Literal(index).Negated()
		list, e := v.watches.Get(index)
		if e != nil {
			return false, e
		}
		if tags.ShouldTrace("sat_bug") {
			gone, e := v.wasEliminated(literal.variable())
			if e != nil {
				return false, e
			}
			if gone {
				size, e := list.Size()
				if e != nil {
					return false, e
				}
				if size != 0 {
					c.traceEliminatedWatch(literal)
				}
			}
		}
		enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
		if e != nil {
			return false, e
		}
		if enabled {
			gone, e := v.wasEliminated(literal.variable())
			if e != nil {
				return false, e
			}
			if gone {
				size, e := list.Size()
				if e != nil {
					return false, e
				}
				if e = satIntegrityAssertion(size == 0, c.debugActions, 167, "!s.was_eliminated(l.var()) || wlist.empty()"); e != nil {
					return false, e
				}
			}
		}
		ref, e := list.ref()
		if e != nil {
			return false, e
		}
		it, e := v.watches.store.Iter(ref)
		if e != nil {
			return false, e
		}
		for {
			w, ok, e := it.Next()
			if e != nil {
				return false, e
			}
			if !ok {
				break
			}
			switch w.Kind() {
			case 0:
				enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
				if e != nil {
					return false, e
				}
				if enabled {
					other, e := w.Literal()
					if e != nil {
						return false, e
					}
					otherGone, e := v.wasEliminated(other.variable())
					if e != nil {
						return false, e
					}
					if e = satIntegrityAssertion(!otherGone, c.debugActions, 171, "!s.was_eliminated(w.get_literal().var())"); e != nil {
						return false, e
					}
				}
				traceBinary := tags.ShouldTrace("sat_watched_bug")
				if traceBinary {
					other, e := w.Literal()
					if e != nil {
						return false, e
					}
					counterpart, e := v.watches.Get(other.Negated().index())
					if e != nil {
						return false, e
					}
					ref, e := counterpart.ref()
					if e != nil {
						return false, e
					}
					learned, e := w.Learned()
					if e != nil {
						return false, e
					}
					expected, e := newSATBinaryWatch(literal, learned)
					if e != nil {
						return false, e
					}
					yes, e := v.watches.store.Contains(ref, expected)
					if e != nil {
						return false, e
					}
					if !yes {
						gone, e := v.wasEliminated(literal.variable())
						if e != nil {
							return false, e
						}
						otherGone, e := v.wasEliminated(other.variable())
						if e != nil {
							return false, e
						}
						c.traceMissingBinaryWatch(literal, other, learned, gone, otherGone, list, counterpart)
					}
				}
				enabled, e = diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
				if e != nil {
					return false, e
				}
				if enabled {
					other, e := w.Literal()
					if e != nil {
						return false, e
					}
					counterpart, e := v.watches.Get(other.Negated().index())
					if e != nil {
						return false, e
					}
					ref, e := counterpart.ref()
					if e != nil {
						return false, e
					}
					learned, e := w.Learned()
					if e != nil {
						return false, e
					}
					expected, e := newSATBinaryWatch(literal, learned)
					if e != nil {
						return false, e
					}
					yes, e := v.watches.store.Contains(ref, expected)
					if e != nil {
						return false, e
					}
					if e = satIntegrityAssertion(yes, c.debugActions, 181, "s.get_wlist(~(w.get_literal())).contains(watched(l, w.is_learned()))"); e != nil {
						return false, e
					}
				}
			case 1:
				enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
				if e != nil {
					return false, e
				}
				if !enabled {
					continue
				}
				one, e := w.Literal1()
				if e != nil {
					return false, e
				}
				gone, e := v.wasEliminated(one.variable())
				if e != nil {
					return false, e
				}
				if e = satIntegrityAssertion(!gone, c.debugActions, 184, "!s.was_eliminated(w.get_literal1().var())"); e != nil {
					return false, e
				}
				enabled, e = diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
				if e != nil {
					return false, e
				}
				if !enabled {
					continue
				}
				two, e := w.Literal2()
				if e != nil {
					return false, e
				}
				gone, e = v.wasEliminated(two.variable())
				if e != nil {
					return false, e
				}
				if e = satIntegrityAssertion(!gone, c.debugActions, 185, "!s.was_eliminated(w.get_literal2().var())"); e != nil {
					return false, e
				}
				enabled, e = diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
				if e != nil {
					return false, e
				}
				if !enabled {
					continue
				}
				if e = satIntegrityAssertion(one.index() < two.index(), c.debugActions, 186, "w.get_literal1().index() < w.get_literal2().index()"); e != nil {
					return false, e
				}
			case 2:
				enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
				if e != nil {
					return false, e
				}
				if !enabled {
					continue
				}
				off, e := w.ClauseOffset()
				if e != nil {
					return false, e
				}
				cl, e := p.clauses.GetClause(off)
				if e != nil {
					return false, e
				}
				removed, e := p.clauses.Removed(cl)
				if e != nil {
					return false, e
				}
				if e = satIntegrityAssertion(!removed, c.debugActions, 189, "!s.m_cls_allocator.get_clause(w.get_clause_offset())->was_removed()"); e != nil {
					return false, e
				}
			}
		}
	}
	return true, nil
}
func (c satIntegrityChecker) checkReinitStack() (bool, error) {
	if e := c.open(); e != nil {
		return false, e
	}
	for _, w := range c.lifecycle.reinit {
		if w.Binary() {
			continue
		}
		if !satStorageSourceDebug {
			continue
		}
		enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
		if e != nil {
			return false, e
		}
		if enabled {
			cl, e := w.ClauseRef()
			if e != nil {
				return false, e
			}
			yes, e := c.lifecycle.propagation.clauses.OnReinitStack(cl)
			if e != nil {
				return false, e
			}
			if e = satIntegrityAssertion(yes, c.debugActions, 205, "it->get_clause()->on_reinit_stack()"); e != nil {
				return false, e
			}
		}
	}
	return true, nil
}
func (c satIntegrityChecker) checkDisjointClauses() (yes bool, err error) {
	if e := c.open(); e != nil {
		return false, e
	}
	// Fresh source uint_set every invocation; use its existing real storage and
	// algorithms. Its cold Go owner allocation is an explicitly charged adapter.
	ids, e := newSATSolverUintSet(c.lifecycle.budget)
	if e != nil {
		return false, e
	}
	defer func() {
		if e := ids.Close(); e != nil {
			err = errors.Join(err, e)
		}
	}()
	for _, cl := range c.lifecycle.Original {
		id, e := c.lifecycle.propagation.clauses.ID(cl)
		if e != nil {
			return false, e
		}
		if e = ids.Insert(id); e != nil {
			return false, e
		}
	}
	for _, cl := range c.lifecycle.learned {
		id, e := c.lifecycle.propagation.clauses.ID(cl)
		if e != nil {
			return false, e
		}
		if ids.Contains(id) {
			c.traceRepeatedClause(id)
			return false, nil
		}
	}
	return true, nil
}
func (c satIntegrityChecker) run() (bool, error) {
	if e := c.open(); e != nil {
		return false, e
	}
	if c.lifecycle.propagation.inconsistent {
		return true, nil
	}
	if satStorageSourceDebug {
		// Each SASSERT expansion reads the process switch at its own site.
		// A debugger action can change that switch before the next site.
		for line := int32(231); line <= 236; line++ {
			enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
			if e != nil {
				return false, e
			}
			if !enabled {
				continue
			}
			var yes bool
			var condition string
			switch line {
			case 231:
				condition = "check_clauses()"
				yes, e = c.checkClauses()
			case 232:
				condition = "check_learned_clauses()"
				yes, e = c.checkLearnedClauses()
			case 233:
				condition = "check_watches()"
				yes, e = c.checkWatches()
			case 234:
				condition = "check_bool_vars()"
				yes, e = c.checkBoolVars()
			case 235:
				condition = "check_reinit_stack()"
				yes, e = c.checkReinitStack()
			case 236:
				condition = "check_disjoint_clauses()"
				yes, e = c.checkDisjointClauses()
			}
			if e != nil {
				return false, e
			}
			if e = satIntegrityAssertion(yes, c.debugActions, line, condition); e != nil {
				return false, e
			}
		}
	}
	return true, nil
}
