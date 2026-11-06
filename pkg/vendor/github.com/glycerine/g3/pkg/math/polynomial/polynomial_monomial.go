package polynomial

// Source: src/math/polynomial/polynomial.cpp; src/math/polynomial/polynomial.h.
// Actual standalone polynomial.cpp monomial owner. This is not polynomial::imp.

import (
	"errors"
	"math"
	"reflect"
	"runtime"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/idgen"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrMonomialOwner = errors.New("mpnum: invalid monomial owner")
var ErrMonomialAssertion = errors.New("mpnum: monomial source assertion")
var ErrMonomialPending = errors.New("mpnum: retained monomial construction")
var ErrMonomialLive = errors.New("mpnum: live caller monomials")
var monomialOrigins atomic.Uint64

type Monomial struct {
	owner            uint64
	slot, generation uint32
}
type MonomialPower struct{ Var, Degree uint32 }
type MonomialInfo struct{ RefCount, ID, TotalDegree, Size, Hash uint32 }
type MonomialManagerConfig struct {
	Budget                      *arena.Budget
	Memory                      *umemory.Runtime
	Allocator                   *umemory.SmallAllocator
	Profile                     umemory.SmallAllocatorProfile
	Trace                       *diagnostic.TraceTags
	polynomialImpAllocationLine int
}
type monomialLocation struct {
	toPolynomialCohort                 *polynomialToPolynomialScope
	toPolynomialInvocation             uint64
	pseudoCohort                       *polynomialPseudoDivisionScope
	pseudoInvocation                   uint64
	dividesCohort                      *polynomialDividesScope
	dividesInvocation                  uint64
	arithmeticCohort                   *polynomialArithmeticScope
	arithmeticInvocation               uint64
	conversionCohort                   *polynomialGlobalConversionScope
	conversionSlot                     uint32
	compositionCohort                  *polynomialCompositionScope
	compositionSlot                    uint32
	squareRootCohort                   *polynomialSquareRootScope
	squareRootSlot                     uint32
	derivativeCohort                   *polynomialDerivativeScope
	derivativeSlot                     uint32
	unreferencedCohort                 *polynomialSubstitutionState
	unreferencedSlot                   uint32
	raw                                umemory.ObjectRef
	sourceRaw                          umemory.Block
	sourceOffset                       uint64
	generation, requested, id          uint32
	stage                              uint8
	live, headerInitialized, published bool
}
type monomialCreation struct {
	toPolynomialParent       *polynomialToPolynomialScope
	toPolynomialInvocation   uint64
	coefficientParent        *polynomialCoefficientScope
	coefficientInvocation    uint64
	generation               uint64
	parent                   *MultivariatePolynomialCore
	tableIndex, slot, id     uint32
	stage                    uint8
	idConsumed, tableRemoved bool
}
type MonomialManager struct {
	lexTraceBorrow                                                            *polynomialLexTraceBorrow
	exactDivTrace                                                             *polynomialExactDivTraceScope
	substitutionTrace                                                         *polynomialSubstitutionTraceBorrow
	assertionRunning                                                          bool
	queryTraceRunning                                                         bool
	managerTraceRunning                                                       bool
	allocatorInvocation                                                       monomialAllocatorScope
	unitDropConsumed                                                          bool
	unitDropCause                                                             error
	continuationGeneration                                                    uint64
	borrowedGeneration                                                        uint64
	borrowedParent                                                            *MultivariatePolynomialCore
	borrowedTemp                                                              *monomialTemp // actual borrowed imp temporary pointer; no ownership
	self                                                                      *MonomialManager
	config                                                                    MonomialManagerConfig
	memory                                                                    *umemory.Runtime
	allocator                                                                 *umemory.SmallAllocator
	ids                                                                       idgen.Generator
	table                                                                     monomialTable
	temps                                                                     [4]monomialTemp
	powers                                                                    []MonomialPower
	renameTable                                                               monomialTable
	renameFound                                                               []bool
	locations                                                                 []monomialLocation
	creation                                                                  monomialCreation
	unit                                                                      Monomial
	origin, header                                                            uint64
	nextVar, managerRefs, publishedObjects                                    uint32
	idsInitialized                                                            bool
	allocatorOwned, memoryOwned, completed, closing, closed, sourceFinalizing bool
	memoryTraceLive                                                           bool
	allocatorMemoryTraceLive                                                  bool
	sourceStage                                                               uint8
	originalCause, cleanupCause                                               error
}

func NewMonomialManager(c MonomialManagerConfig) (*MonomialManager, error) {
	if c.Budget == nil {
		return nil, arena.ErrConfig
	}
	r := c.Memory
	owned := false
	if c.Allocator != nil {
		if _, e := c.Allocator.AllocationSize(); e != nil {
			return nil, e
		}
		if r == nil {
			r = c.Allocator.RuntimeOwner()
		}
		if r != c.Allocator.RuntimeOwner() {
			return nil, arena.ErrConfig
		}
	}
	if r != nil {
		if r.Budget() != c.Budget {
			return nil, arena.ErrConfig
		}
		if e := r.CheckOpen(); e != nil {
			return nil, e
		}
	} else {
		var e error
		r, e = umemory.NewRuntime(umemory.RuntimeConfig{Budget: c.Budget, SizeTBits: c.Profile.PointerBits})
		if e != nil {
			return nil, e
		}
		owned = true
	}
	if c.Profile.PointerBits == 0 {
		c.Profile = umemory.SmallAllocatorProfile{PointerBits: r.SizeTBits(), Debug: polynomialSourceDebug, Windows: runtime.GOOS == "windows"}
	}
	if c.Profile.PointerBits != r.SizeTBits() {
		if owned {
			_ = r.Close()
		}
		return nil, arena.ErrConfig
	}
	h := uint64(reflect.TypeOf(MonomialManager{}).Size())
	if e := c.Budget.Reserve(h); e != nil {
		if owned {
			if cleanup := r.Close(); cleanup != nil {
				return nil, errors.Join(e, cleanup)
			}
		}
		return nil, e
	}
	m := &MonomialManager{config: c, memory: r, memoryOwned: owned, header: h}
	m.self = m
	if c.polynomialImpAllocationLine != 0 {
		m.memoryTraceLive = true
		m.tracePolynomialImpAllocated()
	}
	for {
		old := monomialOrigins.Load()
		if old == math.MaxUint64 {
			c.Budget.Release(h)
			if owned {
				_ = r.Close()
			}
			return nil, arena.ErrSlotLimit
		}
		if monomialOrigins.CompareAndSwap(old, old+1) {
			m.origin = old + 1
			break
		}
	}
	fail := func(e error) (*MonomialManager, error) {
		m.originalCause = e
		m.cleanupCause = m.constructorUnwind()
		return m, m
	}
	m.ids = idgen.New(0, func() bool { return r.State().OutOfMemory })
	m.idsInitialized = true
	m.allocator = c.Allocator
	if m.allocator == nil {
		a, e := umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{
			Runtime: r, Profile: c.Profile, ChunkTrace: diagnostic.NewMemoryChunkTrace(c.Trace, r),
		})
		if e != nil {
			return fail(e)
		}
		m.allocator = a
		m.allocatorOwned = true
		m.allocatorMemoryTraceLive = true
		m.traceOwnedAllocatorAllocated()
	}
	unit, e := m.MkMonomial(nil)
	if e != nil {
		return fail(e)
	}
	m.unit = unit
	if e = m.incRefCore(unit); e != nil {
		return fail(e)
	}
	m.completed = true
	return m, nil
}
func (m *MonomialManager) Error() string {
	if m == nil {
		return "mpnum: failed monomial construction"
	}
	if m.originalCause != nil {
		return m.originalCause.Error()
	}
	if m.cleanupCause != nil {
		return m.cleanupCause.Error()
	}
	return ErrMonomialPending.Error()
}
func (m *MonomialManager) Unwrap() error {
	if m == nil {
		return nil
	}
	return m.originalCause
}
func (m *MonomialManager) OriginalCause() error {
	if m == nil {
		return nil
	}
	return m.originalCause
}
func (m *MonomialManager) CleanupCause() error {
	if m == nil {
		return nil
	}
	return m.cleanupCause
}
func (m *MonomialManager) check() error {
	if m.substitutionTraceCallback() || (m.exactDivTraceCallback() || m.lexTraceCallback()) {
		return ErrMonomialPending
	}
	if m != nil && m.self == m && (m.assertionRunning || m.managerTraceRunning) {
		return ErrMonomialPending
	}
	if m == nil || m.self != m {
		return ErrMonomialOwner
	}
	if m.closed || m.closing || m.sourceFinalizing || m.originalCause != nil {
		return arena.ErrClosed
	}
	return m.memory.CheckOpen()
}
func (m *MonomialManager) RuntimeOwner() *umemory.Runtime {
	if m == nil || m.self != m {
		return nil
	}
	return m.memory
}
func (m *MonomialManager) NumVars() (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	return m.nextVar, nil
}
func (m *MonomialManager) IsValidVar(v uint32) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	return v < m.nextVar, nil
}
func (m *MonomialManager) MkVar() (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	v := m.nextVar
	m.nextVar++
	return v, nil
}
func (m *MonomialManager) Unit() (Monomial, error) {
	if e := m.check(); e != nil {
		return Monomial{}, e
	}
	return m.unit, nil
}
func (m *MonomialManager) location(a Monomial) (*monomialLocation, error) {
	if a.owner != m.origin || a.slot == 0 || uint64(a.slot) > uint64(len(m.locations)) {
		return nil, ErrMonomialOwner
	}
	l := &m.locations[a.slot-1]
	if !l.live || l.generation != a.generation {
		return nil, ErrMonomialOwner
	}
	return l, nil
}
func (m *MonomialManager) headerCore(a Monomial) (MonomialInfo, error) {
	var out MonomialInfo
	l, e := m.location(a)
	if e != nil {
		return out, e
	}
	values := [5]*uint32{&out.RefCount, &out.ID, &out.TotalDegree, &out.Size, &out.Hash}
	for i, p := range values {
		v, e := m.allocator.ReadUint32(l.raw, uint64(i)*4)
		if e != nil {
			return out, e
		}
		*p = v
	}
	return out, nil
}
func (m *MonomialManager) Info(a Monomial) (MonomialInfo, error) {
	if e := m.check(); e != nil {
		return MonomialInfo{}, e
	}
	return m.headerCore(a)
}

