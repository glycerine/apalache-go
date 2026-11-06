package numeral

// Source: src/util/rational.h; src/util/rational.cpp.
// Concrete owner-local core of src/util/rational.h/.cpp and vector.h.

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrRationalServiceOwner = errors.New("mpnum: rational service provenance mismatch")
var ErrRationalServiceCopy = errors.New("mpnum: shallow rational service copy")

// Only this scalar namespace is global; IDs never wrap or get reused.
var rationalServiceNextOwnerID uint64

// OwnedRational is a pointer-free owning token; its zero value is invalid.
// Never shallow-copy an owning token: use Copy/CopyFrom/Move/Swap. Provenance
// identifies the service domain, not individual numeral lifetimes. Service
// Close invalidates every outstanding value from that domain.
type OwnedRational struct {
	Raw     Rational
	ownerID uint64
}
type RationalServiceConfig struct{ Integers IntegerConfig }
type RationalServiceStats struct {
	CacheLength, CacheCapacity uint32
	HeaderBytes                uint64
	LiveCells, LivePayloads    uint32
	// LivePayloads includes actual numeric objects (including a pending cell)
	// and genuine Natural buffers. It is not a count of raw allocator blocks.
	LiveNumericObjects, PendingCellObjects, RetainedCellDescriptors, NaturalPayloads uint32
}

// RationalService owns the source synchronized manager, constants and cache.
// This mutex serializes all manager actions. A shared borrowed Budget still
// requires external owner confinement; independent service locks do not make
// the Budget safe for concurrent use. Never shallow-copy this owner.
type RationalService struct {
	numericBorrow                                         RationalNumericBorrow
	numericReceiver                                       atomic.Value // rationalNumericReceiver; actual synchronous allocator owner
	lexicalPrepared, lexicalExecuting, lexicalInvocations *RationalServiceInvocation
	lexicalSerial                                         uint64
	lexicalGeneration                                     uint64 // actual accepted outer lexical entry
	sourceRuntimeExecuting                                uint32 // actual held lower-allocation invocation stamp

	sourceInsertionExecuting bool

	Lexical                                              rationalLexicalScope
	mu                                                   sync.Mutex
	Manager                                              *RationalManager
	self                                                 *RationalService
	ownerID                                              uint64
	ZeroField, OneField, MinusOneField                   Rational
	InfZeroField, InfOneField, InfMinusOneField          OwnedInfRational
	InfIntZeroField, InfIntOneField, InfIntMinusOneField OwnedInfIntRational
	Cache                                                []Rational
	HeaderBytes                                          uint64
	closed                                               bool
	originalCause, cleanupCause                          error
	cleanupStage                                         uint8
	cleanupCacheIndex                                    uint32
	cleanupComponent                                     uint8
	closing                                              bool
	sourceFinalizing, sourceFinalized, sourceGuard       bool
	sourceCause                                          error
	initializationConfig                                 RationalServiceConfig
	initializationBudget                                 *arena.Budget
	initializationFailure                                *IntegerManager
	initializationCause                                  error
	initializationStage                                  uint8
	sourceGlobalSlot                                     **RationalManager
	sourceEndedStatics                                   uint16
	sourceGlobalStopped                                  bool
}

