package numeral

// Source: src/util/mpbq.h/.cpp.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// BinaryRational owns its numerator in its implicit IntegerManager domain.
// A zero value is valid. Never shallow-copy owning big values. Constructors
// preserve source raw shape; denominator-power setters normalize.
type BinaryRational struct {
	Num    Int
	KField uint32
}

func NewBinaryRational() BinaryRational   { return BinaryRational{} }
func NewBinaryInt(n int32) BinaryRational { return BinaryRational{Num: NewInt(n)} }
func NewBinaryIntPower(n int32, k uint32) BinaryRational {
	return BinaryRational{Num: NewInt(n), KField: k}
}

type BinaryRationalConfig struct{ Diagnostics *RationalService }

// K is the source scalar accessor. Numerator extraction uses an explicit
// owning manager copy rather than exporting an owning raw-handle copy.
func (a *BinaryRational) K() uint32 { return a.KField }

var ErrBinaryOwner = errors.New("mpnum: invalid binary-rational owner")
var ErrBinaryInvariant = errors.New("mpnum: binary-rational source invariant")
var ErrBinaryInterval = errors.New("mpnum: binary-rational source interval precondition")
var ErrBinaryRootDegreeZero = errors.New("mpnum: binary-rational root degree zero")
var ErrBinaryComparePending = errors.New("mpnum: binary-rational comparison cleanup pending")
var ErrBinaryConvertPending = errors.New("mpnum: binary-rational conversion cleanup pending")
var ErrBinaryRoundPending = errors.New("mpnum: binary-rational rounding cleanup pending")
var ErrBinaryToRationalPending = errors.New("mpnum: binary-rational to-rational cleanup pending")
var ErrBinaryOwnedRationalPending = errors.New("mpnum: binary-rational owned conversion cleanup pending")
var ErrBinaryApproxPending = errors.New("mpnum: binary-rational approximation cleanup pending")
var ErrBinaryDebugIntegerPending = errors.New("mpnum: binary-rational debug integer cleanup pending")
var ErrBinaryDebugSnapshotPending = errors.New("mpnum: binary-rational debug snapshot cleanup pending")

type binaryOwnedRationalScope struct {
	calls                    [3]RationalServiceInvocation
	callsPending             [3]bool
	sourceCause              error
	owning                   bool
	service                  *RationalService
	r, twok, two, pw, result OwnedRational
	cursor                   uint8
	pending, executing       bool
}

// This cold operation owner borrows z and diagnostics. Both must outlive it.
// No operations or shared Budget accesses are concurrently safe.
type BinaryRationalManager struct {
	integerDebugInvocation                         uint64
	integerDebugExecuting, integerDebugPending     bool
	integerDebugSourceCause                        error
	integerDebugChildren                           binaryDebugChildren
	convertDebugChildren                           binaryDebugChildren
	refineDebugChildren                            binaryDebugChildren
	toOwnedInvocation                              uint64
	debugIntegerInvocation                         uint64
	debugArithmeticInvocation                      uint64
	debugArithmeticCall                            RationalServiceInvocation
	debugArithmeticPending                         bool
	debugArithmeticExecuting                       bool
	convertInvocation                              uint64
	convertSourceCause                             error
	convertDebugPending                            bool
	refineInvocation                               uint64
	refineSourceCause                              error
	refineDebugPending                             bool
	Z                                              *IntegerManager
	Diagnostics                                    *RationalService
	self                                           *BinaryRationalManager
	Tmp, Tmp2                                      Int
	AddMulTmp                                      BinaryRational
	SelectIntTmp1, SelectIntTmp2, SelectSmallTmp   Int
	SelectSmallTmp1, SelectSmallTmp2               BinaryRational
	DivTmp1, DivTmp2, DivTmp3                      Int
	HeaderBytes                                    uint64
	closed                                         bool
	closing                                        bool
	cleanupStage                                   uint8
	sourceStarted, sourceFinalized, sourceRunning  bool
	sourceStage                                    uint8
	sourceCause                                    error
	Display                                        binaryDisplayScope
	selectIntTemporary                             Int
	selectIntOwner                                 *RationalManager
	selectIntPending                               bool
	selectIntExecuting                             bool
	selectRationalLower, selectRationalUpper       Rational
	selectRationalOwner                            *RationalManager
	selectRationalShape, selectRationalCursor      uint8
	selectRationalPending, selectRationalExecuting bool
	refineMid                                      BinaryRational
	refinePending, refineExecuting                 bool
	refineImported                                 Rational
	refineImportedPending                          bool
	refineImportedCursor                           uint8
	compareImported                                Rational
	compareImportedPending, compareExecuting       bool
	compareImportedCursor                          uint8
	convertImported                                Rational
	convertImportedPending, convertExecuting       bool
	convertImportedCursor                          uint8
	roundImported                                  Int
	roundImportedOwner                             *IntegerManager
	roundImportedPending, roundExecuting           bool
	toRationalImported                             Int
	toRationalImportedOwner                        *IntegerManager
	toRationalImportedPending, toRationalExecuting bool
	toOwnedRationalScope                           binaryOwnedRationalScope
	approxOld                                      BinaryRational
	approxPending                                  bool
	debugIntegerValue                              OwnedRational
	debugIntegerPending, debugIntegerExecuting     bool
	debugSnapshotQueue                             []OwnedRational
}
type BinaryRationalStats struct {
	HeaderBytes           uint64
	LiveScratchNumerators uint32
}

