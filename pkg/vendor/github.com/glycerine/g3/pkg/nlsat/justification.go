package nlsat

// Source: src/nlsat/nlsat_justification.h.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type JustificationKind uint8

const (
	NullJustification JustificationKind = iota
	DecisionJustification
	ClauseJustification
	LazyJustificationKind
)

// Justification is a borrowed, comparable tagged union. Only a lazy reason owns
// a separate allocator object, retired by the solver's del_jst counterpart.
type Justification struct {
	kind   JustificationKind
	clause *Clause
	lazy   *LazyJustification
}

func (j Justification) Kind() JustificationKind { return j.kind }
func (j Justification) IsNull() bool            { return j.kind == NullJustification }
func (j Justification) IsDecision() bool        { return j.kind == DecisionJustification }
func (j Justification) IsClause() bool          { return j.kind == ClauseJustification }
func (j Justification) IsLazy() bool            { return j.kind == LazyJustificationKind }
func (j Justification) Clause() (*Clause, error) {
	if !j.IsClause() {
		return nil, arena.ErrConfig
	}
	return j.clause, nil
}
func (j Justification) Lazy() (*LazyJustification, error) {
	if !j.IsLazy() {
		return nil, arena.ErrConfig
	}
	return j.lazy, nil
}
func NewDecisionJustification() (Justification, error) {
	j := Justification{kind: DecisionJustification}
	return j, atomAssert("nlsat_justification.h", 58, "is_decision()", j.IsDecision())
}
func NewClauseJustification(c *Clause) (Justification, error) {
	j := Justification{kind: ClauseJustification, clause: c}
	return j, atomAssert("nlsat_justification.h", 59, "is_clause()", j.IsClause())
}

type LazyJustification struct {
	self                               *LazyJustification
	allocator                          *umemory.SmallAllocator
	literals                           []sat.Literal
	raw                                umemory.ObjectRef
	extent                             uint64
	allocation                         umemory.SmallAllocatorAllocationOutcome
	allocationFailed                   bool
	release                            umemory.ReleaseProgress
	ready, releasing, deleting, closed bool
}

// NewLazyJustification copies the reason literals into one solver allocation.
// An error may return a partial lazy owner; DeleteJustification acknowledges only
// that owner's accepted allocation and releases it before retry.
func NewLazyJustification(a *umemory.SmallAllocator, lits []sat.Literal) (Justification, error) {
	if a == nil || uint64(len(lits)) > uint64(^uint32(0)) {
		return Justification{}, arena.ErrConfig
	}
	l := &LazyJustification{allocator: a, extent: uint64(reflect.TypeOf(LazyJustification{}).Size()) + uint64(len(lits))*4}
	l.self = l
	j := Justification{kind: LazyJustificationKind, lazy: l}
	raw, started, err := a.AllocateWithOutcomeStarted(l.extent, &l.allocation)
	if err != nil {
		l.allocationFailed = started
		return j, err
	}
	l.raw = raw
	l.literals = make([]sat.Literal, len(lits))
	copy(l.literals, lits)
	l.ready = true
	return j, atomAssert("nlsat_justification.h", 60, "is_lazy()", j.IsLazy())
}
func (l *LazyJustification) open() error {
	if l == nil || l.self != l {
		return umemory.ErrAllocatorOwner
	}
	if l.closed {
		return arena.ErrClosed
	}
	if !l.ready || l.deleting {
		return umemory.ErrAllocatorPending
	}
	return nil
}
func (l *LazyJustification) Size() (uint32, error) {
	if err := l.open(); err != nil {
		return 0, err
	}
	return uint32(len(l.literals)), nil
}
func (l *LazyJustification) Literal(i uint32) (sat.Literal, error) {
	if err := l.open(); err != nil {
		return 0, err
	}
	if err := atomAssert("nlsat_justification.h", 49, "i < size()", uint64(i) < uint64(len(l.literals))); err != nil {
		return 0, err
	}
	if uint64(i) >= uint64(len(l.literals)) {
		return 0, arena.ErrBounds
	}
	return l.literals[i], nil
}

// Literals borrows the immutable source reason; the owner outlives this view.
func (l *LazyJustification) Literals() ([]sat.Literal, error) {
	if err := l.open(); err != nil {
		return nil, err
	}
	return l.literals, nil
}
func DeleteJustification(a *umemory.SmallAllocator, j Justification) error {
	if !j.IsLazy() {
		return nil
	}
	l := j.lazy
	if l == nil || l.self != l || l.allocator != a {
		return umemory.ErrAllocatorOwner
	}
	if l.closed {
		return nil
	}
	if l.allocationFailed {
		if a.MatchesAllocationOutcome(&l.allocation) {
			if err := a.CleanupPending(); err != nil {
				return err
			}
		}
		l.allocationFailed = false
	}
	if !l.ready {
		l.closed = true
		return nil
	}
	l.deleting = true
	var err error
	if l.releasing {
		err = a.CompleteRelease(l.extent, l.raw, &l.release)
	} else {
		l.release, err = a.DeallocateTracked(l.extent, l.raw)
		l.releasing = l.release != (umemory.ReleaseProgress{})
	}
	if err != nil {
		return err
	}
	l.raw = umemory.ObjectRef{}
	l.literals = nil
	l.closed = true
	return nil
}
