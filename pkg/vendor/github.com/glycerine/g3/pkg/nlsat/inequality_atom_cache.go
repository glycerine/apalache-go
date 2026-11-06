package nlsat

// Source: src/nlsat/nlsat_solver.cpp:454-458,510-545. This is the actual
// canonical inequality table/factory subobject, not a solver implementation.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrAtomCacheOwner = errors.New("nlsat: invalid atom-cache owner")
var ErrAtomCachePending = errors.New("nlsat: pending atom-cache operation")
var ErrAtomCacheLive = errors.New("nlsat: live canonical atoms")

type inequalityAtomStorage struct {
	atom               InequalityAtom
	owners             []*polynomial.PolynomialRef
	raw                umemory.ObjectRef
	extent             uint64
	hash               uint32
	inTable, releasing bool
	release            umemory.ReleaseProgress
}
type inequalityAtomCache struct {
	self                    *inequalityAtomCache
	pm                      *polynomial.MultivariatePolynomialManager
	core                    *polynomial.MultivariatePolynomialCore
	polynomials             *polynomial.PolynomialCache
	allocator               *umemory.SmallAllocator
	budget                  *arena.Budget
	buckets                 map[uint32][]*inequalityAtomStorage
	owned                   map[*InequalityAtom]*inequalityAtomStorage
	factor                  *polynomial.PolynomialRef
	pending                 *inequalityAtomStorage
	allocation              umemory.SmallAllocatorAllocationOutcome
	allocationFailed        bool
	header                  uint64
	running, failed, closed bool
}