// Hash is the retained monomial::hash() scalar getter (polynomial.cpp:210).
func (m *MonomialManager) Hash(a Monomial) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	l, e := m.location(a)
	if e != nil {
		return 0, e
	}
	return m.allocator.ReadUint32(l.raw, 16)
}
func (m *MonomialManager) powerCore(a Monomial, i uint32) (MonomialPower, error) {
	var out MonomialPower
	l, e := m.location(a)
	if e != nil {
		return out, e
	}
	if e := m.powerSourceIndexAssert(a, i); e != nil {
		return out, e
	}
	out.Var, e = m.allocator.ReadUint32(l.raw, 20+uint64(i)*8)
	if e != nil {
		return out, e
	}
	out.Degree, e = m.allocator.ReadUint32(l.raw, 24+uint64(i)*8)
	return out, e
}
func (m *MonomialManager) powerIndexGuard(n, i uint32) error {
	if i >= n {
		return arena.ErrBounds
	}
	return nil
}
func (m *MonomialManager) ReadPower(a Monomial, i uint32) (MonomialPower, error) {
	if e := m.check(); e != nil {
		return MonomialPower{}, e
	}
	return m.powerCore(a, i)
}
func (m *MonomialManager) prepareLocation() (uint32, error) {
	for i := range m.locations {
		l := &m.locations[i]
		if !l.live && l.generation < math.MaxUint32 {
			generation := l.generation + 1
			*l = monomialLocation{generation: generation}
			return uint32(i + 1), nil
		}
	}
	if uint64(len(m.locations)) == uint64(math.MaxUint32) {
		return 0, arena.ErrSlotLimit
	}
	if len(m.locations) == int(^uint(0)>>1) {
		return 0, arena.ErrBounds
	}
	m.locations = append(m.locations, monomialLocation{generation: 1})
	return uint32(len(m.locations)), nil
}
func (m *MonomialManager) incRefCore(a Monomial) error {
	l, e := m.location(a)
	if e != nil {
		return e
	}
	n, e := m.allocator.ReadUint32(l.raw, 0)
	if e != nil {
		return e
	}
	if e = m.allocator.WriteUint32(l.raw, 0, n+1); e != nil {
		return e
	}
	l.acknowledgeArithmeticProduct(a)
	l.acknowledgeDividesCreated(a)
	l.acknowledgeToPolynomialCreated(a)
	l.acknowledgePseudoCreated(a)
	if d := l.compositionCohort; d != nil && uint64(l.compositionSlot) < uint64(len(d.orphans)) && d.orphans[l.compositionSlot] == a {
		d.orphans[l.compositionSlot] = Monomial{}
		l.compositionCohort = nil
		l.compositionSlot = 0
	}
	if d := l.squareRootCohort; d != nil && uint64(l.squareRootSlot) < uint64(len(d.orphans)) && d.orphans[l.squareRootSlot] == a {
		d.orphans[l.squareRootSlot] = Monomial{}
		d.orphanInvocations[l.squareRootSlot] = 0
		l.squareRootCohort = nil
		l.squareRootSlot = 0
	}
	if d := l.conversionCohort; d != nil && uint64(l.conversionSlot) < uint64(len(d.orphans)) && d.orphans[l.conversionSlot] == a {
		d.orphans[l.conversionSlot] = Monomial{}
		l.conversionCohort = nil
		l.conversionSlot = 0
	}
	// Source INC is complete. The actual cohort slot alone grants ownership;
	// the cold location tag only routes this infallible, allocation-free transfer.
	if d := l.unreferencedCohort; d != nil && uint64(l.unreferencedSlot) < uint64(len(d.orphans)) && d.orphans[l.unreferencedSlot] == a {
		d.orphans[l.unreferencedSlot] = Monomial{}
		l.unreferencedCohort = nil
		l.unreferencedSlot = 0
	}

	if d := l.derivativeCohort; d != nil && uint64(l.derivativeSlot) < uint64(len(d.orphans)) && d.orphans[l.derivativeSlot] == a {
		d.orphans[l.derivativeSlot] = Monomial{}
		d.orphanInvocations[l.derivativeSlot] = 0
		l.derivativeCohort = nil
		l.derivativeSlot = 0
	}
	return nil
}
func (m *MonomialManager) IncRef(a Monomial) error {
	if e := m.check(); e != nil {
		return e
	}
	return m.incRefCore(a)
}
func (m *MonomialManager) DecRef(a Monomial) error {
	if e := m.check(); e != nil {
		return e
	}
	return m.decRefCore(a)
}

