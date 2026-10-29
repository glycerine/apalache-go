package nlsat

// Source: src/nlsat/nlsat_solver.cpp:727-1001,1034-1054,1165-1187.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrSearchOwner = errors.New("nlsat: invalid search-state owner")
var ErrSearchPending = errors.New("nlsat: pending search-state operation")
var ErrSearchLive = errors.New("nlsat: live search-state borrower")

type trailKind uint8

const (
	trailBoolean trailKind = iota
	trailInfeasible
	trailLevel
	trailStage
	trailEquality
)

type searchTrail struct {
	kind        trailKind
	b           sat.BoolVar
	oldSet      *IntervalSet
	oldEquality ArithmeticAtom
	// Native owning continuation after a consumed source mutation.
	phase       uint8
	retired     *IntervalSet
	transferred bool
}
type undoKind uint8

const (
	undoSize undoKind = iota
	undoStage
	undoLevel
	undoUnassigned
	undoEmpty
)

// The five retained source predicate instantiations, without an arbitrary
// predicate callback or a universal template adapter.
type undoTarget struct {
	kind  undoKind
	value uint32
}
type searchConfig struct {
	Limit         *control.Reslimit
	MaxMemory     uint64
	Lazy          uint32
	SimplifyCores bool
	Randomize     bool
	MaxConflicts  uint32
}

// searchState is the actual trail/propagation state, not a completed Solver.
// Explanation, conflict analysis, search and the public solver remain separate.
type searchState struct {
	self                            *searchState
	store                           *clauseStore
	evaluator                       *Evaluator
	assignment                      *Assignment
	ism                             *IntervalSetManager
	config                          searchConfig
	bk                              sat.BoolVar
	xk                              uint32
	level                           uint32
	decisions, propagations, stages uint32
	conflicts                       uint32
	irrationalAssignments           uint32
	witness                         polynomial.Algebraic
	witnessActive                   bool
	trail                           []searchTrail
	undoing, undoEntryActive        bool
	target                          undoTarget
	setLocal                        evaluatorIntervalRef
	localRetired                    *IntervalSet
	union                           intervalUnionReceipt
	pendingLazy                     Justification
	justificationScope              *intervalJustificationScope
	propRefs                        [3]evaluatorIntervalRef
	propRetired                     []*IntervalSet
	core                            []sat.Literal
	minimizer                       *explanationMinimizer
	projection                      *projectionWorklist
	explanationAssumptions          *explanationAssumptions
	header                          uint64
	running, failed, closed         bool
}