func nextRationalServiceID() (uint64, error) {
	for {
		old := atomic.LoadUint64(&rationalServiceNextOwnerID)
		if old == math.MaxUint64 {
			return 0, arena.ErrBounds
		}
		if atomic.CompareAndSwapUint64(&rationalServiceNextOwnerID, old, old+1) {
			return old + 1, nil
		}
	}
}
func NewRationalService(c RationalServiceConfig) (*RationalService, error) {
	s, e := PrepareRationalService(c)
	if e != nil {
		return nil, e
	}
	if e = s.InitializeSource(); e != nil {
		if s.Manager == nil {
			// The enclosing constructor was never completed. Its header is consumed;
			// any actual failed child constructor remains the returned owning receipt.
			s.initializationFailure = nil
			s.initializationBudget.Release(s.HeaderBytes)
			s.HeaderBytes = 0
			s.closed = true
			return nil, e
		}
		s.originalCause = e
		if cleanup := s.CloseChecked(); cleanup != nil {
			return nil, s
		}
		return nil, e
	}
	return s, nil
}
func (s *RationalService) OwnerID() uint64 {
	if s == nil {
		return 0
	}
	return s.ownerID
}
func (s *RationalService) lock() error {
	if s == nil || s.self != s {
		return ErrRationalServiceCopy
	}
	if s.sourceInsertionExecuting || s.runtimeTransportExecuting() {
		return arena.ErrInvalidSlot
	}
	s.mu.Lock()
	if s.closed || s.closing || s.sourceFinalizing || s.initializationStage != 6 {
		s.mu.Unlock()
		return arena.ErrClosed
	}
	return nil
}
func (s *RationalService) check(v *OwnedRational) error {
	if v == nil || v.ownerID == 0 || v.ownerID != s.ownerID {
		return ErrRationalServiceOwner
	}
	return s.Manager.check(&v.Raw)
}
func (s *RationalService) NewValue() (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	return OwnedRational{Raw: NewRational(), ownerID: s.ownerID}, nil
}
func (s *RationalService) Del(v *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if v == nil || v.ownerID == 0 || v.ownerID != s.ownerID {
		return ErrRationalServiceOwner
	}
	if e := s.Manager.checkCleanup(&v.Raw); e != nil {
		return e
	}
	if e := s.Manager.Del(&v.Raw); e != nil {
		return e
	}
	*v = OwnedRational{}
	return nil
}
func (s *RationalService) Copy(dst, src *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return s.copyLockedWithContext(dst, src, integerAllocationInvocationContext{})
}
func (s *RationalService) copyLockedWithContext(dst, src *OwnedRational, context integerAllocationInvocationContext) error {
	if e := s.rationalScalarOwner(dst); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(src); e != nil {
		return e
	}
	return s.Manager.copyWithAllocationContext(&dst.Raw, &src.Raw, context)
}
func (s *RationalService) CopyFrom(dst *OwnedRational, source *RationalService, src *OwnedRational) error {
	if s == nil || s.self != s || source == nil || source.self != source {
		return ErrRationalServiceCopy
	}
	if s == source {
		return s.Copy(dst, src)
	}
	first, second := s, source
	if first.ownerID > second.ownerID {
		first, second = second, first
	}
	if first.sourceInsertionExecuting || second.sourceInsertionExecuting || first.runtimeTransportExecuting() || second.runtimeTransportExecuting() {
		return arena.ErrInvalidSlot
	}
	first.mu.Lock()
	defer first.mu.Unlock()
	second.mu.Lock()
	defer second.mu.Unlock()
	if s.closed || source.closed || s.initializationStage != 6 || source.initializationStage != 6 {
		return arena.ErrClosed
	}
	if e := s.check(dst); e != nil {
		return e
	}
	if e := source.check(src); e != nil {
		return e
	}
	return s.Manager.CopyFrom(&dst.Raw, source.Manager, &src.Raw)
}
func (s *RationalService) Move(dst, src *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.check(dst); e != nil {
		return e
	}
	if e := s.check(src); e != nil {
		return e
	}
	return s.Manager.Move(&dst.Raw, &src.Raw)
}
func (s *RationalService) Swap(a, b *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return e
	}
	return s.Manager.Swap(&a.Raw, &b.Raw)
}
func (s *RationalService) SetInt64(v *OwnedRational, n int64) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.check(v); e != nil {
		return e
	}
	return s.Manager.SetInt64(&v.Raw, n)
}

