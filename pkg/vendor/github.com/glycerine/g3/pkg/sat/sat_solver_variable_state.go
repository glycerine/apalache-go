package sat

// Actual solver member owners and complete sat_solver.cpp:135-160 mk_var body.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// This source-member grouping is not a complete solver constructor. It supplies
// actual columns/queue/trail directly to that owner. Elimination todo is the
// simplifier's real borrowed set. No callback supplies missing solver behavior.
type satSolverVariableState struct {
	self                                            *satSolverVariableState
	budget                                          *arena.Budget
	watches                                         *satSolverWatchLists
	assignment                                      []lbool.LBool
	memory                                          *umemory.Runtime
	phase, previousPhase                            []int8
	decision, eliminated, external, assignedSinceGC []bool
	mark, literalMark                               []bool
	justification                                   satSolverJustifications
	level                                           []uint32
	activity                                        *satSolverActivity
	queue                                           *satVarQueue
	trail                                           *satSolverTrail
	trailRef                                        satSolverLiteralListRef
	elimTodo                                        *satSolverUintSet
	ownerBytes                                      uint64
	mkVarCount, qhead, scopeLevel, activityInc      uint32
	modelIsCurrent, modelKnown, closed              bool
}
type satSolverVariableStats struct {
	OwnerBytes, ColumnBytes, WatchBytes, ActivityBytes, QueueBytes, TrailBytes, TotalBytes uint64
	NumVars, MkVarCount                                                                    uint32
}