func newInequalityAtomCache(pm *polynomial.MultivariatePolynomialManager, pc *polynomial.PolynomialCache, a *umemory.SmallAllocator) (*inequalityAtomCache, error) {
	if pm == nil || pc == nil || a == nil {
		return nil, arena.ErrConfig
	}
	bound, err := pc.Manager()
	if err != nil {
		return nil, err
	}
	if bound != pm {
		return nil, arena.ErrConfig
	}
	core, err := pm.ArithmeticCore()
	if err != nil {
		return nil, err
	}
	z, err := core.IntegerManager()
	if err != nil {
		return nil, err
	}
	if a.RuntimeOwner() != z.RuntimeOwner() {
		return nil, arena.ErrConfig
	}
	if _, err = a.NumFreeObjects(); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(inequalityAtomCache{}).Size())
	if err = z.Budget().Reserve(n); err != nil {
		return nil, err
	}
	c := &inequalityAtomCache{pm: pm, core: core, polynomials: pc, allocator: a, budget: z.Budget(), header: n, buckets: map[uint32][]*inequalityAtomStorage{}, owned: map[*InequalityAtom]*inequalityAtomStorage{}}
	c.self = c
	return c, nil
}
func (c *inequalityAtomCache) owner() error {
	if c == nil || c.self != c {
		return ErrAtomCacheOwner
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}
func (c *inequalityAtomCache) open() error {
	if err := c.owner(); err != nil {
		return err
	}
	if c.running || c.failed || c.pending != nil || c.factor != nil || c.allocationFailed {
		return ErrAtomCachePending
	}
	_, err := c.polynomials.Manager()
	return err
}
func (c *inequalityAtomCache) allocate(n int) (*inequalityAtomStorage, error) {
	extent := uint64(reflect.TypeOf(inequalityAtomStorage{}).Size()) + uint64(n)*(uint64(reflect.TypeOf(InequalityFactor{}).Size())+uint64(reflect.TypeOf((*polynomial.PolynomialRef)(nil)).Size()))
	raw, started, err := c.allocator.AllocateWithOutcomeStarted(extent, &c.allocation)
	if err != nil {
		c.allocationFailed = started
		return nil, err
	}
	o := &inequalityAtomStorage{raw: raw, extent: extent, owners: make([]*polynomial.PolynomialRef, n)}
	c.owned[&o.atom] = o
	c.pending = o
	return o, nil
}
func (c *inequalityAtomCache) deallocate(o *inequalityAtomStorage) error {
	var err error
	if o.releasing {
		err = c.allocator.CompleteRelease(o.extent, o.raw, &o.release)
	} else {
		o.release, err = c.allocator.DeallocateTracked(o.extent, o.raw)
		o.releasing = o.release != (umemory.ReleaseProgress{})
	}
	if err != nil {
		return err
	}
	delete(c.owned, &o.atom)
	o.raw = umemory.ObjectRef{}
	o.atom.factors = nil
	o.owners = nil
	return nil
}
func (c *inequalityAtomCache) eraseStorage(o *inequalityAtomStorage) {
	if !o.inTable {
		return
	}
	bucket := c.buckets[o.hash]
	for i, entry := range bucket {
		if entry == o {
			copy(bucket[i:], bucket[i+1:])
			bucket[len(bucket)-1] = nil
			bucket = bucket[:len(bucket)-1]
			break
		}
	}
	if len(bucket) == 0 {
		delete(c.buckets, o.hash)
	} else {
		c.buckets[o.hash] = bucket
	}
	o.inTable = false
}

// mk is the inner source overload returning (atom,is_new). It deliberately
// leaves new atoms' Boolean variables unassigned for the actual outer factory.
func (c *inequalityAtomCache) mk(k AtomKind, ps []polynomial.Polynomial, even []bool) (atom *InequalityAtom, isNew bool, err error) {
	if err = c.open(); err != nil {
		return nil, false, err
	}
	if err = atomAssert("nlsat_solver.cpp", 511, "sz >= 1", len(ps) >= 1); err != nil {
		return nil, false, err
	}
	if err = atomAssert("nlsat_solver.cpp", 512, "k == atom::LT || k == atom::GT || k == atom::EQ", k == AtomLT || k == AtomGT || k == AtomEQ); err != nil {
		return nil, false, err
	}
	if len(ps) == 0 || len(ps) != len(even) || uint64(len(ps)) > uint64(^uint32(0)) {
		return nil, false, arena.ErrBounds
	}
	c.running = true
	defer func() {
		c.running = false
		if err != nil {
			c.failed = true
		}
	}()
	c.factor, err = polynomial.NewPolynomialRef(c.core)
	if err != nil {
		return nil, false, err
	}
	sign := 1
	max := uint32(NullVariable)
	unique := make([]polynomial.Polynomial, 0, len(ps))
	for _, p := range ps {
		normalized, e := c.pm.FlipSignIfLeadingMonomialNegative(p)
		if e != nil {
			return nil, false, e
		}
		if err = c.factor.Assign(normalized); err != nil {
			return nil, false, err
		}
		if normalized != p {
			sign = -sign
		}
		current, e := c.core.MaxVariable(normalized)
		if e != nil {
			return nil, false, e
		}
		if current > max || max == NullVariable {
			max = current
		}
		canonical, e := c.polynomials.MkUnique(normalized)
		if e != nil {
			return nil, false, e
		}
		unique = append(unique, canonical)
	}
	o, e := c.allocate(len(ps))
	if e != nil {
		return nil, false, e
	}
	if sign < 0 {
		k, err = k.Flip()
		if err != nil {
			return nil, false, err
		}
	}
	if err = o.atom.initialize(k, unique, even, max, make([]InequalityFactor, len(ps))); err != nil {
		return nil, false, err
	}
	o.hash, err = o.atom.Hash(c.core)
	if err != nil {
		return nil, false, err
	}
	for _, old := range c.buckets[o.hash] {
		if old.atom.Equal(&o.atom) {
			atom = &old.atom
			break
		}
	}
	if atom == nil {
		c.buckets[o.hash] = append(c.buckets[o.hash], o)
		o.inTable = true
		atom = &o.atom
		isNew = true
	}
	if err = atomAssert("nlsat_solver.cpp", 535, "atom->max_var() == max", atom.maxVar == max); err != nil {
		return nil, false, err
	}
	if isNew {
		for i, p := range unique {
			o.owners[i], err = polynomial.NewPolynomialRef(c.core)
			if err != nil {
				return nil, false, err
			}
			if err = o.owners[i].Assign(p); err != nil {
				return nil, false, err
			}
		}
	} else {
		if err = c.deallocate(o); err != nil {
			return nil, false, err
		}
		c.pending = nil
	}
	if err = c.factor.Close(); err != nil {
		return nil, false, err
	}
	c.factor = nil
	c.pending = nil
	return atom, isNew, nil
}

// erase is the source table-erasure step. The outer solver deletes the Boolean
// variable after this step and before release drops the actual factor owners.
func (c *inequalityAtomCache) erase(a *InequalityAtom) error {
	if err := c.open(); err != nil {
		return err
	}
	o := c.owned[a]
	if o == nil || !o.inTable {
		return ErrAtomCacheOwner
	}
	if err := atomAssert("nlsat_solver.cpp", 477, "a->ref_count() == 0", a.refs == 0); err != nil {
		return err
	}
	if a.refs != 0 {
		return ErrAtomCacheLive
	}
	c.eraseStorage(o)
	return nil
}
func (c *inequalityAtomCache) release(a *InequalityAtom) error {
	if err := c.owner(); err != nil {
		return err
	}
	if c.running {
		return ErrAtomCachePending
	}
	o := c.owned[a]
	if o == nil || o.inTable {
		return ErrAtomCacheOwner
	}
	for i, r := range o.owners {
		if r != nil {
			if err := r.Close(); err != nil {
				return err
			}
			o.owners[i] = nil
		}
	}
	return c.deallocate(o)
}

// cleanup retires only this factory's temporaries. A caller acknowledges stopped
// accepted polynomial/cache callees through their actual owners first.
func (c *inequalityAtomCache) cleanup() error {
	if err := c.owner(); err != nil {
		return err
	}
	if c.running {
		return ErrAtomCachePending
	}
	if c.allocationFailed {
		if c.allocator.MatchesAllocationOutcome(&c.allocation) {
			if err := c.allocator.CleanupPending(); err != nil {
				return err
			}
		}
		c.allocationFailed = false
	}
	if c.pending != nil {
		o := c.pending
		c.eraseStorage(o)
		for i, r := range o.owners {
			if r != nil {
				if err := r.Close(); err != nil {
					return err
				}
				o.owners[i] = nil
			}
		}
		if err := c.deallocate(o); err != nil {
			return err
		}
		c.pending = nil
	}
	if c.factor != nil {
		if err := c.factor.Close(); err != nil {
			return err
		}
		c.factor = nil
	}
	c.failed = false
	return nil
}
func (c *inequalityAtomCache) close() error {
	if c != nil && c.self == c && c.closed {
		return nil
	}
	if err := c.cleanup(); err != nil {
		return err
	}
	if len(c.owned) != 0 {
		return ErrAtomCacheLive
	}
	c.buckets = nil
	c.owned = nil
	c.budget.Release(c.header)
	c.header = 0
	c.closed = true
	return nil
}
