package nlsat

// Source: src/nlsat/nlsat_solver.cpp:195-201,601-650,694-720.

import (
	"reflect"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/idgen"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type clauseStorage struct {
	clause                            Clause
	raw                               umemory.ObjectRef
	extent                            uint64
	acquired                          []sat.Literal
	constructing, published, attached bool
	asOwned                           bool
	stage                             uint8
	index                             int
	literalConsumed                   bool
	atomPending                       ArithmeticAtom
	asmReceipt                        assumptionDeletionReceipt
	asmPending                        bool
	releasing                         bool
	release                           umemory.ReleaseProgress
}

// clauseStore is the actual owning clause/variable subobject. The parent still
// has to supply explanation, trail, propagation and search; no solver is faked.
type clauseStore struct {
	self                    *clauseStore
	atoms                   *atomFactory
	assumptions             *assumptionManager
	vars                    arithmeticVariables
	search                  *searchState
	ids                     idgen.Generator
	originals, learned      []*Clause
	owned                   map[*Clause]*clauseStorage
	pending, retiring       *clauseStorage
	allocation              umemory.SmallAllocatorAllocationOutcome
	allocationFailed        bool
	inputLeaf               assumptionLeafReceipt
	header                  uint64
	running, failed, closed bool
}

func newClauseStore(atoms *atomFactory, assumptions *assumptionManager) (*clauseStore, error) {
	if err := atoms.open(); err != nil {
		return nil, err
	}
	if err := assumptions.open(); err != nil {
		return nil, err
	}
	if atoms.clauses != nil || assumptions.clauses != nil || assumptions.allocator != atoms.ineq.allocator {
		return nil, arena.ErrConfig
	}
	n, err := atoms.ineq.core.NumVars()
	if err != nil {
		return nil, err
	}
	if n != 0 {
		// Existing variables have caller-owned integer/permutation metadata;
		// the new source owner must not synthesize substitutes for it.
		return nil, arena.ErrConfig
	}
	h := uint64(reflect.TypeOf(clauseStore{}).Size())
	if err := atoms.ineq.budget.Reserve(h); err != nil {
		return nil, err
	}
	s := &clauseStore{atoms: atoms, assumptions: assumptions, owned: map[*Clause]*clauseStorage{}, header: h}
	s.self = s
	s.ids = idgen.New(0, func() bool { return atoms.ineq.allocator.RuntimeOwner().State().OutOfMemory })
	atoms.clauses, assumptions.clauses = s, s
	return s, nil
}
func (s *clauseStore) owner() error {
	if s == nil || s.self != s {
		return ErrAtomCacheOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	if s.atoms.clauses != s || s.assumptions.clauses != s {
		return ErrAtomCacheOwner
	}
	return nil
}
func (s *clauseStore) open() error {
	if err := s.owner(); err != nil {
		return err
	}
	if s.running || s.failed || s.pending != nil || s.retiring != nil || s.allocationFailed || s.inputLeaf.owner != nil {
		return ErrAtomCachePending
	}
	if err := s.atoms.open(); err != nil {
		return err
	}
	return s.assumptions.open()
}

func (s *clauseStore) mkClause(lits []sat.Literal, learned bool, as *AssumptionSet) (c *Clause, err error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	s.running = true
	defer func() { s.running = false; s.failed = err != nil }()
	return s.createClause(lits, learned, as)
}

func (s *clauseStore) mkInputClause(lits []sat.Literal, a Assumption) (err error) {
	if err := s.open(); err != nil {
		return err
	}
	s.running = true
	defer func() { s.running = false; s.failed = err != nil }()
	if err := atomAssert("nlsat_solver.cpp", 714, "num_lits > 0", len(lits) > 0); err != nil {
		return err
	}
	if len(lits) == 0 {
		return arena.ErrBounds
	}
	var as *AssumptionSet
	if !a.IsNull() {
		as, err = s.assumptions.mkLeafWithReceipt(a, &s.inputLeaf)
		if err != nil {
			return err
		}
	}
	if _, err = s.createClause(lits, false, as); err != nil {
		return err
	}
	s.inputLeaf = assumptionLeafReceipt{} // the real clause INC acquired the leaf.
	return nil
}
func (s *clauseStore) createClause(lits []sat.Literal, learned bool, as *AssumptionSet) (*Clause, error) {
	if err := atomAssert("nlsat_solver.cpp", 695, "num_lits > 0", len(lits) > 0); err != nil {
		return nil, err
	}
	if len(lits) == 0 || uint64(len(lits)) > uint64(^uint32(0)) {
		return nil, arena.ErrBounds
	}
	// A failed allocation has consumed cid, exactly as the source does.
	id := s.ids.Mk()
	extent := uint64(reflect.TypeOf(clauseStorage{}).Size()) + uint64(len(lits))*8
	raw, started, err := s.atoms.ineq.allocator.AllocateWithOutcomeStarted(extent, &s.allocation)
	if err != nil {
		s.allocationFailed = started
		return nil, err
	}
	o := &clauseStorage{raw: raw, extent: extent, constructing: true, acquired: make([]sat.Literal, 0, len(lits))}
	s.pending = o
	s.owned[&o.clause] = o
	if err := o.clause.initialize(id, lits, learned, as, make([]sat.Literal, len(lits))); err != nil {
		return nil, err
	}
	for _, l := range lits {
		if err := s.atoms.incRef(l.Variable()); err != nil {
			return nil, err
		}
		o.acquired = append(o.acquired, l)
	}
	if as != nil {
		if err := s.assumptions.incRef(as); err != nil {
			return nil, err
		}
		o.asOwned = true
	}
	var sortError error
	values := o.clause.Literals()
	sort.Slice(values, func(i, j int) bool {
		if sortError != nil {
			return false
		}
		var less bool
		less, sortError = s.literalLess(values[i], values[j])
		return less
	})
	if sortError != nil {
		return nil, sortError
	}
	if learned {
		s.learned = append(s.learned, &o.clause)
	} else {
		s.originals = append(s.originals, &o.clause)
	}
	o.published = true
	if err := s.attach(&o.clause); err != nil {
		return nil, err
	}
	o.attached = true
	o.constructing = false
	o.acquired = nil
	s.pending = nil
	return &o.clause, nil
}

func (s *clauseStore) mkTrueBooleanVariable() (err error) {
	if err := s.open(); err != nil {
		return err
	}
	s.running = true
	defer func() { s.running = false; s.failed = err != nil }()
	b, err := s.atoms.mkBoolVar()
	if err != nil {
		return err
	}
	if err := atomAssert("nlsat_solver.cpp", 198, "b == true_bool_var", b == TrueBooleanVariable); err != nil {
		return err
	}
	_, err = s.createClause([]sat.Literal{sat.MakeLiteral(b, false)}, false, nil)
	return err
}

func (s *clauseStore) watchVector(c *Clause) (*[]*Clause, error) {
	x, err := s.maxVar(c.Literals())
	if err != nil {
		return nil, err
	}
	if x != NullVariable {
		if uint64(x) >= uint64(len(s.vars.watches)) {
			return nil, arena.ErrBounds
		}
		return &s.vars.watches[x], nil
	}
	b := clauseMaxBoolVar(c.Literals())
	if uint64(b) >= uint64(len(s.atoms.bools.watches)) {
		return nil, arena.ErrBounds
	}
	return &s.atoms.bools.watches[b], nil
}
func (s *clauseStore) attach(c *Clause) error {
	v, err := s.watchVector(c)
	if err != nil {
		return err
	}
	*v = append(*v, c)
	return nil
}
func removeClause(values []*Clause, c *Clause) []*Clause {
	for i, old := range values {
		if old == c {
			copy(values[i:], values[i+1:])
			values[len(values)-1] = nil
			return values[:len(values)-1]
		}
	}
	return values
}
func (s *clauseStore) detach(c *Clause) error {
	v, err := s.watchVector(c)
	if err != nil {
		return err
	}
	*v = removeClause(*v, c)
	return nil
}
func (s *clauseStore) beginRetirement(o *clauseStorage) {
	if o.published {
		if o.clause.learned {
			s.learned = removeClause(s.learned, &o.clause)
		} else {
			s.originals = removeClause(s.originals, &o.clause)
		}
		o.published = false
	}
	s.retiring = o
}
func (s *clauseStore) deleteClause(c *Clause) error {
	if err := s.open(); err != nil {
		return err
	}
	o := s.owned[c]
	if o == nil {
		return ErrAtomCacheOwner
	}
	s.running = true
	defer func() { s.running = false }()
	s.beginRetirement(o)
	return s.drainRetirement()
}

func (s *clauseStore) drainRetirement() error {
	o := s.retiring
	if o == nil {
		return nil
	}
	if o.stage == 0 {
		if o.attached {
			if err := s.detach(&o.clause); err != nil {
				return err
			}
			o.attached = false
		}
		o.stage = 1
	}
	if o.stage == 1 {
		if err := s.ids.Recycle(o.clause.ID()); err != nil {
			return err
		}
		o.stage = 2
	}
	if o.stage == 2 {
		if o.atomPending != nil {
			if s.atoms.retiring == o.atomPending {
				if err := s.atoms.cleanup(); err != nil {
					return err
				}
			}
			o.atomPending = nil
		}
		if o.literalConsumed {
			o.index++
			o.literalConsumed = false
		}
		lits := o.clause.Literals()
		if o.constructing {
			lits = o.acquired
		}
		for o.index < len(lits) {
			l := lits[o.index]
			a, err := s.literalAtom(l)
			if err != nil {
				return err
			}
			h := arithmeticAtomHeader(a)
			before := uint32(0)
			if h != nil {
				before = h.refs
			}
			if err := s.atoms.decRef(l.Variable()); err != nil {
				if h != nil && before > 0 && h.refs == before-1 {
					o.literalConsumed = true
					if s.atoms.retiring == a {
						o.atomPending = a
					}
				}
				return err
			}
			o.index++
		}
		o.stage = 3
	}
	if o.stage == 3 {
		if o.asmPending {
			if err := s.assumptions.cleanupDeletion(&o.asmReceipt); err != nil {
				return err
			}
			o.asmPending = false
		}
		if o.asOwned {
			if err := s.assumptions.decRefWithReceipt(o.clause.assumptions, &o.asmReceipt); err != nil {
				if o.asmReceipt.owner == s.assumptions && o.asmReceipt.serial != 0 {
					o.asOwned = false
					o.asmPending = true
				}
				return err
			}
			o.asOwned = false
		}
		o.stage = 4
	}
	var err error
	if o.releasing {
		err = s.atoms.ineq.allocator.CompleteRelease(o.extent, o.raw, &o.release)
	} else {
		o.release, err = s.atoms.ineq.allocator.DeallocateTracked(o.extent, o.raw)
		o.releasing = o.release != (umemory.ReleaseProgress{})
	}
	if err != nil {
		return err
	}
	delete(s.owned, &o.clause)
	if s.pending == o {
		s.pending = nil
	}
	s.retiring = nil
	o.raw = umemory.ObjectRef{}
	o.clause.literals = nil
	o.acquired = nil
	return nil
}

func (s *clauseStore) cleanup() error {
	if err := s.owner(); err != nil {
		return err
	}
	if s.running {
		return ErrAtomCachePending
	}
	s.running = true
	defer func() { s.running = false }()
	if s.allocationFailed {
		if s.atoms.ineq.allocator.MatchesAllocationOutcome(&s.allocation) {
			if err := s.atoms.ineq.allocator.CleanupPending(); err != nil {
				return err
			}
		}
		s.allocationFailed = false
	}
	if err := s.drainRetirement(); err != nil {
		return err
	}
	if s.pending != nil {
		s.beginRetirement(s.pending)
		if err := s.drainRetirement(); err != nil {
			return err
		}
	}
	if err := s.assumptions.cleanupLeaf(&s.inputLeaf); err != nil {
		return err
	}
	s.failed = false
	return nil
}
func (s *clauseStore) close() error {
	if s != nil && s.self == s && s.closed {
		return nil
	}
	if s != nil && s.self == s && s.search != nil {
		return ErrSearchLive
	}
	if err := s.cleanup(); err != nil {
		return err
	}
	for _, set := range s.vars.infeasible {
		if set != nil {
			return ErrIntervalLive
		}
	}
	s.running = true
	defer func() { s.running = false }()
	for len(s.originals) != 0 || len(s.learned) != 0 {
		list := s.originals
		if len(list) == 0 {
			list = s.learned
		}
		s.beginRetirement(s.owned[list[0]])
		if err := s.drainRetirement(); err != nil {
			return err
		}
	}
	s.atoms.clauses, s.assumptions.clauses = nil, nil
	s.atoms.ineq.budget.Release(s.header)
	s.header = 0
	s.vars = arithmeticVariables{}
	s.originals, s.learned, s.owned = nil, nil, nil
	s.closed = true
	return nil
}
