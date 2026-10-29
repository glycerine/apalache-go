package nlsat

// Source: src/nlsat/nlsat_solver.cpp:272-298,454-499,548-599.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type rootAtomStorage struct {
	atom      RootAtom
	owner     *polynomial.PolynomialRef
	raw       umemory.ObjectRef
	extent    uint64
	hash      uint32
	inTable   bool
	releasing bool
	release   umemory.ReleaseProgress
}

// atomFactory owns the solver's atom tables and Boolean registry. Search,
// clauses, assignments and explanation are separate actual solver components;
// this factory does not implement a solver constructor or assert the true clause.
type atomFactory struct {
	self             *atomFactory
	ineq             *inequalityAtomCache
	bools            booleanVariables
	roots            map[uint32][]*rootAtomStorage
	ownedRoots       map[*RootAtom]*rootAtomStorage
	rootFactor       *polynomial.PolynomialRef
	pendingRoot      *rootAtomStorage
	rootAllocation   umemory.SmallAllocatorAllocationOutcome
	allocationFailed bool
	ineqCall         bool
	retiring         ArithmeticAtom
	clauses          *clauseStore
	retireStage      uint8
	header           uint64
	running, failed  bool
	closed           bool
}

func newAtomFactory(pm *polynomial.MultivariatePolynomialManager, pc *polynomial.PolynomialCache, allocator *umemory.SmallAllocator) (*atomFactory, error) {
	ineq, err := newInequalityAtomCache(pm, pc, allocator)
	if err != nil {
		return nil, err
	}
	f := &atomFactory{ineq: ineq, roots: map[uint32][]*rootAtomStorage{}, ownedRoots: map[*RootAtom]*rootAtomStorage{}}
	f.self = f
	f.bools.initialize(allocator.RuntimeOwner())
	n := uint64(reflect.TypeOf(atomFactory{}).Size())
	if err = ineq.budget.Reserve(n); err != nil {
		f.failed = true
		return f, err
	}
	f.header = n
	return f, nil
}

func (f *atomFactory) owner() error {
	if f == nil || f.self != f || f.ineq == nil {
		return ErrAtomCacheOwner
	}
	if f.closed {
		return arena.ErrClosed
	}
	return nil
}
func (f *atomFactory) open() error {
	if err := f.owner(); err != nil {
		return err
	}
	if f.running || f.failed || f.retiring != nil || f.pendingRoot != nil || f.rootFactor != nil || f.allocationFailed || f.ineqCall {
		return ErrAtomCachePending
	}
	return f.ineq.open()
}

func (f *atomFactory) mkBoolVar() (sat.BoolVar, error) {
	if err := f.open(); err != nil {
		return sat.NullBoolVar, err
	}
	return f.bools.mk()
}

func (f *atomFactory) mkInequality(k AtomKind, ps []polynomial.Polynomial, even []bool) (result sat.BoolVar, err error) {
	if err = f.open(); err != nil {
		return sat.NullBoolVar, err
	}
	f.running = true
	defer func() { f.running = false; f.failed = err != nil }()
	f.ineqCall = true
	a, fresh, err := f.ineq.mk(k, ps, even)
	if err != nil {
		return sat.NullBoolVar, err
	}
	f.ineqCall = false
	if !fresh {
		return a.boolVar, nil
	}
	b, err := f.bools.mk()
	if err != nil {
		// The source atom is already inserted and owns its factor references.
		f.retiring = a
		return sat.NullBoolVar, err
	}
	f.bools.atoms[b] = a
	a.boolVar = b
	return b, nil
}

func (f *atomFactory) mkInequalityLiteral(k AtomKind, ps []polynomial.Polynomial, even []bool) (sat.Literal, error) {
	if err := f.open(); err != nil {
		return sat.DefaultLiteral(), err
	}
	if err := atomAssert("nlsat_solver.cpp", 563, "k == atom::LT || k == atom::GT || k == atom::EQ", k == AtomLT || k == AtomGT || k == AtomEQ); err != nil {
		return sat.DefaultLiteral(), err
	}
	if len(ps) == 0 {
		switch k {
		case AtomLT, AtomEQ:
			return FalseLiteral, nil
		case AtomGT:
			return TrueLiteral, nil
		default:
			return sat.DefaultLiteral(), arena.ErrConfig
		}
	}
	b, err := f.mkInequality(k, ps, even)
	if err != nil {
		return sat.DefaultLiteral(), err
	}
	return sat.MakeLiteral(b, false), nil
}