func NewBinaryRationalManager(z *IntegerManager, c BinaryRationalConfig) (*BinaryRationalManager, error) {
	if z == nil || z.closed || z.Cells == nil {
		return nil, ErrBinaryOwner
	}
	if e := binaryCheckDiagnostics(z, c.Diagnostics); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(BinaryRationalManager{}).Size())
	if e := z.BudgetField.Reserve(h); e != nil {
		return nil, e
	}
	m := &BinaryRationalManager{Z: z, Diagnostics: c.Diagnostics, HeaderBytes: h}
	m.self = m
	return m, nil
}
func (m *BinaryRationalManager) open() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.closed || m.closing || m.Z.closed {
		return arena.ErrClosed
	}
	if m.Display.running {
		return ErrBinaryDisplayPending
	}
	if m.selectIntPending {
		return ErrBinarySelectPending
	}
	if m.selectRationalPending && !m.selectRationalExecuting {
		return ErrBinarySelectPending
	}
	if (m.refinePending || m.refineImportedPending) && !m.refineExecuting {
		return ErrBinaryRefinePending
	}
	if m.compareImportedPending && !m.compareExecuting {
		return ErrBinaryComparePending
	}
	if m.convertImportedPending && !m.convertExecuting {
		return ErrBinaryConvertPending
	}
	if m.roundImportedPending && !m.roundExecuting {
		return ErrBinaryRoundPending
	}
	if m.toRationalImportedPending && !m.toRationalExecuting {
		return ErrBinaryToRationalPending
	}
	if m.toOwnedRationalScope.pending && !m.toOwnedRationalScope.executing {
		return ErrBinaryOwnedRationalPending
	}
	if m.approxPending {
		return ErrBinaryApproxPending
	}
	if m.debugIntegerPending && !m.debugIntegerExecuting {
		return ErrBinaryDebugIntegerPending
	}
	if len(m.debugSnapshotQueue) != 0 {
		return ErrBinaryDebugSnapshotPending
	}
	if m.debugArithmeticPending && !m.debugArithmeticExecuting {
		return ErrBinaryDebugSnapshotPending
	}
	return nil
}
func (m *BinaryRationalManager) check(a *BinaryRational) error {
	if e := m.open(); e != nil {
		return e
	}
	if a == nil {
		return arena.ErrInvalidSlot
	}
	return m.Z.check(&a.Num)
}
func (m *BinaryRationalManager) checkCleanup(a *BinaryRational) error {
	if e := m.open(); e != nil {
		// Completed source destructors leave only unjoined RAW children.
		// Older deletions can proceed while new arithmetic remains guarded.
		completed := e == ErrBinaryOwnedRationalPending && m.toOwnedRationalScope.cursor == 5 && m.toOwnedRationalScope.sourceCause == nil
		completed = completed || (e == ErrBinaryDebugSnapshotPending && len(m.debugSnapshotQueue) == 0 && m.debugArithmeticPending && m.debugArithmeticCall.SourceReady() == nil)
		if !completed {
			return e
		}
	}
	if a == nil {
		return arena.ErrInvalidSlot
	}
	return m.Z.checkCleanup(&a.Num)
}
func (m *BinaryRationalManager) pair(a, b *BinaryRational) error {
	if e := m.check(a); e != nil {
		return e
	}
	return m.check(b)
}
func (m *BinaryRationalManager) triple(a, b, r *BinaryRational) error {
	if e := m.pair(a, b); e != nil {
		return e
	}
	return m.check(r)
}
func (m *BinaryRationalManager) Precise() bool { return true }
func (m *BinaryRationalManager) Field() bool   { return false }
func (m *BinaryRationalManager) checkDestructorExecution() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.Display.running {
		return ErrBinaryDisplayPending
	}
	for binding := m.Z.CallerBindings(); binding != nil; binding = binding.Previous() {
		if guard, ok := binding.Guard().(interface {
			NumericBinaryExecuting(*BinaryRationalManager) bool
		}); ok && guard.NumericBinaryExecuting(m) {
			return ErrNumericPending
		}
	}
	if m.selectIntExecuting || m.selectRationalExecuting {
		return ErrBinarySelectPending
	}
	if m.refineExecuting {
		return ErrBinaryRefinePending
	}
	if m.compareExecuting {
		return ErrBinaryComparePending
	}
	if m.convertExecuting {
		return ErrBinaryConvertPending
	}
	if m.roundExecuting {
		return ErrBinaryRoundPending
	}
	if m.toRationalExecuting {
		return ErrBinaryToRationalPending
	}
	if m.toOwnedRationalScope.executing {
		return ErrBinaryOwnedRationalPending
	}
	if m.debugIntegerExecuting {
		return ErrBinaryDebugIntegerPending
	}
	if m.sourceRunning {
		return ErrNumericPending
	}
	return nil
}
func (m *BinaryRationalManager) CloseChecked() error {
	if e := m.checkDestructorExecution(); e != nil {
		return e
	}
	if m.closed {
		return nil
	}
	m.closing = true
	if e := m.CleanupSelectIntegerTemporary(); e != nil {
		return e
	}
	if e := m.CleanupSelectRationalTemporaries(); e != nil {
		return e
	}
	if e := m.CleanupRefineImported(); e != nil {
		return e
	}
	if e := m.CleanupCompareImported(); e != nil {
		return e
	}
	if e := m.CleanupConvertImported(); e != nil {
		return e
	}
	if e := m.CleanupRoundImported(); e != nil {
		return e
	}
	if e := m.CleanupToRationalImported(); e != nil {
		return e
	}
	if e := m.CleanupToOwnedRational(); e != nil {
		return e
	}
	if e := m.CleanupApproxDebug(); e != nil {
		return e
	}
	if e := m.CleanupDebugInteger(); e != nil {
		return e
	}
	if e := m.CleanupDebugSnapshots(); e != nil {
		return e
	}
	if e := m.CleanupRefineMid(); e != nil {
		return e
	}
	if e := m.CleanupDisplay(); e != nil {
		return e
	}
	roots := [11]*Int{&m.AddMulTmp.Num, &m.Tmp, &m.Tmp2, &m.SelectIntTmp1, &m.SelectIntTmp2, &m.SelectSmallTmp, &m.SelectSmallTmp1.Num, &m.SelectSmallTmp2.Num, &m.DivTmp1, &m.DivTmp2, &m.DivTmp3}
	for m.cleanupStage < 11 {
		if e := m.Z.Del(roots[m.cleanupStage]); e != nil {
			return e
		}
		m.cleanupStage++
	}
	m.Z.BudgetField.Release(m.HeaderBytes)
	m.HeaderBytes = 0
	m.closed = true
	return nil
}
func (m *BinaryRationalManager) Close() {
	if m == nil || m.self != m || m.closed {
		return
	}
	if e := m.CloseChecked(); e != nil {
		panic(e)
	}
}