// SetUnsigned translates the source unsigned32 constructor overload, including
// INT_MAX inline. The source uint64 setter uses a distinct strict boundary.
func (s *RationalService) SetUnsigned(v *OwnedRational, n uint32) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.check(v); e != nil {
		return e
	}
	return s.Manager.SetUnsigned(&v.Raw, n)
}
func (s *RationalService) SetUint64(v *OwnedRational, n uint64) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e := s.check(v); e != nil {
		return e
	}
	return s.Manager.SetUint64(&v.Raw, n)
}
func (s *RationalService) SetFraction(v *OwnedRational, n int64, d uint64) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.check(v); e != nil {
		return e
	}
	return s.Manager.SetFraction(&v.Raw, n, d)
}
func (s *RationalService) SetFractionSigned(v *OwnedRational, n, d int32) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.check(v); e != nil {
		return e
	}
	return s.Manager.SetFractionSigned(&v.Raw, n, d)
}
func (s *RationalService) SetString(v *OwnedRational, text string) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.check(v); e != nil {
		return e
	}
	return s.Manager.SetString(&v.Raw, text)
}
func (s *RationalService) String(v *OwnedRational) (string, error) {
	if e := s.lock(); e != nil {
		return "", e
	}
	defer s.mu.Unlock()
	if e := s.check(v); e != nil {
		return "", e
	}
	return s.Manager.String(&v.Raw)
}
func (s *RationalService) Equal(a, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}
	return s.Manager.Equal(&a.Raw, &b.Raw)
}
func (s *RationalService) copyOutput(raw *Rational) (_lexOut OwnedRational, _lexErr error) {
	return s.copyOutputWithInvocation(raw, nil)
}
func (s *RationalService) copyOutputWithInvocation(raw *Rational, c *RationalServiceInvocation) (_lexOut OwnedRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	v := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	if e := s.Manager.copyWithAllocationContext(&v.Raw, raw, c.allocationContext()); e != nil {
		s.abortLexicalRational(&v.Raw)
		return OwnedRational{}, e
	}
	return v, nil
}
func (s *RationalService) Zero() (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	return s.copyOutput(&s.ZeroField)
}
func (s *RationalService) One() (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	return s.copyOutput(&s.OneField)
}
func (s *RationalService) MinusOne() (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	return s.copyOutput(&s.MinusOneField)
}
func (s *RationalService) pushCache(v *Rational) (_lexErr error) {
	return s.pushCacheWithInvocation(v, nil)
}
func (s *RationalService) pushCacheWithInvocation(v *Rational, c *RationalServiceInvocation) (_lexErr error) {
	if e := s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&_lexErr)
	item := NewRational()
	if e := s.Manager.copyWithAllocationContext(&item, v, c.allocationContext()); e != nil {
		s.abortLexicalRational(&item)
		return e
	}
	// Complete the owned numeric copy before publishing it in Go-managed storage.
	s.Cache = append(s.Cache, item)
	return nil
}
func (s *RationalService) fillPower(n uint32) (_lexErr error) {
	return s.fillPowerWithInvocation(n, nil)
}
func (s *RationalService) fillPowerWithInvocation(n uint32, c *RationalServiceInvocation) (_lexErr error) {
	if e := s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&_lexErr)
	if len(s.Cache) == 0 {
		if e := s.pushCacheWithInvocation(&s.OneField, c); e != nil {
			return e
		}
	}
	size := uint32(len(s.Cache))
	curr, two := NewRational(), NewRational()
	currComplete := false
	defer s.endLexicalLocal(&curr, &currComplete)
	twoComplete := false
	defer s.endLexicalLocal(&two, &twoComplete)
	if e := s.Manager.copyWithAllocationContext(&curr, &s.Cache[size-1], c.allocationContext()); e != nil {
		return e
	}
	currComplete = true
	if e := s.Manager.SetInt64(&two, 2); e != nil {
		return e
	}
	twoComplete = true
	for i := size; i <= n; i++ {
		if e := s.Manager.mulWithAllocationContext(&curr, &two, &curr, c.allocationContext()); e != nil {
			return e
		}
		if e := s.pushCacheWithInvocation(&curr, c); e != nil {
			return e
		}
	}
	return nil
}
func (s *RationalService) PowerOfTwo(k uint32) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer func() { atomic.StoreUint32(&s.sourceRuntimeExecuting, 0) }()
	return s.powerOfTwoLocked(k)
}
func (s *RationalService) powerOfTwoLocked(k uint32) (OwnedRational, error) {
	return s.powerOfTwoLockedWithInvocation(k, nil)
}
func (s *RationalService) powerOfTwoLockedWithInvocation(k uint32, c *RationalServiceInvocation) (_lexOut OwnedRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	// Source completes this default result before the cache fill/assignment.
	result := NewRational()
	defer s.destroyLexicalRational(&result)
	// Checked adapter for source k+1/loop and k+2 length overflow. Host backing
	// cardinality is checked before any numerical action or budget reservation.
	if k > math.MaxUint32-2 || uint64(k+2)*uint64(reflect.TypeOf(Rational{}).Size()) > uint64(^uint(0)>>1) {
		return OwnedRational{}, arena.ErrBounds
	}
	if uint64(k) >= uint64(len(s.Cache)) {
		if e := s.fillPowerWithInvocation(k+1, c); e != nil {
			return OwnedRational{}, e
		}
	}
	if e := s.Manager.copyWithAllocationContext(&result, &s.Cache[k], c.allocationContext()); e != nil {
		return OwnedRational{}, e
	}
	return s.facadeTakeLocked(&result)
}
func (s *RationalService) Stats() (RationalServiceStats, error) {
	if e := s.lock(); e != nil {
		return RationalServiceStats{}, e
	}
	defer s.mu.Unlock()
	z := s.Manager.Z
	stats := RationalServiceStats{CacheLength: uint32(len(s.Cache)), CacheCapacity: uint32(cap(s.Cache)), HeaderBytes: s.HeaderBytes, LiveCells: z.Cells.Stats().Live}
	for n := uint64(1); n <= uint64(z.Cells.Stats().Slots); n++ {
		c, e := z.Cells.Read(arena.Slot(n))
		if e != nil {
			continue
		}
		if !c.Released {
			stats.LiveNumericObjects++
		}
		if c.Unpublished || c.CleanupPending {
			stats.RetainedCellDescriptors++
		}
	}
	if z.PendingCell.Object != (umemory.ObjectRef{}) {
		stats.PendingCellObjects = 1
	}
	stats.LivePayloads = stats.LiveNumericObjects + stats.PendingCellObjects + stats.NaturalPayloads
	return stats, nil
}