func newSearchState(s *clauseStore, e *Evaluator, config searchConfig) (*searchState, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if err := e.open(); err != nil {
		return nil, err
	}
	if config.Limit == nil || config.Limit.Closed() || s.search != nil || e.search != nil || e.assignment.search != nil || e.ism.search != nil || e.pm != s.atoms.ineq.pm || e.ism.allocator != s.atoms.ineq.allocator {
		return nil, arena.ErrConfig
	}
	if config.MaxConflicts == 0 {
		config.MaxConflicts = ^uint32(0)
	}
	h := uint64(reflect.TypeOf(searchState{}).Size())
	if err := s.atoms.ineq.budget.Reserve(h); err != nil {
		return nil, err
	}
	r := &searchState{store: s, evaluator: e, assignment: e.assignment, ism: e.ism, config: config, bk: 0, xk: NullVariable, header: h}
	r.self = r
	r.setLocal.manager = r.ism
	for i := range r.propRefs {
		r.propRefs[i].manager = r.ism
	}
	s.search, e.search, e.assignment.search, e.ism.search = r, r, r, r
	return r, nil
}
func (r *searchState) owner() error {
	if r == nil || r.self != r {
		return ErrSearchOwner
	}
	if r.closed {
		return arena.ErrClosed
	}
	if r.store.search != r || r.evaluator.search != r || r.assignment.search != r || r.ism.search != r {
		return ErrSearchOwner
	}
	return nil
}
func (r *searchState) open() error {
	if err := r.owner(); err != nil {
		return err
	}
	if r.running || r.failed || r.undoing {
		return ErrSearchPending
	}
	if r.minimizer != nil && (r.minimizer.running || r.minimizer.failed || r.minimizer.resetting) {
		return ErrSearchPending
	}
	if r.projection != nil && (r.projection.running || r.projection.failed || r.projection.set == nil) {
		return ErrSearchPending
	}
	if r.explanationAssumptions != nil && (r.explanationAssumptions.running || r.explanationAssumptions.failed || r.explanationAssumptions.factors == nil || (r.explanationAssumptions.projectionTodo != nil && r.explanationAssumptions.pscTmp == nil)) {
		return ErrSearchPending
	}
	if err := r.store.open(); err != nil {
		return err
	}
	if err := r.evaluator.open(); err != nil {
		return err
	}
	if r.config.Limit.Closed() {
		return arena.ErrClosed
	}
	return r.ism.open()
}
func (r *searchState) assignedValue(l sat.Literal) (lbool.LBool, error) {
	b := l.Variable()
	if uint64(b) >= uint64(len(r.store.atoms.bools.values)) {
		return lbool.Undef, arena.ErrBounds
	}
	v := r.store.atoms.bools.values[b]
	if l.Sign() {
		v = v.Negated()
	}
	return v, nil
}
func (r *searchState) valueCore(l sat.Literal) (lbool.LBool, error) {
	v, err := r.assignedValue(l)
	if err != nil || v != lbool.Undef {
		return v, err
	}
	a, err := r.store.literalAtom(l)
	if err != nil || a == nil {
		return lbool.Undef, err
	}
	assigned, err := r.assignment.Contains(arithmeticAtomHeader(a).maxVar)
	if err != nil || !assigned {
		return lbool.Undef, err
	}
	value, err := r.evaluator.Eval(a, l.Sign())
	return lbool.FromBool(value), err
}
func (r *searchState) value(l sat.Literal) (result lbool.LBool, err error) {
	if err := r.open(); err != nil {
		return lbool.Undef, err
	}
	r.running = true
	defer func() { r.running = false; r.failed = err != nil }()
	return r.valueCore(l)
}
func (r *searchState) newLevelCore() error {
	if err := r.evaluator.Push(); err != nil {
		return err
	}
	r.level++
	r.trail = append(r.trail, searchTrail{kind: trailLevel})
	return nil
}
func (r *searchState) newLevel() (err error) {
	if err := r.open(); err != nil {
		return err
	}
	r.running = true
	defer func() { r.running = false; r.failed = err != nil }()
	return r.newLevelCore()
}
func (r *searchState) newStage() error {
	if err := r.open(); err != nil {
		return err
	}
	r.stages++
	r.trail = append(r.trail, searchTrail{kind: trailStage})
	if r.xk == NullVariable {
		r.xk = 0
	} else {
		r.xk++
	}
	return nil
}
func (r *searchState) assignCore(l sat.Literal, j Justification) error {
	v, err := r.assignedValue(l)
	if err != nil {
		return err
	}
	if err := atomAssert("nlsat_solver.cpp", 900, "assigned_value(l) == l_undef", v == lbool.Undef); err != nil {
		return err
	}
	if err := atomAssert("nlsat_solver.cpp", 901, "j != null_justification", !j.IsNull()); err != nil {
		return err
	}
	if err := atomAssert("nlsat_solver.cpp", 902, "!j.is_null()", !j.IsNull()); err != nil {
		return err
	}
	if v != lbool.Undef || j.IsNull() {
		return arena.ErrConfig
	}
	if j.IsLazy() {
		if j.lazy == nil || j.lazy.allocator != r.store.atoms.ineq.allocator {
			return ErrSearchOwner
		}
		if err := j.lazy.open(); err != nil {
			return err
		}
	}
	if j.IsDecision() {
		r.decisions++
	} else {
		r.propagations++
	}
	b := l.Variable()
	r.store.atoms.bools.values[b] = lbool.FromBool(!l.Sign())
	r.store.atoms.bools.levels[b] = r.level
	r.store.atoms.bools.justifications[b] = j
	if r.pendingLazy == j {
		r.pendingLazy = Justification{}
	}
	r.trail = append(r.trail, searchTrail{kind: trailBoolean, b: b})
	return r.updateEquality(b)
}
func (r *searchState) assign(l sat.Literal, j Justification) (err error) {
	if err := r.open(); err != nil {
		return err
	}
	r.running = true
	defer func() { r.running = false; r.failed = err != nil }()
	return r.assignCore(l, j)
}
func (r *searchState) decideCore(l sat.Literal) error {
	if err := r.newLevelCore(); err != nil {
		return err
	}
	j, err := NewDecisionJustification()
	if err != nil {
		return err
	}
	return r.assignCore(l, j)
}
func (r *searchState) decide(l sat.Literal) (err error) {
	if err := r.open(); err != nil {
		return err
	}
	r.running = true
	defer func() { r.running = false; r.failed = err != nil }()
	return r.decideCore(l)
}
func (r *searchState) updateEquality(b sat.BoolVar) error {
	if !r.config.SimplifyCores || r.store.atoms.bools.values[b] != lbool.True {
		return nil
	}
	a := r.store.atoms.bools.atoms[b]
	ineq, ok := a.(*InequalityAtom)
	if !ok || ineq.kind != AtomEQ || len(ineq.factors) > 1 || ineq.factors[0].Even {
		return nil
	}
	x := r.xk
	if err := atomAssert("nlsat_solver.cpp", 1043, "a->max_var() == x", ineq.maxVar == x); err != nil {
		return err
	}
	if err := atomAssert("nlsat_solver.cpp", 1044, "x != null_var", x != NullVariable); err != nil {
		return err
	}
	if uint64(x) >= uint64(len(r.store.vars.equalities)) {
		return arena.ErrBounds
	}
	if old := r.store.vars.equalities[x]; old != nil {
		d1, err := r.store.atomDegree(old)
		if err != nil {
			return err
		}
		d2, err := r.store.atomDegree(a)
		if err != nil {
			return err
		}
		if d1 <= d2 {
			return nil
		}
	}
	r.trail = append(r.trail, searchTrail{kind: trailEquality, oldEquality: r.store.vars.equalities[x]})
	r.store.vars.equalities[x] = a
	return nil
}