func (m *BinaryRationalManager) cleanupDebugInteger() error {
	if !m.debugIntegerPending {
		return nil
	}
	if m.debugIntegerValue.ownerID != 0 {
		if e := m.Diagnostics.Del(&m.debugIntegerValue); e != nil {
			return e
		}
	}
	m.debugIntegerValue = OwnedRational{}
	m.debugIntegerPending = false
	return nil
}

// CleanupDebugInteger releases a stopped mixed-arithmetic diagnostic copy.
func (m *BinaryRationalManager) CleanupDebugInteger() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.debugIntegerExecuting {
		return ErrBinaryDebugIntegerPending
	}
	return m.cleanupDebugInteger()
}

// CleanupDebugSnapshots resumes the returned rational snapshots in the order
// their source destructors were reached.
func (m *BinaryRationalManager) CleanupDebugSnapshots() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.debugArithmeticExecuting {
		return ErrBinaryDebugSnapshotPending
	}
	if m.debugArithmeticPending {
		if e := m.debugArithmeticCall.Cleanup(false); e != nil {
			return e
		}
	}
	for len(m.debugSnapshotQueue) != 0 {
		if e := m.Diagnostics.Del(&m.debugSnapshotQueue[0]); e != nil {
			return e
		}
		m.debugSnapshotQueue[0] = OwnedRational{}
		m.debugSnapshotQueue = m.debugSnapshotQueue[1:]
	}
	m.debugSnapshotQueue = nil
	if m.debugArithmeticPending {
		if e := m.debugArithmeticCall.Cleanup(true); e != nil {
			return e
		}
		if e := m.debugArithmeticCall.EndContext(); e != nil {
			return e
		}
		m.debugArithmeticPending = false
	}
	return nil
}