type monomialDecrementOutcome struct{ written, last bool }

func (m *MonomialManager) decRefCore(a Monomial) error { return m.decRefCoreOutcome(a, nil) }
func (m *MonomialManager) decRefCoreOutcome(a Monomial, out *monomialDecrementOutcome) error {
	// A borrowed trace writer can reenter through polynomial owners as well
	// as DecRef. Refuse at the shared decrement before publishing RC or any
	// table/allocator deletion prefix while the query still reads this owner.
	if m != nil && m.self == m && m.queryTraceRunning {
		return ErrMonomialPending
	}
	l, e := m.location(a)
	if e != nil {
		return e
	}
	if l.stage != 0 {
		return m.deleteCore(a, l)
	}
	n, e := m.allocator.ReadUint32(l.raw, 0)
	if e != nil {
		return e
	}
	if e = m.sourceScalarAssert(216, "m_ref_count > 0", n > 0); e != nil {
		return e
	}
	n--
	if e = m.allocator.WriteUint32(l.raw, 0, n); e != nil {
		return e
	}
	if out != nil {
		out.written = true
		out.last = n == 0
	}
	if n == 0 {
		return m.deleteCore(a, l)
	}
	return nil
}
func (m *MonomialManager) deleteCore(a Monomial, l *monomialLocation) error {
	if l.stage == 0 {
		n, e := m.allocator.ReadUint32(l.raw, 12)
		if e != nil {
			return e
		}
		l.requested = monomialObjectSize(n)
		if e := m.table.erase(m, monomialKey{kind: monomialObjectKey, slot: a.slot}); e != nil {
			return e
		}
		l.stage = 1
	}
	if l.stage == 1 {
		id, e := m.allocator.ReadUint32(l.raw, 4)
		if e != nil {
			return e
		}
		l.id = id
		if e := m.recycleID(l.id); e != nil {
			return e
		}
		l.stage = 2
	}
	if l.raw != (umemory.ObjectRef{}) {
		progress, e := m.allocator.DeallocateTracked(uint64(l.requested), l.raw)
		if progress.PhysicalReleased || progress.PooledLinked {
			l.raw = umemory.ObjectRef{}
		}
		if e != nil {
			return e
		}
	}
	if l.published {
		m.publishedObjects--
		l.published = false
	}
	l.live = false
	l.stage = 0
	return nil
}
