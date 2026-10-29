package sat

// Source: src/sat/sat_solver.cpp; src/sat/sat_clause.h.
// Concrete standalone storage translation of sat_clause.h/.cpp.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
	"reflect"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/idgen"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

var (
	ErrSATOwner     = errors.New("sat: foreign or copied storage owner")
	ErrSATAssertion = udiag.ErrDebugAssertion
	ErrSATKind      = errors.New("sat: invalid pointer-union kind")
)
var satStorageNextOwner atomic.Uint64

func satNextOwner() (uint64, error) {
	for {
		n := satStorageNextOwner.Load()
		if n == ^uint64(0) {
			return 0, arena.ErrSlotLimit
		}
		if satStorageNextOwner.CompareAndSwap(n, n+1) {
			return n + 1, nil
		}
	}
}

type StoredClauseRef struct {
	owner                         uint64
	slot, generation, storageKind uint32
}
type satClauseOffset StoredClauseRef
type satClauseHotHeader struct{ ID, Size, Capacity, Approx, Flags uint32 }
type satClauseAllocatorConfig struct {
	Storage        arena.SmallBytesConfig
	OutOfMemory    func() bool
	OOMRuntime     *umemory.Runtime
	PooledOverride bool
}
type ClauseAllocator struct {
	self                                   *ClauseAllocator
	traceTags                              *udiag.TraceTags
	origin                                 uint64
	storage                                *arena.SmallBytes
	ids                                    idgen.Generator
	oomRuntime                             *umemory.Runtime
	budget                                 *arena.Budget
	large                                  []satLargeClauseEntry
	largeFree                              uint32
	largeBytes, largeDirectory, ownerBytes uint64
	temporary, closed                      bool
}