func (m *BinaryRationalManager) Stats() (BinaryRationalStats, error) {
	if e := m.open(); e != nil {
		return BinaryRationalStats{}, e
	}
	n := uint32(0)
	for _, v := range [...]Int{m.Tmp, m.Tmp2, m.AddMulTmp.Num, m.SelectIntTmp1, m.SelectIntTmp2, m.SelectSmallTmp, m.SelectSmallTmp1.Num, m.SelectSmallTmp2.Num, m.DivTmp1, m.DivTmp2, m.DivTmp3} {
		if ValidNumericRef(m.Z.Cells, v.Cell) {
			n++
		}
	}
	return BinaryRationalStats{HeaderBytes: m.HeaderBytes, LiveScratchNumerators: n}, nil
}
func (m *BinaryRationalManager) Del(a *BinaryRational) error {
	if e := m.checkCleanup(a); e != nil {
		return e
	}
	return m.Z.Del(&a.Num)
}
func (m *BinaryRationalManager) Reset(a *BinaryRational) error {
	if e := m.checkCleanup(a); e != nil {
		return e
	}
	if e := m.Z.Reset(&a.Num); e != nil {
		return e
	}
	a.KField = 0
	return nil
}
func (m *BinaryRationalManager) Swap(a, b *BinaryRational) error {
	if e := m.pair(a, b); e != nil {
		return e
	}
	*a, *b = *b, *a
	return nil
}
func (m *BinaryRationalManager) Copy(dst, src *BinaryRational) error {
	return m.copyWithAllocationReceipt(dst, src, nil)
}

// CopyWithAllocationReceipt records only the actually reached integer copy.
// The denominator exponent is published after that copy succeeds.
func (m *BinaryRationalManager) CopyWithAllocationReceipt(dst, src *BinaryRational, receipt *IntegerCopyAllocationReceipt) error {
	return m.copyWithAllocationReceipt(dst, src, receipt)
}
func (m *BinaryRationalManager) copyWithAllocationReceipt(dst, src *BinaryRational, receipt *integerCopyAllocationReceipt) error {
	if e := m.pair(dst, src); e != nil {
		return e
	}
	if e := m.Z.CopyFromOwnerWithReceipt(&dst.Num, m.Z, &src.Num, receipt); e != nil {
		return e
	}
	dst.KField = src.KField
	return nil
}

// CopyFromOwnerWithAllocationReceipt is the concrete cross-manager mpbq SET
// used when algebraic cells move between independent arithmetic plugins.
// Preserve numerator SET before exponent publication, including its exact
// accepted destructive allocation receipt. The caller owns enclosing lifetimes.
func (m *BinaryRationalManager) CopyFromOwnerWithAllocationReceipt(dst *BinaryRational, source *BinaryRationalManager, src *BinaryRational, receipt *IntegerCopyAllocationReceipt) error {
	if source == m {
		return m.CopyWithAllocationReceipt(dst, src, receipt)
	}
	if e := m.check(dst); e != nil {
		return e
	}
	if source == nil || dst == src {
		return arena.ErrConfig
	}
	if e := source.check(src); e != nil {
		return e
	}
	if e := m.Z.CopyFromOwnerWithReceipt(&dst.Num, source.Z, &src.Num, receipt); e != nil {
		return e
	}
	dst.KField = src.KField
	return nil
}