func (f *atomFactory) allocateRoot() (*rootAtomStorage, error) {
	extent := uint64(reflect.TypeOf(rootAtomStorage{}).Size())
	raw, started, err := f.ineq.allocator.AllocateWithOutcomeStarted(extent, &f.rootAllocation)
	if err != nil {
		f.allocationFailed = started
		return nil, err
	}
	o := &rootAtomStorage{raw: raw, extent: extent}
	f.ownedRoots[&o.atom] = o
	f.pendingRoot = o
	return o, nil
}
func (f *atomFactory) deallocateRoot(o *rootAtomStorage) error {
	var err error
	if o.releasing {
		err = f.ineq.allocator.CompleteRelease(o.extent, o.raw, &o.release)
	} else {
		o.release, err = f.ineq.allocator.DeallocateTracked(o.extent, o.raw)
		o.releasing = o.release != (umemory.ReleaseProgress{})
	}
	if err != nil {
		return err
	}
	delete(f.ownedRoots, &o.atom)
	if f.pendingRoot == o {
		f.pendingRoot = nil
	}
	o.raw = umemory.ObjectRef{}
	return nil
}
func (f *atomFactory) eraseRoot(o *rootAtomStorage) {
	if !o.inTable {
		return
	}
	bucket := f.roots[o.hash]
	for i, old := range bucket {
		if old == o {
			copy(bucket[i:], bucket[i+1:])
			bucket[len(bucket)-1] = nil
			bucket = bucket[:len(bucket)-1]
			break
		}
	}
	if len(bucket) == 0 {
		delete(f.roots, o.hash)
	} else {
		f.roots[o.hash] = bucket
	}
	o.inTable = false
}

func (f *atomFactory) mkRoot(k AtomKind, x, index uint32, p polynomial.Polynomial) (result sat.BoolVar, err error) {
	if err = f.open(); err != nil {
		return sat.NullBoolVar, err
	}
	if err = atomAssert("nlsat_solver.cpp", 580, "i > 0", index > 0); err != nil {
		return sat.NullBoolVar, err
	}
	max, err := f.ineq.core.MaxVariable(p)
	if err != nil {
		return sat.NullBoolVar, err
	}
	if err = atomAssert("nlsat_solver.cpp", 581, "x >= max_var(p)", x >= max); err != nil {
		return sat.NullBoolVar, err
	}
	validKind := k >= AtomRootEQ && k <= AtomRootGE
	if err = atomAssert("nlsat_solver.cpp", 582, "k == atom::ROOT_LT || k == atom::ROOT_GT || k == atom::ROOT_EQ || k == atom::ROOT_LE || k == atom::ROOT_GE", validKind); err != nil {
		return sat.NullBoolVar, err
	}
	if index == 0 || x < max || !validKind {
		return sat.NullBoolVar, arena.ErrConfig
	}
	f.running = true
	defer func() { f.running = false; f.failed = err != nil }()
	f.rootFactor, err = polynomial.NewPolynomialRef(f.ineq.core)
	if err != nil {
		return sat.NullBoolVar, err
	}
	normalized, err := f.ineq.pm.FlipSignIfLeadingMonomialNegative(p)
	if err != nil {
		return sat.NullBoolVar, err
	}
	if err = f.rootFactor.Assign(normalized); err != nil {
		return sat.NullBoolVar, err
	}
	unique, err := f.ineq.polynomials.MkUnique(normalized)
	if err != nil {
		return sat.NullBoolVar, err
	}
	o, err := f.allocateRoot()
	if err != nil {
		return sat.NullBoolVar, err
	}
	if err = o.atom.initialize(k, x, index, unique); err != nil {
		return sat.NullBoolVar, err
	}
	o.hash, err = o.atom.Hash(f.ineq.core)
	if err != nil {
		return sat.NullBoolVar, err
	}
	atom := &o.atom
	for _, old := range f.roots[o.hash] {
		if old.atom.Equal(atom) {
			atom = &old.atom
			break
		}
	}
	if atom == &o.atom {
		f.roots[o.hash] = append(f.roots[o.hash], o)
		o.inTable = true
	}
	if err = atomAssert("nlsat_solver.cpp", 589, "old_atom->max_var() == x", atom.maxVar == x); err != nil {
		return sat.NullBoolVar, err
	}
	if atom != &o.atom {
		if err = f.deallocateRoot(o); err != nil {
			return sat.NullBoolVar, err
		}
	} else {
		b, e := f.bools.mk()
		if e != nil {
			return sat.NullBoolVar, e
		}
		f.bools.atoms[b] = atom
		atom.boolVar = b
		// Source root INC follows Boolean publication. The concrete native
		// reference owner adds its own charged header at precisely this stage.
		o.owner, err = polynomial.NewPolynomialRef(f.ineq.core)
		if err != nil {
			return sat.NullBoolVar, err
		}
		if err = o.owner.Assign(unique); err != nil {
			return sat.NullBoolVar, err
		}
	}
	if err = f.rootFactor.Close(); err != nil {
		return sat.NullBoolVar, err
	}
	f.rootFactor = nil
	f.pendingRoot = nil
	return atom.boolVar, nil
}