// destroyCache is the source vector::finalize traversal. Caller holds the owner
// lock; its separate cold helper permits observing release order before the
// manager itself is destroyed, without stored callbacks or per-record boxing.
func (s *RationalService) destroyCache() {
	for i := range s.Cache {
		s.Manager.Del(&s.Cache[i])
	}
	s.Cache = nil
}

// The two source finalizer hooks run before the powers vector is destroyed.
// Caller holds the service lock (or the constructor has not published it).
func (s *RationalService) destroyInfinitesimalConstants() {
	s.destroyInfRationalConstants()
	s.destroyInfIntRationalConstants()
}
func (s *RationalService) Close() error { return s.CloseChecked() }

// Allocator failure transport and trace output are caller-confined Go callback
// domains. Ordinary concurrent service calls continue to serialize on mu.
func (s *RationalService) runtimeTransportExecuting() bool {
	// A locked numeric borrow can invoke a different owner's allocator. Its
	// actual host callbacks must not reenter this service's held mutex.
	if v := s.numericReceiver.Load(); v != nil {
		if z := v.(rationalNumericReceiver).integer; z != nil {
			if z.BorrowTraceTags().OutputCallbackActive() || z.Memory.FailureTransportActive() {
				return true
			}
		}
	}
	if atomic.LoadUint32(&s.sourceRuntimeExecuting) == 0 {
		return false
	}
	return s.Manager != nil && s.Manager.Z != nil && s.Manager.Z.Memory.FailureTransportActive()
}