func (m *BinaryRationalManager) SetInt32(a *BinaryRational, n int32) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.Z.SetInt32(&a.Num, n); e != nil {
		return e
	}
	a.KField = 0
	return nil
}
func (m *BinaryRationalManager) SetUint32(a *BinaryRational, n uint32) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.Z.SetUint32(&a.Num, n); e != nil {
		return e
	}
	a.KField = 0
	return nil
}
func (m *BinaryRationalManager) SetInt32Power(a *BinaryRational, n int32, k uint32) error {
	if e := m.SetInt32(a, n); e != nil {
		return e
	}
	a.KField = k
	return m.normalize(a)
}
func (m *BinaryRationalManager) SetInt64Power(a *BinaryRational, n int64, k uint32) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.Z.SetInt64(&a.Num, n); e != nil {
		return e
	}
	a.KField = k
	return m.normalize(a)
}
func (m *BinaryRationalManager) SetInteger(a *BinaryRational, n *Int) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.Z.Copy(&a.Num, n); e != nil {
		return e
	}
	a.KField = 0
	return nil
}
func (m *BinaryRationalManager) SetIntegerPower(a *BinaryRational, n *Int, k uint32) error {
	if e := m.SetInteger(a, n); e != nil {
		return e
	}
	a.KField = k
	return m.normalize(a)
}
func (m *BinaryRationalManager) Numerator(a *BinaryRational, n *Int) error {
	if e := m.check(a); e != nil {
		return e
	}
	return m.Z.Copy(n, &a.Num)
}
func (m *BinaryRationalManager) DenominatorPower(a *BinaryRational) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	return a.KField, nil
}
func (m *BinaryRationalManager) IsInteger(a *BinaryRational) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return a.KField == 0, nil
}
func (m *BinaryRationalManager) IsZero(a *BinaryRational) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return m.Z.IsZero(&a.Num)
}
func (m *BinaryRationalManager) IsNonzero(a *BinaryRational) (bool, error) {
	v, e := m.IsZero(a)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (m *BinaryRationalManager) IsOne(a *BinaryRational) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if a.KField != 0 {
		return false, nil
	}
	return m.Z.IsOne(&a.Num)
}
func (m *BinaryRationalManager) IsPos(a *BinaryRational) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return m.Z.IsPos(&a.Num)
}
func (m *BinaryRationalManager) IsNeg(a *BinaryRational) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return m.Z.IsNeg(&a.Num)
}
func (m *BinaryRationalManager) IsNonPos(a *BinaryRational) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return m.Z.IsNonPos(&a.Num)
}
func (m *BinaryRationalManager) IsNonNeg(a *BinaryRational) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return m.Z.IsNonNeg(&a.Num)
}
func (m *BinaryRationalManager) normalize(a *BinaryRational) (err error) {
	return m.normalizeWithAllocationContext(a, integerAllocationInvocationContext{})
}
func (m *BinaryRationalManager) normalizeWithAllocationContext(a *BinaryRational, context integerAllocationInvocationContext) (err error) {
	if a.KField == 0 {
		return nil
	}
	if a.Num.Val == 0 {
		a.KField = 0
		return nil
	}
	old, e := m.debugValue(a)
	if e != nil {
		return e
	}
	defer func() {
		if cleanup := m.debugDel(&old); err == nil {
			err = cleanup
		}
	}()
	k, e := m.Z.PowerOfTwoMultiple(&a.Num)
	if e != nil {
		return e
	}
	if k > a.KField {
		k = a.KField
	}
	if e = m.Z.shiftWithAllocationContext(&a.Num, k, &a.Num, false, context); e != nil {
		return e
	}
	a.KField -= k
	return m.debugEqualValue(&old, a)
}

// Copy bridges retain source raw numerator/denominator shape and never read
// a foreign numeric handle through this owner. The caller supplies provenance.
func (m *BinaryRationalManager) importRational(qm *RationalManager, q *Rational, local *Rational) (*Rational, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	if qm == nil || qm.Z == nil {
		return nil, ErrBinaryOwner
	}
	if e := qm.check(q); e != nil {
		return nil, e
	}
	if qm.Z == m.Z {
		return q, nil
	}
	if e := m.Z.CopyFrom(&local.Num, qm.Z, &q.Num); e != nil {
		return nil, e
	}
	if e := m.Z.CopyFrom(&local.Den, qm.Z, &q.Den); e != nil {
		return nil, e
	}
	return local, nil
}