// Cold preparation provides actual source member borrows while later members
// are still unconstructed. Full solver ordinary operations remain guarded by
// its constructor completion; the standalone adapter below retains its order.
func newSATColdVariableState(todo *satSolverUintSet, b *arena.Budget, c arena.VectorConfig, raw *umemory.Runtime) (*satSolverVariableState, error) {
	if e := todo.open(); e != nil {
		return nil, e
	}
	if c.Descriptors.Budget != b {
		return nil, arena.ErrConfig
	}
	if raw == nil {
		return nil, ErrSATOwner
	}
	if _, e := raw.AllocationSize(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(satSolverVariableState{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &satSolverVariableState{budget: b, elimTodo: todo, ownerBytes: n, activityInc: 128}
	s.self = s
	s.memory = raw
	s.justification.bind()
	return s, nil
}

func newSATSolverVariableState(todo *satSolverUintSet, b *arena.Budget, c arena.VectorConfig, raw *umemory.Runtime) (*satSolverVariableState, error) {
	s, e := newSATColdVariableState(todo, b, c, raw)
	if e != nil {
		return nil, e
	}
	s.watches, e = newSATSolverWatchLists(b)
	if e == nil {
		s.activity, e = newSATSolverActivity(b)
	}
	if e == nil {
		s.queue, e = newSATSolverVarQueue(s.activity, b)
	}
	if e == nil {
		s.trail, s.trailRef, e = newSATSolverTrail(b)
	}
	if e != nil {
		if cleanup := s.Close(); cleanup != nil {
			return s, cleanup
		}
		return nil, e
	}
	return s, nil
}
func (s *satSolverVariableState) constructionBorrow() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *satSolverVariableState) open() error {
	if e := s.constructionBorrow(); e != nil {
		return e
	}
	if s.watches == nil || s.activity == nil || s.queue == nil || s.trail == nil {
		return arena.ErrClosed
	}
	return nil
}

func (s *satSolverVariableState) NumVars() uint32 {
	if s == nil || s.self != s || s.closed {
		return 0
	}
	return uint32(len(s.level))
}
func (s *satSolverVariableState) numVarsChecked() (uint32, error) {
	if e := s.constructionBorrow(); e != nil {
		return 0, e
	}
	return uint32(len(s.level)), nil
}
func (s *satSolverVariableState) mkVar(ext, dvar bool) (BoolVar, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	s.modelIsCurrent = false
	s.modelKnown = true
	s.mkVarCount++
	n, e := s.numVarsChecked()
	if e != nil {
		return 0, e
	}
	v := BoolVar(n)
	if e := s.watches.PushEmpty(); e != nil {
		return v, e
	}
	if e := s.watches.PushEmpty(); e != nil {
		return v, e
	}
	s.assignment = append(s.assignment, lbool.Undef, lbool.Undef)
	if e := s.justification.push(satJustification{}); e != nil {
		return v, e
	}
	s.decision = append(s.decision, dvar)
	s.eliminated = append(s.eliminated, false)
	s.external = append(s.external, ext)
	if e := s.activity.Push(0); e != nil {
		return v, e
	}
	s.level = append(s.level, ^uint32(0))
	s.mark = append(s.mark, false)
	s.literalMark = append(s.literalMark, false, false)
	s.phase = append(s.phase, 2)
	s.previousPhase = append(s.previousPhase, 2)
	s.assignedSinceGC = append(s.assignedSinceGC, false)
	if e := s.queue.MkVar(v); e != nil {
		return v, e
	}
	if e := s.elimTodo.Insert(uint32(v)); e != nil {
		return v, e
	}
	if satStorageSourceDebug {
		eliminated, e := s.wasEliminated(v)
		if e != nil {
			return v, e
		}
		if eliminated {
			return v, ErrSATAssertion
		}
	}
	return v, nil
}

// This view is an ephemeral borrow for a clause operation with no variable
// creation during that call. It never copies source assignment/level storage.
func (s *satSolverVariableState) clauseState() satSolverClauseState {
	return satSolverClauseState{variables: s}
}
func (s *satSolverVariableState) valueLiteral(l Literal) (lbool.LBool, error) {
	if e := s.open(); e != nil {
		return lbool.Undef, e
	}
	if uint64(l.index()) >= uint64(len(s.assignment)) {
		if satStorageSourceDebug {
			return lbool.Undef, ErrSATAssertion
		}
		return lbool.Undef, arena.ErrBounds
	}
	return s.assignment[l.index()], nil
}
func (s *satSolverVariableState) setAssignment(l Literal, value lbool.LBool) error {
	if _, e := s.valueLiteral(l); e != nil {
		return e
	}
	s.assignment[l.index()] = value
	return nil
}
func (s *satSolverVariableState) valueVar(v BoolVar) (lbool.LBool, error) {
	return s.valueLiteral(Literal(uint32(v) << 1))
}
func (s *satSolverVariableState) levelVar(v BoolVar) (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	if uint64(v) >= uint64(len(s.level)) {
		if satStorageSourceDebug {
			return 0, ErrSATAssertion
		}
		return 0, arena.ErrBounds
	}
	return s.level[v], nil
}
func (s *satSolverVariableState) setLevel(v BoolVar, value uint32) error {
	if _, e := s.levelVar(v); e != nil {
		return e
	}
	s.level[v] = value
	return nil
}
func (s *satSolverVariableState) IsExternal(v BoolVar) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if uint64(v) >= uint64(len(s.external)) {
		if satStorageSourceDebug {
			return false, ErrSATAssertion
		}
		return false, arena.ErrBounds
	}
	return s.external[v], nil
}
func (s *satSolverVariableState) wasEliminated(v BoolVar) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if uint64(v) >= uint64(len(s.eliminated)) {
		if satStorageSourceDebug {
			return false, ErrSATAssertion
		}
		return false, arena.ErrBounds
	}
	return s.eliminated[v], nil
}
func (s *satSolverVariableState) decisionEligible(v BoolVar) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if uint64(v) >= uint64(len(s.decision)) {
		if satStorageSourceDebug {
			return false, ErrSATAssertion
		}
		return false, arena.ErrBounds
	}
	return s.decision[v], nil
}
func (s *satSolverVariableState) gcAssigned(v BoolVar) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if uint64(v) >= uint64(len(s.assignedSinceGC)) {
		if satStorageSourceDebug {
			return false, ErrSATAssertion
		}
		return false, arena.ErrBounds
	}
	return s.assignedSinceGC[v], nil
}
func (s *satSolverVariableState) setExternal(v BoolVar, value bool) error {
	if _, e := s.IsExternal(v); e != nil {
		return e
	}
	s.external[v] = value
	return nil
}
func (s *satSolverVariableState) setEliminated(v BoolVar, value bool) error {
	if _, e := s.wasEliminated(v); e != nil {
		return e
	}
	s.eliminated[v] = value
	return nil
}
func (s *satSolverVariableState) setGCAssigned(v BoolVar, value bool) error {
	if _, e := s.gcAssigned(v); e != nil {
		return e
	}
	s.assignedSinceGC[v] = value
	return nil
}

// phaseAt returns the saved polarity: 0 or 1, with 2 meaning undefined.
func (s *satSolverVariableState) phaseAt(v BoolVar) (int8, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	if uint64(v) >= uint64(len(s.phase)) {
		if satStorageSourceDebug {
			return 0, ErrSATAssertion
		}
		return 0, arena.ErrBounds
	}
	return s.phase[v], nil
}
func (s *satSolverVariableState) setPhase(v BoolVar, value int8) error {
	if _, e := s.phaseAt(v); e != nil {
		return e
	}
	s.phase[v] = value
	return nil
}