func NewSATClauseAllocator(c satClauseAllocatorConfig) (*ClauseAllocator, error) {
	return newSATClauseAllocator(c, false)
}
func newSATClauseAllocator(c satClauseAllocatorConfig, tmp bool) (*ClauseAllocator, error) {
	if c.OOMRuntime != nil {
		if c.OutOfMemory != nil || c.OOMRuntime.Budget() != c.Storage.Descriptors.Budget {
			return nil, arena.ErrConfig
		}
		if _, e := c.OOMRuntime.AllocationSize(); e != nil {
			return nil, e
		}
	}

	if tmp || satStorageSourceDebug && !c.PooledOverride {
		c.Storage.DirectOnly = true
	}
	b := c.Storage.Descriptors.Budget
	n := uint64(reflect.TypeOf(ClauseAllocator{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	id, e := satNextOwner()
	if e != nil {
		b.Release(n)
		return nil, e
	}
	store, e := arena.NewSmallBytes(c.Storage)
	if e != nil {
		b.Release(n)
		return nil, e
	}
	a := &ClauseAllocator{origin: id, storage: store, budget: b, ids: idgen.New(0, c.OutOfMemory), oomRuntime: c.OOMRuntime, ownerBytes: n, temporary: tmp}
	a.self = a
	return a, nil
}
func (a *ClauseAllocator) open() error {
	if a == nil || a.self != a {
		return ErrSATOwner
	}
	if a.closed {
		return arena.ErrClosed
	}
	return nil
}
func satClauseObjectBytes(n uint32) (uint64, error) {
	return uint64(reflect.TypeOf(satClauseHotHeader{}).Size()) + 4*uint64(n), nil
}
func satClauseWideExtent(bytes uint64) bool { return bytes > uint64(^uint32(0)) }
func satClauseHostExtent(bytes, max uint64) error {
	if bytes > max {
		return arena.ErrBounds
	}
	return nil
}
func (a *ClauseAllocator) validate(r StoredClauseRef) error {
	if e := a.open(); e != nil {
		return e
	}
	if r.owner != a.origin {
		return ErrSATOwner
	}
	if r.storageKind == 0 {
		if !a.storage.IsLive(arena.SmallBytesRef{Slot: arena.Slot(r.slot), Generation: arena.Generation(r.generation)}) {
			return arena.ErrInvalidSlot
		}
	} else if r.storageKind == 1 {
		if uint64(r.slot) >= uint64(len(a.large)) || !a.large[r.slot].live || a.large[r.slot].generation != r.generation {
			return arena.ErrInvalidSlot
		}
	} else {
		return arena.ErrInvalidSlot
	}
	return nil
}
func (a *ClauseAllocator) IsLive(r StoredClauseRef) bool { return a.validate(r) == nil }
func (a *ClauseAllocator) readWord(r StoredClauseRef, i uint64) (uint32, error) {
	if e := a.validate(r); e != nil {
		return 0, e
	}
	if r.storageKind == 1 {
		if i >= uint64(len(a.large[r.slot].data)) {
			return 0, arena.ErrBounds
		}
		return a.large[r.slot].data[i], nil
	}
	if i > uint64(^uint32(0))/4 {
		return 0, arena.ErrBounds
	}
	var v uint32
	ref := arena.SmallBytesRef{Slot: arena.Slot(r.slot), Generation: arena.Generation(r.generation)}
	for j := uint32(0); j < 4; j++ {
		x, e := a.storage.Read(ref, uint32(i*4)+j)
		if e != nil {
			return 0, e
		}
		v |= uint32(x) << (8 * j)
	}
	return v, nil
}
func (a *ClauseAllocator) writeWord(r StoredClauseRef, i uint64, v uint32) error {
	if e := a.validate(r); e != nil {
		return e
	}
	if r.storageKind == 1 {
		if i >= uint64(len(a.large[r.slot].data)) {
			return arena.ErrBounds
		}
		a.large[r.slot].data[i] = v
		return nil
	}
	if i > uint64(^uint32(0))/4 {
		return arena.ErrBounds
	}
	ref := arena.SmallBytesRef{Slot: arena.Slot(r.slot), Generation: arena.Generation(r.generation)}
	for j := uint32(0); j < 4; j++ {
		if e := a.storage.Write(ref, uint32(i*4)+j, byte(v>>(8*j))); e != nil {
			return e
		}
	}
	return nil
}
func (a *ClauseAllocator) allocate(n uint32) (StoredClauseRef, error) {
	bytes, _ := satClauseObjectBytes(n)
	if e := satClauseHostExtent(bytes, uint64(^uint(0)>>1)); e != nil {
		return StoredClauseRef{}, e
	}
	if satClauseWideExtent(bytes) {
		return a.allocateLarge(bytes)
	}
	r, e := a.storage.Allocate(uint32(bytes))
	if e != nil {
		return StoredClauseRef{}, e
	}
	return StoredClauseRef{a.origin, uint32(r.Slot), uint32(r.Generation), 0}, nil
}
func (a *ClauseAllocator) construct(r StoredClauseRef, id, n uint32, lits []Literal, learned bool) error {
	return a.constructSpan(r, id, n, satSolverLiteralSpan{literals: lits}, learned)
}
func (a *ClauseAllocator) constructSpan(r StoredClauseRef, id, n uint32, span satSolverLiteralSpan, learned bool) error {
	flags := uint32(0)
	if learned {
		flags = 4
	}
	for i, v := range [5]uint32{id, n, n, 0, flags} {
		if e := a.writeWord(r, uint64(i), v); e != nil {
			return e
		}
	}
	if span.column == nil && span.store == nil && uint64(len(span.literals)) < uint64(n) {
		return arena.ErrBounds
	}
	for i := uint32(0); i < n; i++ {
		lit, e := span.literal(i)
		if e != nil {
			return e
		}
		if e := a.writeWord(r, uint64(i)+5, uint32(lit)); e != nil {
			return e
		}
	}
	if e := a.MarkStrengthened(r); e != nil {
		return e
	}
	if satStorageSourceDebug {
		if _, e := a.CheckApprox(r); e != nil {
			return e
		}
		if n <= 1 {
			return ErrSATAssertion
		}
	}
	return nil
}
func (a *ClauseAllocator) MkClause(lits []Literal, learned bool) (StoredClauseRef, error) {
	if uint64(len(lits)) > uint64(^uint32(0)) {
		return StoredClauseRef{}, arena.ErrBounds
	}
	return a.MkClauseN(uint32(len(lits)), lits, learned)
}

// A constructor error after storage/ID publication returns its owned ref so the
// caller can inspect/reclaim the completed prefix; no hidden rollback of IDs.
func (a *ClauseAllocator) MkClauseN(n uint32, lits []Literal, learned bool) (StoredClauseRef, error) {
	return a.mkClauseSpan(n, satSolverLiteralSpan{literals: lits}, learned)
}
func (a *ClauseAllocator) mkClauseSpan(n uint32, span satSolverLiteralSpan, learned bool) (StoredClauseRef, error) {
	return a.mkClauseSpanPrefix(n, span, learned, nil)
}

// Scalar construction stage accompanies the real returned allocation, without
// inventing readable header fields after a failed ID free-vector read.
type satClauseConstructionPrefix struct{ unconstructed bool }

func (a *ClauseAllocator) mkClauseSpanPrefix(n uint32, span satSolverLiteralSpan, learned bool, prefix *satClauseConstructionPrefix) (StoredClauseRef, error) {
	if e := a.open(); e != nil {
		return StoredClauseRef{}, e
	}
	r, e := a.allocate(n)
	if e != nil {
		return r, e
	}
	if prefix != nil {
		prefix.unconstructed = true
	}
	id := uint32(^uint32(0))
	if !a.temporary {
		id = a.ids.Mk()
	}
	if prefix != nil {
		prefix.unconstructed = false
	}
	if e := a.constructSpan(r, id, n, span, learned); e != nil {
		return r, e
	}
	a.traceAllocated(r, learned)
	return r, nil
}
func (a *ClauseAllocator) release(r StoredClauseRef) error {
	if e := a.validate(r); e != nil {
		return e
	}
	if r.storageKind == 1 {
		return a.releaseLarge(r)
	}
	cap, e := a.Capacity(r)
	if e != nil {
		return e
	}
	n, _ := satClauseObjectBytes(cap)
	return a.storage.Deallocate(uint32(n), arena.SmallBytesRef{Slot: arena.Slot(r.slot), Generation: arena.Generation(r.generation)})
}
func (a *ClauseAllocator) Delete(r StoredClauseRef) error {
	if e := a.validate(r); e != nil {
		return e
	}
	a.traceDeleted(r)
	if !a.temporary {
		id, e := a.ID(r)
		if e != nil {
			return e
		}
		if e = a.recycleID(id); e != nil {
			return e
		}
	}
	return a.release(r)
}
func (a *ClauseAllocator) Offset(r StoredClauseRef) (satClauseOffset, error) {
	if e := a.validate(r); e != nil {
		return satClauseOffset{}, e
	}
	return satClauseOffset(r), nil
}
func (a *ClauseAllocator) GetClause(o satClauseOffset) (StoredClauseRef, error) {
	r := StoredClauseRef(o)
	return r, a.validate(r)
}
func (a *ClauseAllocator) ID(r StoredClauseRef) (uint32, error)       { return a.readWord(r, 0) }
func (a *ClauseAllocator) Size(r StoredClauseRef) (uint32, error)     { return a.readWord(r, 1) }
func (a *ClauseAllocator) Capacity(r StoredClauseRef) (uint32, error) { return a.readWord(r, 2) }
func (a *ClauseAllocator) Approx(r StoredClauseRef) (satApproxSet, error) {
	n, e := a.readWord(r, 3)
	return satApproxSet(n), e
}
func (a *ClauseAllocator) Literal(r StoredClauseRef, i uint32) (Literal, error) {
	n, e := a.Size(r)
	if e != nil {
		return 0, e
	}
	if satStorageSourceDebug && i >= n {
		return 0, ErrSATAssertion
	}
	capacity, e := a.Capacity(r)
	if e != nil {
		return 0, e
	}
	if i >= capacity {
		return 0, arena.ErrBounds
	}
	v, e := a.readWord(r, 5+uint64(i))
	return Literal(v), e
}
func (a *ClauseAllocator) SetLiteral(r StoredClauseRef, i uint32, l Literal) error {
	n, e := a.Size(r)
	if e != nil {
		return e
	}
	if satStorageSourceDebug && i >= n {
		return ErrSATAssertion
	}
	capacity, e := a.Capacity(r)
	if e != nil {
		return e
	}
	if i >= capacity {
		return arena.ErrBounds
	}
	return a.writeWord(r, 5+uint64(i), uint32(l))
}
func (a *ClauseAllocator) flag(r StoredClauseRef, mask uint32) (bool, error) {
	v, e := a.readWord(r, 4)
	return v&mask != 0, e
}
func (a *ClauseAllocator) setFlag(r StoredClauseRef, mask uint32, v bool) error {
	old, e := a.readWord(r, 4)
	if e != nil {
		return e
	}
	old &^= mask
	if v {
		old |= mask
	}
	return a.writeWord(r, 4, old)
}
func (a *ClauseAllocator) Learned(r StoredClauseRef) (bool, error)       { return a.flag(r, 4) }
func (a *ClauseAllocator) Strengthened(r StoredClauseRef) (bool, error)  { return a.flag(r, 1) }
func (a *ClauseAllocator) Removed(r StoredClauseRef) (bool, error)       { return a.flag(r, 2) }
func (a *ClauseAllocator) Used(r StoredClauseRef) (bool, error)          { return a.flag(r, 8) }
func (a *ClauseAllocator) Frozen(r StoredClauseRef) (bool, error)        { return a.flag(r, 16) }
func (a *ClauseAllocator) OnReinitStack(r StoredClauseRef) (bool, error) { return a.flag(r, 32) }
func (a *ClauseAllocator) SetRemoved(r StoredClauseRef, b bool) error {
	return a.setFlag(r, 2, b)
}
func (a *ClauseAllocator) MarkUsed(r StoredClauseRef) error   { return a.setFlag(r, 8, true) }
func (a *ClauseAllocator) UnmarkUsed(r StoredClauseRef) error { return a.setFlag(r, 8, false) }
func (a *ClauseAllocator) SetReinitStack(r StoredClauseRef, b bool) error {
	return a.setFlag(r, 32, b)
}
func (a *ClauseAllocator) UnmarkStrengthened(r StoredClauseRef) error {
	return a.setFlag(r, 1, false)
}
func (a *ClauseAllocator) UnsetLearned(r StoredClauseRef) error {
	v, e := a.Learned(r)
	if e != nil {
		return e
	}
	if satStorageSourceDebug && !v {
		return ErrSATAssertion
	}
	return a.setFlag(r, 4, false)
}
func (a *ClauseAllocator) Freeze(r StoredClauseRef) error {
	l, e := a.Learned(r)
	if e != nil {
		return e
	}
	f, e := a.Frozen(r)
	if e != nil {
		return e
	}
	if satStorageSourceDebug && (!l || f) {
		return ErrSATAssertion
	}
	return a.setFlag(r, 16, true)
}
func (a *ClauseAllocator) Unfreeze(r StoredClauseRef) error {
	l, e := a.Learned(r)
	if e != nil {
		return e
	}
	f, e := a.Frozen(r)
	if e != nil {
		return e
	}
	if satStorageSourceDebug && (!l || !f) {
		return ErrSATAssertion
	}
	return a.setFlag(r, 16, false)
}
func SATApproxLiterals(ls []Literal) satApproxSet {
	var m satApproxSet
	for _, l := range ls {
		m.insert(uint32(l.variable()))
	}
	return m
}
func (a *ClauseAllocator) UpdateApprox(r StoredClauseRef) error {
	n, e := a.Size(r)
	if e != nil {
		return e
	}
	var m satApproxSet
	for i := uint32(0); i < n; i++ {
		l, e := a.Literal(r, i)
		if e != nil {
			return e
		}
		m.insert(uint32(l.variable()))
	}
	return a.writeWord(r, 3, uint32(m))
}
func (a *ClauseAllocator) MarkStrengthened(r StoredClauseRef) error {
	if e := a.setFlag(r, 1, true); e != nil {
		return e
	}
	return a.UpdateApprox(r)
}
func (a *ClauseAllocator) CheckApprox(r StoredClauseRef) (bool, error) {
	old, e := a.Approx(r)
	if e != nil {
		return false, e
	}
	if e = a.UpdateApprox(r); e != nil {
		return false, e
	}
	m, e := a.Approx(r)
	if e != nil {
		return false, e
	}
	if satStorageSourceDebug && old != m {
		return false, ErrSATAssertion
	}
	return true, nil
}
func (a *ClauseAllocator) Shrink(r StoredClauseRef, n uint32) error {
	old, e := a.Size(r)
	if e != nil {
		return e
	}
	if satStorageSourceDebug && n > old {
		return ErrSATAssertion
	}
	if n < old {
		if e = a.writeWord(r, 1, n); e != nil {
			return e
		}
		return a.MarkStrengthened(r)
	}
	return nil
}
func (a *ClauseAllocator) Elim(r StoredClauseRef, l Literal) error {
	n, e := a.Size(r)
	if e != nil {
		return e
	}
	i := uint32(0)
	for ; i < n; i++ {
		v, e := a.Literal(r, i)
		if e != nil {
			return e
		}
		if v == l {
			break
		}
	}
	if satStorageSourceDebug && i == n {
		return ErrSATAssertion
	}
	if n == 0 {
		return arena.ErrBounds
	}
	if e := satClauseElimCopyStart(n, i); e != nil {
		return e
	}
	for j := uint64(i) + 1; j < uint64(n); j++ {
		v, e := a.Literal(r, uint32(j))
		if e != nil {
			return e
		}
		if e = a.SetLiteral(r, uint32(j-1), v); e != nil {
			return e
		}
	}
	if e = a.writeWord(r, 1, n-1); e != nil {
		return e
	}
	return a.MarkStrengthened(r)
}
func (a *ClauseAllocator) ContainsLiteral(r StoredClauseRef, l Literal) (bool, error) {
	n, e := a.Size(r)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		v, e := a.Literal(r, i)
		if e != nil {
			return false, e
		}
		if v == l {
			return true, nil
		}
	}
	return false, nil
}
func (a *ClauseAllocator) ContainsVar(r StoredClauseRef, v BoolVar) (bool, error) {
	n, e := a.Size(r)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		l, e := a.Literal(r, i)
		if e != nil {
			return false, e
		}
		if l.variable() == v {
			return true, nil
		}
	}
	return false, nil
}
func (a *ClauseAllocator) SatisfiedBy(r StoredClauseRef, m Model) (bool, error) {
	return a.satisfiedByModelSpan(r, satSolverModelSpan{slice: m})
}
func (a *ClauseAllocator) satisfiedByModelSpan(r StoredClauseRef, m satSolverModelSpan) (bool, error) {
	n, e := a.Size(r)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		l, e := a.Literal(r, i)
		if e != nil {
			return false, e
		}
		v, e := m.read(l.variable())
		if e != nil {
			return false, e
		}
		if l.sign() && v == lbool.False || !l.sign() && v == lbool.True {
			return true, nil
		}
	}
	return false, nil
}
func (a *ClauseAllocator) lane(r StoredClauseRef, shift uint32) (uint32, error) {
	v, e := a.readWord(r, 4)
	return v >> shift & 255, e
}
func (a *ClauseAllocator) setLane(r StoredClauseRef, shift, v uint32) error {
	old, e := a.readWord(r, 4)
	if e != nil {
		return e
	}
	return a.writeWord(r, 4, old&^(255<<shift)|((v&255)<<shift))
}
func (a *ClauseAllocator) IncInactivity(r StoredClauseRef) error {
	v, e := a.Inactivity(r)
	if e != nil {
		return e
	}
	return a.setLane(r, 6, v+1)
}
func (a *ClauseAllocator) ResetInactivity(r StoredClauseRef) error      { return a.setLane(r, 6, 0) }
func (a *ClauseAllocator) Inactivity(r StoredClauseRef) (uint32, error) { return a.lane(r, 6) }
func (a *ClauseAllocator) SetGlue(r StoredClauseRef, v uint32) error {
	if v > 255 {
		v = 255
	}
	return a.setLane(r, 14, v)
}
func (a *ClauseAllocator) Glue(r StoredClauseRef) (uint32, error) { return a.lane(r, 14) }
func (a *ClauseAllocator) SetPSM(r StoredClauseRef, v uint32) error {
	if v > 255 {
		v = 255
	}
	return a.setLane(r, 22, v)
}
func (a *ClauseAllocator) PSM(r StoredClauseRef) (uint32, error) { return a.lane(r, 22) }
func (a *ClauseAllocator) AppendLiterals(r StoredClauseRef, dst []Literal) ([]Literal, error) {
	n, e := a.Size(r)
	if e != nil {
		return dst, e
	}
	for i := uint32(0); i < n; i++ {
		l, e := a.Literal(r, i)
		if e != nil {
			return dst, e
		}
		dst = append(dst, l)
	}
	return dst, nil
}
func satLiteralInto(out *utext.NumericTextStream, l Literal) error {
	sign := ""
	if l.sign() {
		sign = "-"
	}
	if e := out.AppendCString(sign); e != nil {
		return e
	}
	return out.AppendUint32(uint32(l.variable()))
}
func (a *ClauseAllocator) DisplayInto(r StoredClauseRef, out *utext.NumericTextStream) error {
	n, e := a.Size(r)
	if e != nil {
		return e
	}
	if e = out.AppendCString("("); e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		if i != 0 {
			if e = out.AppendCString(" "); e != nil {
				return e
			}
		}
		l, e := a.Literal(r, i)
		if e != nil {
			return e
		}
		if e = satLiteralInto(out, l); e != nil {
			return e
		}
	}
	if e = out.AppendCString(")"); e != nil {
		return e
	}
	for _, flag := range []struct {
		mask uint32
		text string
	}{{2, "x"}, {1, "+"}, {4, "*"}} {
		v, e := a.flag(r, flag.mask)
		if e != nil {
			return e
		}
		if v {
			if e = out.AppendCString(flag.text); e != nil {
				return e
			}
		}
	}
	return nil
}
func (a *ClauseAllocator) Close() error {
	if a == nil || a.self != a {
		return ErrSATOwner
	}
	if a.closed {
		return nil
	}
	for i := range a.large {
		if a.large[i].live {
			a.budget.Release(uint64(len(a.large[i].data)) * 4)
			a.large[i].data = nil
		}
	}
	a.budget.Release(a.largeDirectory + a.ownerBytes)
	a.large = nil
	a.largeDirectory = 0
	a.largeBytes = 0
	a.ownerBytes = 0
	a.ids.Cleanup(0)
	a.storage.Close()
	a.closed = true
	return nil
}

