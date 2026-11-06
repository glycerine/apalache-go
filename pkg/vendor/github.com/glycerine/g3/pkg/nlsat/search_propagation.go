package nlsat

// Source: src/nlsat/nlsat_solver.cpp:234-237,951-1160.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (r *searchState) checkpoint() error {
	if r.config.Limit.Closed() {
		return arena.ErrClosed
	}
	if !r.config.Limit.Inc() {
		return z3exception.NewDefaultException(r.config.Limit.CancelMessage())
	}
	size, err := r.store.atoms.ineq.allocator.RuntimeOwner().AllocationSize()
	if err != nil {
		return err
	}
	if size > r.config.MaxMemory {
		return z3exception.NewDefaultException(messages.MaxMemory)
	}
	return nil
}
func (r *searchState) isSatisfied(c *Clause) (bool, error) {
	for _, l := range c.Literals() {
		v, err := r.valueCore(l)
		if err != nil {
			return false, err
		}
		if v == lbool.True {
			return true, nil
		}
	}
	return false, nil
}
func (r *searchState) isInconsistent(lits []sat.Literal) (bool, error) {
	for _, l := range lits {
		v, err := r.valueCore(l)
		if err != nil {
			return false, err
		}
		if v != lbool.False {
			return false, nil
		}
	}
	return true, nil
}
func (r *searchState) processBooleanClause(c *Clause) (bool, error) {
	if err := atomAssert("nlsat_solver.cpp", 978, "m_xk == null_var", r.xk == NullVariable); err != nil {
		return false, err
	}
	count := uint32(0)
	first := ^uint32(0)
	for i, l := range c.Literals() {
		a, err := r.store.literalAtom(l)
		if err != nil {
			return false, err
		}
		if err := atomAssert("nlsat_solver.cpp", 984, "m_atoms[l.var()] == 0", a == nil); err != nil {
			return false, err
		}
		v, err := r.valueCore(l)
		if err != nil {
			return false, err
		}
		if err := atomAssert("nlsat_solver.cpp", 985, "value(l) != l_true", v != lbool.True); err != nil {
			return false, err
		}
		if v == lbool.False {
			continue
		}
		if err := atomAssert("nlsat_solver.cpp", 988, "value(l) == l_undef", v == lbool.Undef); err != nil {
			return false, err
		}
		count++
		if first == ^uint32(0) {
			first = uint32(i)
		}
	}
	if count == 0 {
		return false, nil
	}
	if err := atomAssert("nlsat_solver.cpp", 995, "first_undef != UINT_MAX", first != ^uint32(0)); err != nil {
		return false, err
	}
	if count == 1 {
		j, err := NewClauseJustification(c)
		if err != nil {
			return false, err
		}
		return true, r.assignCore(c.literals[first], j)
	}
	return true, r.decideCore(c.literals[first])
}