func (f *atomFactory) incRef(b sat.BoolVar) error {
	if err := f.open(); err != nil {
		return err
	}
	if b == sat.NullBoolVar {
		return nil
	}
	if uint64(b) >= uint64(len(f.bools.atoms)) || f.bools.dead[b] {
		return arena.ErrBounds
	}
	if a := arithmeticAtomHeader(f.bools.atoms[b]); a != nil {
		a.IncRef()
	}
	return nil
}
func (f *atomFactory) decRef(b sat.BoolVar) error {
	if err := f.open(); err != nil {
		return err
	}
	if b == sat.NullBoolVar {
		return nil
	}
	if uint64(b) >= uint64(len(f.bools.atoms)) || f.bools.dead[b] {
		return arena.ErrBounds
	}
	a := f.bools.atoms[b]
	header := arithmeticAtomHeader(a)
	if header == nil {
		return nil
	}
	if err := atomAssert("nlsat_solver.cpp", 289, "a->ref_count() > 0", header.refs > 0); err != nil {
		return err
	}
	if err := header.DecRef(); err != nil {
		return err
	}
	if header.refs == 0 {
		return f.deleteAtom(a)
	}
	return nil
}

func (f *atomFactory) deleteAtom(a ArithmeticAtom) error {
	if err := f.open(); err != nil {
		return err
	}
	if a == nil {
		return nil
	}
	header := arithmeticAtomHeader(a)
	if header == nil {
		return ErrAtomCacheOwner
	}
	switch a := a.(type) {
	case *InequalityAtom:
		if f.ineq.owned[a] == nil {
			return ErrAtomCacheOwner
		}
	case *RootAtom:
		if f.ownedRoots[a] == nil {
			return ErrAtomCacheOwner
		}
	}
	line := int32(477)
	if header.IsRoot() {
		line = 487
	}
	if err := atomAssert("nlsat_solver.cpp", line, "a->ref_count() == 0", header.refs == 0); err != nil {
		return err
	}
	if header.refs != 0 {
		return ErrAtomCacheLive
	}
	f.running = true
	defer func() { f.running = false }()
	f.retiring = a
	err := f.continueRetirement()
	f.failed = err != nil
	return err
}

func (f *atomFactory) continueRetirement() error {
	a := f.retiring
	if a == nil {
		return nil
	}
	if f.retireStage == 0 {
		switch a := a.(type) {
		case *InequalityAtom:
			if err := f.ineq.erase(a); err != nil {
				return err
			}
		case *RootAtom:
			o := f.ownedRoots[a]
			if o == nil {
				return ErrAtomCacheOwner
			}
			f.eraseRoot(o)
		}
		f.retireStage = 1
	}
	if f.retireStage == 1 {
		b := arithmeticAtomHeader(a).boolVar
		if b != sat.NullBoolVar {
			if err := f.bools.erase(b); err != nil {
				return err
			}
		}
		f.retireStage = 2
	}
	switch a := a.(type) {
	case *InequalityAtom:
		if err := f.ineq.release(a); err != nil {
			return err
		}
	case *RootAtom:
		o := f.ownedRoots[a]
		if o.owner != nil {
			if err := o.owner.Close(); err != nil {
				return err
			}
			o.owner = nil
		}
		if err := f.deallocateRoot(o); err != nil {
			return err
		}
	}
	f.retiring = nil
	f.retireStage = 0
	return nil
}

// cleanup acknowledges only this factory's accepted storage/temporary work.
// The caller first acknowledges reached polynomial manager/cache callees through
// their actual owners, just as for the inner inequality factory.
func (f *atomFactory) cleanup() error {
	if err := f.owner(); err != nil {
		return err
	}
	if f.running {
		return ErrAtomCachePending
	}
	f.running = true
	defer func() { f.running = false }()
	if f.ineqCall {
		if err := f.ineq.cleanup(); err != nil {
			return err
		}
		f.ineqCall = false
	}
	if err := f.continueRetirement(); err != nil {
		return err
	}
	if f.allocationFailed {
		if f.ineq.allocator.MatchesAllocationOutcome(&f.rootAllocation) {
			if err := f.ineq.allocator.CleanupPending(); err != nil {
				return err
			}
		}
		f.allocationFailed = false
	}
	if o := f.pendingRoot; o != nil {
		f.retiring = &o.atom
		if err := f.continueRetirement(); err != nil {
			return err
		}
	}
	if f.rootFactor != nil {
		if err := f.rootFactor.Close(); err != nil {
			return err
		}
		f.rootFactor = nil
	}
	f.failed = false
	return nil
}

func (f *atomFactory) close() error {
	if f != nil && f.self == f && f.closed {
		return nil
	}
	if f != nil && f.self == f && f.clauses != nil {
		if f.clauses.running {
			return ErrAtomCachePending
		}
		return ErrAtomCacheLive
	}
	if err := f.cleanup(); err != nil {
		return err
	}
	if len(f.ownedRoots) != 0 || len(f.ineq.owned) != 0 {
		return ErrAtomCacheLive
	}
	if err := f.ineq.close(); err != nil {
		return err
	}
	f.ineq.budget.Release(f.header)
	f.header = 0
	f.bools = booleanVariables{}
	f.roots = nil
	f.ownedRoots = nil
	f.closed = true
	return nil
}