type satTmpClause struct {
	self         *satTmpClause
	owner        *ClauseAllocator
	ref, pending StoredClauseRef
	budget       *arena.Budget
	ownerBytes   uint64
	closed       bool
}

func NewSATTmpClause(b *arena.Budget) (*satTmpClause, error) {
	n := uint64(reflect.TypeOf(satTmpClause{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	a, e := newSATClauseAllocator(satClauseAllocatorConfig{Storage: arena.SmallBytesConfig{Descriptors: arena.Config{Budget: b}}}, true)
	if e != nil {
		b.Release(n)
		return nil, e
	}
	t := &satTmpClause{owner: a, budget: b, ownerBytes: n}
	t.self = t
	return t, nil
}
func (t *satTmpClause) open() error {
	if t == nil || t.self != t {
		return ErrSATOwner
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.pending != (StoredClauseRef{}) {
		return ErrSATAssertion
	}
	return nil
}
func (t *satTmpClause) Get() StoredClauseRef { return t.ref }
func (t *satTmpClause) Set(ls []Literal, learned bool) error {
	if uint64(len(ls)) > uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	return t.SetN(uint32(len(ls)), ls, learned)
}
func (t *satTmpClause) SetN(n uint32, ls []Literal, learned bool) error {
	if e := t.open(); e != nil {
		return e
	}
	if t.ref != (StoredClauseRef{}) {
		cap, e := t.owner.Capacity(t.ref)
		if e != nil {
			return e
		}
		if cap < n {
			if e = t.owner.release(t.ref); e != nil {
				return e
			}
			t.ref = StoredClauseRef{}
		}
	}
	if t.ref == (StoredClauseRef{}) {
		r, e := t.owner.MkClauseN(n, ls, learned)
		if e != nil {
			t.pending = r
			return e
		}
		t.ref = r
		return nil
	}
	id, e := t.owner.ID(t.ref)
	if e != nil {
		return e
	}
	if satStorageSourceDebug && id != ^uint32(0) {
		return ErrSATAssertion
	}
	if e = t.owner.writeWord(t.ref, 1, n); e != nil {
		return e
	}
	if e = t.owner.setFlag(t.ref, 4, learned); e != nil {
		return e
	}
	if uint64(len(ls)) < uint64(n) {
		return arena.ErrBounds
	}
	for i := uint32(0); i < n; i++ {
		if e = t.owner.SetLiteral(t.ref, i, ls[i]); e != nil {
			return e
		}
	}
	return nil
}
func (t *satTmpClause) SetBinary(a, b Literal, l bool) error {
	v := [2]Literal{a, b}
	return t.Set(v[:], l)
}
func (t *satTmpClause) SetBinClause(b satBinClause) error {
	return t.SetBinary(b.Literal1(), b.Literal2(), b.Learned())
}
func (t *satTmpClause) Close() error {
	if t == nil || t.self != t {
		return ErrSATOwner
	}
	if t.closed {
		return nil
	}
	t.owner.Close()
	t.budget.Release(t.ownerBytes)
	t.ownerBytes = 0
	t.ref = StoredClauseRef{}
	t.pending = StoredClauseRef{}
	t.closed = true
	return nil
}

type satBinClause struct{ val1, val2 uint32 }

func newSATBinClause(a, b Literal, l bool) satBinClause {
	n := uint32(0)
	if l {
		n = 1
	}
	return satBinClause{uint32(a), (uint32(b) << 1) + n}
}
func (b satBinClause) Literal1() Literal { return Literal(b.val1) }
func (b satBinClause) Literal2() Literal { return Literal(b.val2 >> 1) }
func (b satBinClause) Learned() bool     { return b.val2&1 == 1 }
func (b satBinClause) Equal(c satBinClause) bool {
	return b.val1 == c.val1 && b.val2 == c.val2 || b.val1 == c.val2 && b.val2 == c.val1
}

type satClauseWrapper struct {
	ref           StoredClauseRef
	first, second Literal
}

func newSATClauseWrapper(r StoredClauseRef) satClauseWrapper {
	return satClauseWrapper{ref: r, second: DefaultLiteral()}
}
func newSATBinaryWrapper(a, b Literal) satClauseWrapper {
	return satClauseWrapper{first: a, second: b}
}
func (w satClauseWrapper) Binary() bool { return w.second != DefaultLiteral() }
func (w satClauseWrapper) Size(a *ClauseAllocator) (uint32, error) {
	if w.Binary() {
		return 2, nil
	}
	if w.ref.owner == 0 {
		return 0, ErrSATKind
	}
	return a.Size(w.ref)
}
func (w satClauseWrapper) Literal(a *ClauseAllocator, i uint32) (Literal, error) {
	if w.Binary() {
		if satStorageSourceDebug && i > 1 {
			return 0, ErrSATAssertion
		}
		if i == 0 {
			return w.first, nil
		}
		return w.second, nil
	}
	if w.ref.owner == 0 {
		return 0, ErrSATKind
	}
	return a.Literal(w.ref, i)
}
func (w satClauseWrapper) ClauseRef() (StoredClauseRef, error) {
	if w.Binary() {
		return StoredClauseRef{}, ErrSATKind
	}
	if w.ref.owner == 0 {
		return StoredClauseRef{}, ErrSATKind
	}
	return w.ref, nil
}
func (w satClauseWrapper) ContainsLiteral(a *ClauseAllocator, l Literal) (bool, error) {
	n, e := w.Size(a)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		v, e := w.Literal(a, i)
		if e != nil {
			return false, e
		}
		if v == l {
			return true, nil
		}
	}
	return false, nil
}
func (w satClauseWrapper) ContainsVar(a *ClauseAllocator, v BoolVar) (bool, error) {
	n, e := w.Size(a)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		l, e := w.Literal(a, i)
		if e != nil {
			return false, e
		}
		if l.variable() == v {
			return true, nil
		}
	}
	return false, nil
}
func (w satClauseWrapper) DisplayInto(a *ClauseAllocator, out *utext.NumericTextStream) error {
	n, e := w.Size(a)
	if e != nil {
		return e
	}
	if e = out.AppendCString("("); e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		if i > 0 {
			if e = out.AppendCString(" "); e != nil {
				return e
			}
		}
		l, e := w.Literal(a, i)
		if e != nil {
			return e
		}
		if e = satLiteralInto(out, l); e != nil {
			return e
		}
	}
	return out.AppendCString(")")
}

// Elim increments the uint32 search index before entering its copy loop.
func satClauseElimCopyStart(size, index uint32) error {
	if size == ^uint32(0) && index == size {
		return arena.ErrBounds
	}
	return nil
}

// Iterators are caller-owned values; only scalar locators are retained
// by storage. No mutable slice aliases escape.
type satClauseLiteralIterator struct {
	owner      *ClauseAllocator
	ref        StoredClauseRef
	index, end uint32
}

func (a *ClauseAllocator) LiteralIter(r StoredClauseRef) (satClauseLiteralIterator, error) {
	n, e := a.Size(r)
	if e != nil {
		return satClauseLiteralIterator{}, e
	}
	return satClauseLiteralIterator{owner: a, ref: r, end: n}, nil
}
func (i *satClauseLiteralIterator) Next() (Literal, bool, error) {
	if i == nil {
		return 0, false, ErrSATOwner
	}
	if e := i.owner.validate(i.ref); e != nil {
		return 0, false, e
	}
	if i.index >= i.end {
		return 0, false, nil
	}
	v, e := i.owner.readWord(i.ref, 5+uint64(i.index))
	if e != nil {
		return 0, false, e
	}
	i.index++
	return Literal(v), true, nil
}
func SATApproxLiteralsN(n uint32, ls []Literal) (satApproxSet, error) {
	var m satApproxSet
	for i := uint32(0); i < n; i++ {
		if uint64(i) >= uint64(len(ls)) {
			return m, arena.ErrBounds
		}
		m.insert(uint32(ls[i].variable()))
	}
	return m, nil
}

type satClauseAllocatorStats struct {
	OwnerBytes, IDFreeBytes, WideDirectoryBytes, WidePayloadBytes uint64
	SmallBytes                                                    arena.SmallBytesStats
}

func (a *ClauseAllocator) Stats() satClauseAllocatorStats {
	if a == nil {
		return satClauseAllocatorStats{}
	}
	return satClauseAllocatorStats{OwnerBytes: a.ownerBytes, WideDirectoryBytes: a.largeDirectory, WidePayloadBytes: a.largeBytes, SmallBytes: a.storage.Stats()}
}
func (w satWatched) NotEqual(v satWatched) bool { return !w.Equal(v) }

// The source memory OOM check precedes free-list access. Standalone allocators
// use their supplied callback; solver allocators borrow the one Runtime state
// provider; native ID slice growth does not request arena storage.
func (a *ClauseAllocator) recycleID(id uint32) error {
	if a.oomRuntime != nil {
		if _, e := a.oomRuntime.AllocationSize(); e != nil {
			return e
		}
		if a.oomRuntime.State().OutOfMemory {
			return nil
		}
	}
	if e := a.ids.Recycle(id); e != nil {
		if a.oomRuntime != nil && (errors.Is(e, arena.ErrBudget) || errors.Is(e, arena.ErrInjectedFailure)) {
			return a.oomRuntime.RecordAllocationFailure()
		}
		return e
	}
	return nil
}

// A later construction action can refuse after physical object allocation but
// before placement completes. The caller's ref and requested size remain the
// owning root. This explicit abort does not invent readable source headers or
// run source del_clause on an unconstructed clause.
func (a *ClauseAllocator) cleanupAllocated(r StoredClauseRef, n uint32) error {
	if e := a.validate(r); e != nil {
		return e
	}
	if r.storageKind == 1 {
		return a.releaseLarge(r)
	}
	extent, e := satClauseObjectBytes(n)
	if e != nil {
		return e
	}
	return a.storage.Deallocate(uint32(extent), arena.SmallBytesRef{Slot: arena.Slot(r.slot), Generation: arena.Generation(r.generation)})
}