func (r *searchState) undoNeeded() (bool, error) {
	switch r.target.kind {
	case undoSize:
		return uint64(len(r.trail)) > uint64(r.target.value), nil
	case undoStage:
		return r.xk != r.target.value, nil
	case undoLevel:
		return r.level > r.target.value, nil
	case undoUnassigned:
		b := sat.BoolVar(r.target.value)
		if uint64(b) >= uint64(len(r.store.atoms.bools.values)) {
			return false, arena.ErrBounds
		}
		return r.store.atoms.bools.values[b] != lbool.Undef, nil
	case undoEmpty:
		return true, nil
	}
	return false, arena.ErrConfig
}
func (r *searchState) undoEntry(t *searchTrail) error {
	switch t.kind {
	case trailBoolean:
		b := t.b
		if t.phase == 0 {
			r.store.atoms.bools.values[b] = lbool.Undef
			r.store.atoms.bools.levels[b] = ^uint32(0)
			t.phase = 1
		}
		if err := DeleteJustification(r.store.atoms.ineq.allocator, r.store.atoms.bools.justifications[b]); err != nil {
			return err
		}
		r.store.atoms.bools.justifications[b] = Justification{}
		if r.store.atoms.bools.atoms[b] == nil && b < r.bk {
			r.bk = b
		}
	case trailInfeasible:
		if r.xk == NullVariable || uint64(r.xk) >= uint64(len(r.store.vars.infeasible)) {
			return nil
		}
		if t.phase == 0 && t.transferred {
			current := r.store.vars.infeasible[r.xk]
			err := r.ism.DecRef(current)
			if err != nil {
				if current != nil && (current.deleting || current.closed) {
					t.retired = current
					t.phase = 1
				}
				return err
			}
			t.phase = 1
		}
		if t.retired != nil {
			if err := r.ism.cleanupRetiredSet(t.retired); err != nil {
				return err
			}
			t.retired = nil
		}
		r.store.vars.infeasible[r.xk] = t.oldSet
	case trailStage:
		if t.phase == 0 {
			if r.xk == 0 {
				r.xk = NullVariable
				return nil
			}
			if r.xk == NullVariable {
				return nil
			}
			r.xk--
			t.phase = 1
		}
		return r.assignment.ResetVar(r.xk)
	case trailLevel:
		if t.phase == 0 {
			if err := atomAssert("nlsat_solver.cpp", 776, "m_scope_lvl > 0", r.level > 0); err != nil {
				return err
			}
			r.level--
			t.phase = 1
		}
		return r.evaluator.Pop(1)
	case trailEquality:
		if uint64(r.xk) < uint64(len(r.store.vars.equalities)) {
			r.store.vars.equalities[r.xk] = t.oldEquality
		}
	}
	return nil
}
func (r *searchState) drainUndo() error {
	for len(r.trail) > 0 {
		needed, err := r.undoNeeded()
		if err != nil {
			return err
		}
		if !r.undoEntryActive && !needed {
			break
		}
		r.undoEntryActive = true
		i := len(r.trail) - 1
		if err := r.undoEntry(&r.trail[i]); err != nil {
			return err
		}
		r.trail[i] = searchTrail{}
		r.trail = r.trail[:i]
		r.undoEntryActive = false
	}
	if r.target.kind == undoUnassigned {
		b := sat.BoolVar(r.target.value)
		if err := atomAssert("nlsat_solver.cpp", 862, "m_bvalues[b] == l_undef", r.store.atoms.bools.values[b] == lbool.Undef); err != nil {
			return err
		}
	}
	r.undoing = false
	return nil
}
func (r *searchState) undo(target undoTarget) (err error) {
	if err := r.open(); err != nil {
		return err
	}
	if target.kind == undoUnassigned && uint64(target.value) >= uint64(len(r.store.atoms.bools.values)) {
		return arena.ErrBounds
	}
	if target.kind == undoSize {
		if err := atomAssert("nlsat_solver.cpp", 822, "m_trail.size() >= old_size", uint64(len(r.trail)) >= uint64(target.value)); err != nil {
			return err
		}
	}
	r.running = true
	defer func() { r.running = false; r.failed = err != nil }()
	r.target = target
	r.undoing = true
	return r.drainUndo()
}
func (r *searchState) undoUntilSize(size uint32) error   { return r.undo(undoTarget{undoSize, size}) }
func (r *searchState) undoUntilStage(stage uint32) error { return r.undo(undoTarget{undoStage, stage}) }
func (r *searchState) undoUntilLevel(level uint32) error { return r.undo(undoTarget{undoLevel, level}) }
func (r *searchState) undoUntilUnassigned(b sat.BoolVar) error {
	return r.undo(undoTarget{undoUnassigned, uint32(b)})
}
func (r *searchState) undoUntilEmpty() error { return r.undo(undoTarget{undoEmpty, 0}) }