// previousPhaseAt returns the saved polarity: 0 or 1, with 2 meaning undefined.
func (s *satSolverVariableState) previousPhaseAt(v BoolVar) (int8, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	if uint64(v) >= uint64(len(s.previousPhase)) {
		if satStorageSourceDebug {
			return 0, ErrSATAssertion
		}
		return 0, arena.ErrBounds
	}
	return s.previousPhase[v], nil
}
func (s *satSolverVariableState) setPreviousPhase(v BoolVar, value int8) error {
	if _, e := s.previousPhaseAt(v); e != nil {
		return e
	}
	s.previousPhase[v] = value
	return nil
}

func (s *satSolverVariableState) trailPush(l Literal) error {
	if e := s.open(); e != nil {
		return e
	}
	return s.trail.Push(s.trailRef, l)
}
func (s *satSolverVariableState) trailRead(i uint32) (Literal, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.trail.Read(s.trailRef, i)
}
func (s *satSolverVariableState) Stats() satSolverVariableStats {
	if s == nil || s.self != s || s.closed {
		return satSolverVariableStats{}
	}
	out := satSolverVariableStats{OwnerBytes: s.ownerBytes, NumVars: s.NumVars(), MkVarCount: s.mkVarCount}
	if s.watches != nil {
		out.WatchBytes = s.watches.Stats().TotalBytes
	}
	if s.activity != nil {
		v := s.activity.Stats()
		out.ActivityBytes = v.OwnerBytes + v.BackingBytes + v.BindingBytes
	}
	if s.queue != nil {
		v := s.queue.Stats()
		out.QueueBytes = v.OwnerBytes
	}
	if s.trail != nil {
		out.TrailBytes = s.trail.ownerBytes
	}
	out.TotalBytes = out.OwnerBytes + out.ColumnBytes + out.WatchBytes + out.ActivityBytes + out.QueueBytes + out.TrailBytes
	return out
}

// Independent graph walk: registry liveness alone is not evidence of ownership.
// Every active watch descriptor is named exactly once in the source outer vector;
// the only trail descriptor is the actual trail member token.
func (s *satSolverVariableState) checkPhysicalOwners() error {
	if e := s.open(); e != nil {
		return e
	}
	for i := range s.watches.store.records {
		r := &s.watches.store.records[i]
		if !r.live {
			continue
		}
		count := 0
		for _, owner := range s.watches.lists {
			if owner.owner == s.watches.store.origin && owner.slot == uint32(i) && owner.generation == r.generation {
				count++
			}
		}
		if count != 1 {
			return ErrSATOwner
		}
	}
	for _, r := range s.watches.lists {
		if !s.watches.store.IsLive(r) {
			return arena.ErrInvalidSlot
		}
	}
	if !s.trail.IsLive(s.trailRef) {
		return arena.ErrInvalidSlot
	}
	return nil
}
func (s *satSolverVariableState) Close() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return nil
	}
	// Selected member destruction follows reverse declaration order. No clauses
	// or borrowed elimination todo are owned here, and no AST/SAT callbacks run.
	if e := s.finalizeTrail(); e != nil {
		return e
	}
	if e := s.finalizeQueue(); e != nil {
		return e
	}
	if e := s.finalizeColumnsWatches(); e != nil {
		return e
	}
	s.budget.Release(s.ownerBytes)
	s.ownerBytes = 0
	s.closed = true
	return nil
}

// These concrete stages expose the actual reverse member actions for the full
// facade. Standalone Close keeps its previous order. Retry is an explicit Go
// owning acknowledgment, not a C++ destructor exception continuation.
func (s *satSolverVariableState) finalizeTrail() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.trail != nil {
		if e := s.trail.Close(); e != nil {
			return e
		}
		s.trail = nil
	}
	return nil
}
func (s *satSolverVariableState) finalizeQueue() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.queue != nil {
		if e := s.queue.Close(); e != nil {
			return e
		}
		s.queue = nil
	}
	return nil
}
func (s *satSolverVariableState) finalizeColumnsWatches() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	s.assignedSinceGC = nil
	s.previousPhase = nil
	s.phase = nil
	if s.activity != nil {
		if e := s.activity.Close(); e != nil {
			return e
		}
		s.activity = nil
	}
	s.level = nil
	s.external = nil
	s.eliminated = nil
	s.literalMark = nil
	s.mark = nil
	s.decision = nil
	if e := s.justification.finalize(); e != nil {
		return e
	}
	s.assignment = nil
	if s.watches != nil {
		if e := s.watches.Close(); e != nil {
			return e
		}
		s.watches = nil
	}
	return nil
}