func (r *searchState) unionInto(dst *evaluatorIntervalRef, a, b *IntervalSet) error {
	s, err := r.ism.mkUnionWithReceipt(a, b, &r.union)
	if err != nil {
		return err
	}
	if err := dst.assign(s); err != nil {
		return err
	}
	r.union = intervalUnionReceipt{}
	return nil
}
func (r *searchState) dropSetLocal() error {
	s := r.setLocal.value
	if err := r.setLocal.reset(); err != nil {
		if s != nil && (s.deleting || s.closed) {
			r.localRetired = s
		}
		return err
	}
	return nil
}
func (r *searchState) updateInfeasibleCore(s *IntervalSet) error {
	if err := atomAssert("nlsat_solver.cpp", 1019, "m_xk != null_var", r.xk != NullVariable); err != nil {
		return err
	}
	if uint64(r.xk) >= uint64(len(r.store.vars.infeasible)) {
		return arena.ErrBounds
	}
	old := r.store.vars.infeasible[r.xk]
	index := len(r.trail)
	r.trail = append(r.trail, searchTrail{kind: trailInfeasible, oldSet: old})
	if err := r.unionInto(&r.setLocal, s, old); err != nil {
		return err
	}
	full, err := r.ism.IsFull(r.setLocal.value)
	if err != nil {
		return err
	}
	if err := atomAssert("nlsat_solver.cpp", 1026, "!m_ism.is_full(new_set)", !full); err != nil {
		return err
	}
	if err := r.ism.IncRef(r.setLocal.value); err != nil {
		return err
	}
	r.store.vars.infeasible[r.xk] = r.setLocal.value
	r.trail[index].transferred = true
	return r.dropSetLocal()
}
func (r *searchState) updateInfeasible(s *IntervalSet) (err error) {
	if err := r.open(); err != nil {
		return err
	}
	r.running = true
	defer func() { r.running = false; r.failed = err != nil }()
	return r.updateInfeasibleCore(s)
}
func (r *searchState) rPropagate(l sat.Literal, s *IntervalSet, include bool) error {
	if err := r.ism.getJustificationsWithReceipt(s, &r.core, &r.justificationScope); err != nil {
		return err
	}
	r.justificationScope = nil
	if include {
		r.core = append(r.core, l.Negated())
	}
	j, err := NewLazyJustification(r.store.atoms.ineq.allocator, r.core)
	r.pendingLazy = j
	if err != nil {
		return err
	}
	if err := r.assignCore(l, j); err != nil {
		return err
	}
	v, err := r.valueCore(l)
	if err != nil {
		return err
	}
	return atomAssert("nlsat_solver.cpp", 1012, "value(l) == l_true", v == lbool.True)
}
func (r *searchState) dropPropagationRef(i int) error {
	s := r.propRefs[i].value
	if err := r.propRefs[i].reset(); err != nil {
		if s != nil && (s.deleting || s.closed) {
			r.propRetired = append(r.propRetired, s)
		}
		return err
	}
	return nil
}
func (r *searchState) closePropagationLocals() error {
	for i := len(r.propRefs) - 1; i >= 0; i-- {
		if err := r.dropPropagationRef(i); err != nil {
			return err
		}
	}
	return nil
}
func (r *searchState) processArithmeticClause(c *Clause, satisfyLearned bool) (result bool, err error) {
	if !satisfyLearned && r.config.Lazy >= 2 && c.learned {
		return true, nil
	}
	max, err := r.store.maxVar(c.Literals())
	if err != nil {
		return false, err
	}
	if err := atomAssert("nlsat_solver.cpp", 1063, "m_xk == max_var(cls)", r.xk == max); err != nil {
		return false, err
	}
	if uint64(r.xk) >= uint64(len(r.store.vars.infeasible)) {
		return false, arena.ErrBounds
	}
	defer func() { err = errors.Join(err, r.closePropagationLocals()) }()
	count := uint32(0)
	first := ^uint32(0)
	xset := r.store.vars.infeasible[r.xk]
	full, err := r.ism.IsFull(xset)
	if err != nil {
		return false, err
	}
	if err := atomAssert("nlsat_solver.cpp", 1068, "!m_ism.is_full(xk_set)", !full); err != nil {
		return false, err
	}
	for i, l := range c.Literals() {
		if err := r.checkpoint(); err != nil {
			return false, err
		}
		v, err := r.valueCore(l)
		if err != nil {
			return false, err
		}
		if v == lbool.False {
			continue
		}
		if err := atomAssert("nlsat_solver.cpp", 1074, "value(l) == l_undef", v == lbool.Undef); err != nil {
			return false, err
		}
		a, err := r.store.literalAtom(l)
		if err != nil {
			return false, err
		}
		h := arithmeticAtomHeader(a)
		if h == nil {
			return false, arena.ErrConfig
		}
		if err := atomAssert("nlsat_solver.cpp", 1075, "max_var(l) == m_xk", h.maxVar == r.xk); err != nil {
			return false, err
		}
		if err := atomAssert("nlsat_solver.cpp", 1078, "a != 0", a != nil); err != nil {
			return false, err
		}
		set, err := r.evaluator.InfeasibleIntervals(a, l.Sign())
		if err != nil {
			return false, err
		}
		// The actual Go evaluator returns one owned reference; transfer it
		// into this concrete local, rather than inventing an extra INC.
		r.propRefs[1].value = set
		empty, err := r.ism.IsEmpty(set)
		if err != nil {
			return false, err
		}
		if empty {
			if err := r.rPropagate(l, nil, true); err != nil {
				return false, err
			}
			sat, err := r.isSatisfied(c)
			if err != nil {
				return false, err
			}
			if err := atomAssert("nlsat_solver.cpp", 1086, "is_satisfied(cls)", sat); err != nil {
				return false, err
			}
			return true, nil
		}
		full, err := r.ism.IsFull(set)
		if err != nil {
			return false, err
		}
		if full {
			if err := r.rPropagate(l.Negated(), nil, true); err != nil {
				return false, err
			}
			if err := r.dropPropagationRef(1); err != nil {
				return false, err
			}
			continue
		}
		subset, err := r.ism.Subset(set, xset)
		if err != nil {
			return false, err
		}
		if subset {
			if err := r.rPropagate(l, xset, true); err != nil {
				return false, err
			}
			return true, nil
		}
		if err := r.unionInto(&r.propRefs[2], set, xset); err != nil {
			return false, err
		}
		full, err = r.ism.IsFull(r.propRefs[2].value)
		if err != nil {
			return false, err
		}
		if full {
			if err := r.rPropagate(l.Negated(), r.propRefs[2].value, false); err != nil {
				return false, err
			}
			if err := r.dropPropagationRef(2); err != nil {
				return false, err
			}
			if err := r.dropPropagationRef(1); err != nil {
				return false, err
			}
			continue
		}
		count++
		if first == ^uint32(0) {
			first = uint32(i)
			if err := r.propRefs[0].assign(set); err != nil {
				return false, err
			}
		}
		if err := r.dropPropagationRef(2); err != nil {
			return false, err
		}
		if err := r.dropPropagationRef(1); err != nil {
			return false, err
		}
	}
	if count == 0 {
		return false, nil
	}
	if err := atomAssert("nlsat_solver.cpp", 1115, "first_undef != UINT_MAX", first != ^uint32(0)); err != nil {
		return false, err
	}
	if count == 1 {
		j, err := NewClauseJustification(c)
		if err != nil {
			return false, err
		}
		if err := r.assignCore(c.literals[first], j); err != nil {
			return false, err
		}
		return true, r.updateInfeasibleCore(r.propRefs[0].value)
	}
	if satisfyLearned || !c.learned || r.config.Lazy == 0 {
		if err := r.decideCore(c.literals[first]); err != nil {
			return false, err
		}
		if err := r.updateInfeasibleCore(r.propRefs[0].value); err != nil {
			return false, err
		}
	}
	return true, nil
}
func (r *searchState) processClauseCore(c *Clause, satisfyLearned bool) (bool, error) {
	satisfied, err := r.isSatisfied(c)
	if err != nil || satisfied {
		return satisfied, err
	}
	if r.xk == NullVariable {
		return r.processBooleanClause(c)
	}
	return r.processArithmeticClause(c, satisfyLearned)
}
func (r *searchState) processClause(c *Clause, satisfyLearned bool) (result bool, err error) {
	if err := r.open(); err != nil {
		return false, err
	}
	if r.store.owned[c] == nil {
		return false, ErrSearchOwner
	}
	r.running = true
	defer func() { r.running = false; r.failed = err != nil }()
	return r.processClauseCore(c, satisfyLearned)
}
func (r *searchState) processClauses(clauses []*Clause) (conflict *Clause, err error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	r.running = true
	defer func() { r.running = false; r.failed = err != nil }()
	for _, c := range clauses {
		if r.store.owned[c] == nil {
			return nil, ErrSearchOwner
		}
		ok, err := r.processClauseCore(c, false)
		if err != nil {
			return nil, err
		}
		if !ok {
			return c, nil
		}
	}
	return nil, nil
}