func (r *searchState) peekNextBooleanVariable() error {
	if err := r.open(); err != nil {
		return err
	}
	v := &r.store.atoms.bools
	for uint64(r.bk) < uint64(len(v.atoms)) {
		if !v.dead[r.bk] && v.atoms[r.bk] == nil && v.values[r.bk] == lbool.Undef {
			return nil
		}
		r.bk++
	}
	r.bk = sat.NullBoolVar
	return nil
}

func (r *searchState) cleanup() error {
	if err := r.owner(); err != nil {
		return err
	}
	if r.running {
		return ErrSearchPending
	}
	r.running = true
	defer func() { r.running = false }()
	if r.explanationAssumptions != nil {
		if err := r.explanationAssumptions.cleanup(); err != nil {
			return err
		}
	}
	if r.projection != nil {
		if err := r.projection.cleanup(); err != nil {
			return err
		}
	}
	if r.minimizer != nil {
		if err := r.minimizer.cleanup(); err != nil {
			return err
		}
	}
	if r.witnessActive {
		// Only this invocation's lexical scoped_anum is owned here. Numerical
		// callees retain responsibility for their own interrupted operations.
		if err := r.evaluator.am.CleanupValue(&r.witness); err != nil {
			return err
		}
		r.witnessActive = false
	}
	if err := r.ism.cleanupJustifications(r.justificationScope); err != nil {
		return err
	}
	r.justificationScope = nil
	if err := r.ism.cleanupUnion(&r.union); err != nil {
		return err
	}
	if r.localRetired != nil {
		if err := r.ism.cleanupRetiredSet(r.localRetired); err != nil {
			return err
		}
		r.localRetired = nil
	}
	value := r.setLocal.value
	if err := r.setLocal.reset(); err != nil {
		if value != nil && (value.deleting || value.closed) {
			r.localRetired = value
		}
		return err
	}
	if !r.pendingLazy.IsNull() {
		if err := DeleteJustification(r.store.atoms.ineq.allocator, r.pendingLazy); err != nil {
			return err
		}
		r.pendingLazy = Justification{}
	}
	for len(r.propRetired) > 0 {
		i := len(r.propRetired) - 1
		if err := r.ism.cleanupRetiredSet(r.propRetired[i]); err != nil {
			return err
		}
		r.propRetired = r.propRetired[:i]
	}
	if err := r.closePropagationLocals(); err != nil {
		return err
	}
	if r.undoing {
		if err := r.drainUndo(); err != nil {
			return err
		}
	}
	r.failed = false
	return nil
}
func (r *searchState) close() error {
	if r != nil && r.self == r && r.closed {
		return nil
	}
	if err := r.cleanup(); err != nil {
		return err
	}
	r.running = true
	defer func() { r.running = false }()
	if r.explanationAssumptions != nil {
		if err := r.explanationAssumptions.close(); err != nil {
			return err
		}
	}
	if r.projection != nil {
		if err := r.projection.close(); err != nil {
			return err
		}
	}
	if r.minimizer != nil {
		if err := r.minimizer.close(); err != nil {
			return err
		}
	}
	r.target = undoTarget{undoEmpty, 0}
	r.undoing = true
	if err := r.drainUndo(); err != nil {
		return err
	}
	r.store.search = nil
	r.evaluator.search = nil
	r.assignment.search = nil
	r.ism.search = nil
	r.store.atoms.ineq.budget.Release(r.header)
	r.header = 0
	r.trail = nil
	r.closed = true
	return nil
}
