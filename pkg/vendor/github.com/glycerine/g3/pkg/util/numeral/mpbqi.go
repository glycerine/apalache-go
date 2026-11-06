package numeral

// Source: src/util/basic_interval.h:159-163; src/util/mpbqi.h.
// Concrete executable basic_interval<mpbq,false> and mpbqi.h specialization.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// BinaryInterval owns two bounds in its implicit BinaryRationalManager domain.
// Never shallow-copy owning intervals. Raw zero endpoints are valid.
type BinaryInterval struct{ lower, upper BinaryRational }

func NewBinaryInterval() BinaryInterval { return BinaryInterval{} }

// BinaryIntervalManager borrows bounds; caller values/scopes die before source
// destruction. Owning cleanup may follow bounds source destruction; the integer
// owner outlives both. Owner-confined, not concurrent.
type BinaryIntervalManager struct {
	debugInvocation                               uint64
	debugExecuting                                bool
	debugPending                                  bool
	debugSourceCause                              error
	debugChildren                                 binaryDebugChildren
	bounds                                        *BinaryRationalManager
	self                                          *BinaryIntervalManager
	MulCurr, MulMax, MulMin                       BinaryRational
	HeaderBytes                                   uint64
	closed                                        bool
	closing                                       bool
	cleanupStage                                  uint8
	sourceStarted, sourceFinalized, sourceRunning bool
	sourceStage                                   uint8
	sourceCause                                   error
}
type BinaryIntervalStats struct {
	HeaderBytes           uint64
	LiveScratchNumerators uint32
}

func NewBinaryIntervalManager(b *BinaryRationalManager) (*BinaryIntervalManager, error) {
	if e := b.open(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(BinaryIntervalManager{}).Size())
	if e := b.Z.BudgetField.Reserve(h); e != nil {
		return nil, e
	}
	m := &BinaryIntervalManager{bounds: b, HeaderBytes: h}
	m.self = m
	return m, nil
}
func (m *BinaryIntervalManager) open() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.closed || m.closing {
		return arena.ErrClosed
	}
	return m.bounds.open()
}
func (m *BinaryIntervalManager) check(a *BinaryInterval) error {
	if e := m.open(); e != nil {
		return e
	}
	if a == nil {
		return arena.ErrInvalidSlot
	}
	return m.bounds.pair(&a.lower, &a.upper)
}
func (m *BinaryIntervalManager) checkCleanup(a *BinaryInterval) error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.closed || m.closing {
		return arena.ErrClosed
	}
	if a == nil {
		return arena.ErrInvalidSlot
	}
	if e := m.bounds.checkCleanup(&a.lower); e != nil {
		return e
	}
	return m.bounds.checkCleanup(&a.upper)
}
func (m *BinaryIntervalManager) pair(a, b *BinaryInterval) error {
	if e := m.check(a); e != nil {
		return e
	}
	return m.check(b)
}
func (m *BinaryIntervalManager) triple(a, b, c *BinaryInterval) error {
	if e := m.pair(a, b); e != nil {
		return e
	}
	return m.check(c)
}
func (m *BinaryIntervalManager) Bounds() (*BinaryRationalManager, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	return m.bounds, nil
}
func (m *BinaryIntervalManager) Stats() (BinaryIntervalStats, error) {
	if e := m.open(); e != nil {
		return BinaryIntervalStats{}, e
	}
	n := uint32(0)
	for _, v := range [...]BinaryRational{m.MulCurr, m.MulMax, m.MulMin} {
		if v.Num.Cell != 0 {
			n++
		}
	}
	return BinaryIntervalStats{m.HeaderBytes, n}, nil
}
func (m *BinaryIntervalManager) Close() {
	if m == nil || m.self != m || m.closed {
		return
	}
	if e := m.CloseChecked(); e != nil {
		panic(e)
	}
}

// CloseChecked owns interrupted scratch releases and the Go header.
// SourceFinalize preserves the separate literal source destructor history.
func (m *BinaryIntervalManager) CloseChecked() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.sourceRunning {
		return ErrNumericPending
	}
	if m.closed {
		return nil
	}
	m.closing = true
	roots := [3]*BinaryRational{&m.MulCurr, &m.MulMax, &m.MulMin}
	for m.cleanupStage < 3 {
		var e error
		if m.sourceStarted {
			// The borrowed binary manager may already have completed its
			// later source destructor. Own only these actual NUM fields.
			e = m.bounds.Z.Del(&roots[m.cleanupStage].Num)
		} else {
			e = m.bounds.Del(roots[m.cleanupStage])
		}
		if e != nil {
			return e
		}
		m.cleanupStage++
	}
	m.bounds.Z.BudgetField.Release(m.HeaderBytes)
	m.HeaderBytes = 0
	m.closed = true
	return nil
}
func (m *BinaryIntervalManager) Del(a *BinaryInterval) error {
	if e := m.checkCleanup(a); e != nil {
		return e
	}
	if e := m.bounds.Del(&a.lower); e != nil {
		return e
	}
	return m.bounds.Del(&a.upper)
}
func (m *BinaryIntervalManager) Reset(a *BinaryInterval) error {
	if e := m.checkCleanup(a); e != nil {
		return e
	}
	if e := m.bounds.Reset(&a.lower); e != nil {
		return e
	}
	return m.bounds.Reset(&a.upper)
}
func (m *BinaryIntervalManager) Lower(a *BinaryInterval, out *BinaryRational) error {
	if e := m.check(a); e != nil {
		return e
	}
	return m.bounds.Copy(out, &a.lower)
}
func (m *BinaryIntervalManager) Upper(a *BinaryInterval, out *BinaryRational) error {
	if e := m.check(a); e != nil {
		return e
	}
	return m.bounds.Copy(out, &a.upper)
}

// Source SASSERT expressions do not execute in release builds.
func (m *BinaryIntervalManager) ordered(l, u *BinaryRational) error {
	if !SourceExtendedGCDDebug {
		return nil
	}
	ok, e := m.bounds.LessEqual(l, u)
	if e != nil {
		return e
	}
	if !ok {
		return ErrBinaryInvariant
	}
	return nil
}
func (m *BinaryIntervalManager) SetBounds(a *BinaryInterval, l, u *BinaryRational) error {
	return m.setBoundsWithAllocationReceipt(a, l, u, nil)
}
func (m *BinaryIntervalManager) setBoundsWithAllocationReceipt(a *BinaryInterval, l, u *BinaryRational, receipt *IntegerCopyAllocationReceipt) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.bounds.pair(l, u); e != nil {
		return e
	}
	if e := m.ordered(l, u); e != nil {
		return e
	}
	if e := m.bounds.CopyWithAllocationReceipt(&a.lower, l, receipt); e != nil {
		return e
	}
	return m.bounds.CopyWithAllocationReceipt(&a.upper, u, receipt)
}
func (m *BinaryIntervalManager) Copy(dst, src *BinaryInterval) error {
	return m.CopyWithAllocationReceipt(dst, src, nil)
}
func (m *BinaryIntervalManager) CopyWithAllocationReceipt(dst, src *BinaryInterval, receipt *IntegerCopyAllocationReceipt) error {
	if e := m.pair(dst, src); e != nil {
		return e
	}
	return m.setBoundsWithAllocationReceipt(dst, &src.lower, &src.upper, receipt)
}
func (m *BinaryIntervalManager) SetPoint(a *BinaryInterval, n *BinaryRational) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.bounds.check(n); e != nil {
		return e
	}
	if e := m.bounds.Copy(&a.lower, n); e != nil {
		return e
	}
	return m.bounds.Copy(&a.upper, n)
}
func (m *BinaryIntervalManager) SetInteger(a *BinaryInterval, n *Int) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.bounds.Z.check(n); e != nil {
		return e
	}
	if e := m.bounds.SetInteger(&a.lower, n); e != nil {
		return e
	}
	return m.bounds.SetInteger(&a.upper, n)
}
func (m *BinaryIntervalManager) SetLower(a *BinaryInterval, n *BinaryRational) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.bounds.check(n); e != nil {
		return e
	}
	if e := m.ordered(n, &a.upper); e != nil {
		return e
	}
	return m.bounds.Copy(&a.lower, n)
}
func (m *BinaryIntervalManager) SetUpper(a *BinaryInterval, n *BinaryRational) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.bounds.check(n); e != nil {
		return e
	}
	if e := m.ordered(&a.lower, n); e != nil {
		return e
	}
	return m.bounds.Copy(&a.upper, n)
}
func (m *BinaryIntervalManager) Swap(a, b *BinaryInterval) error {
	if e := m.pair(a, b); e != nil {
		return e
	}
	if e := m.bounds.Swap(&a.lower, &b.lower); e != nil {
		return e
	}
	return m.bounds.Swap(&a.upper, &b.upper)
}
func (m *BinaryIntervalManager) Neg(a *BinaryInterval) error {
	return m.negWithSourceContext(a, integerAllocationInvocationContext{})
}
func (m *BinaryIntervalManager) negWithSourceContext(a *BinaryInterval, context integerAllocationInvocationContext) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.bounds.negWithSourceContext(&a.lower, context); e != nil {
		return e
	}
	if e := m.bounds.negWithSourceContext(&a.upper, context); e != nil {
		return e
	}
	return m.bounds.Swap(&a.lower, &a.upper)
}
func (m *BinaryIntervalManager) IsEmpty(a *BinaryInterval) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return m.bounds.Equal(&a.lower, &a.upper)
}
func (m *BinaryIntervalManager) IsPos(a *BinaryInterval) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return m.bounds.IsNonNeg(&a.lower)
}
func (m *BinaryIntervalManager) IsNeg(a *BinaryInterval) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return m.bounds.IsNonPos(&a.upper)
}
func (m *BinaryIntervalManager) ContainsZero(a *BinaryInterval) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	ok, e := m.bounds.IsNeg(&a.lower)
	if e != nil || !ok {
		return false, e
	}
	return m.bounds.IsPos(&a.upper)
}
func (m *BinaryIntervalManager) IsSubset(a, b *BinaryInterval) (bool, error) {
	if e := m.pair(a, b); e != nil {
		return false, e
	}
	ok, e := m.bounds.LessEqual(&b.lower, &a.lower)
	if e != nil || !ok {
		return false, e
	}
	return m.bounds.LessEqual(&a.upper, &b.upper)
}

// Deliberately preserves source upper/upper typo for the open specialization.
func (m *BinaryIntervalManager) Disjoint(a, b *BinaryInterval) (bool, error) {
	if e := m.pair(a, b); e != nil {
		return false, e
	}
	ok, e := m.bounds.LessEqual(&a.upper, &b.upper)
	if e != nil || ok {
		return ok, e
	}
	return m.bounds.LessEqual(&b.upper, &a.lower)
}
func (m *BinaryIntervalManager) Precedes(a, b *BinaryInterval) (bool, error) {
	if e := m.pair(a, b); e != nil {
		return false, e
	}
	return m.bounds.LessEqual(&a.upper, &b.lower)
}
func (m *BinaryIntervalManager) PrecedesBound(a *BinaryInterval, b *BinaryRational) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return m.bounds.LessEqual(&a.upper, b)
}
func (m *BinaryIntervalManager) BoundPrecedes(a *BinaryRational, b *BinaryInterval) (bool, error) {
	if e := m.check(b); e != nil {
		return false, e
	}
	return m.bounds.LessEqual(a, &b.lower)
}
func (m *BinaryIntervalManager) Add(a, b, c *BinaryInterval) error {
	if e := m.triple(a, b, c); e != nil {
		return e
	}
	if e := m.bounds.Add(&a.lower, &b.lower, &c.lower); e != nil {
		return e
	}
	return m.bounds.Add(&a.upper, &b.upper, &c.upper)
}
func (m *BinaryIntervalManager) AddInteger(a *BinaryInterval, n *Int, c *BinaryInterval) error {
	if e := m.pair(a, c); e != nil {
		return e
	}
	if e := m.bounds.Z.check(n); e != nil {
		return e
	}
	if e := m.bounds.AddInteger(&a.lower, n, &c.lower); e != nil {
		return e
	}
	return m.bounds.AddInteger(&a.upper, n, &c.upper)
}
func (m *BinaryIntervalManager) Sub(a, b, c *BinaryInterval) error {
	if e := m.triple(a, b, c); e != nil {
		return e
	}
	if e := m.bounds.Sub(&a.lower, &b.upper, &c.lower); e != nil {
		return e
	}
	return m.bounds.Sub(&a.upper, &b.lower, &c.upper)
}
func (m *BinaryIntervalManager) initMulMaxMin() error {
	if e := m.bounds.Copy(&m.MulMin, &m.MulCurr); e != nil {
		return e
	}
	return m.bounds.Swap(&m.MulMax, &m.MulCurr)
}
func (m *BinaryIntervalManager) updateMulMaxMin() error {
	ok, e := m.bounds.Less(&m.MulCurr, &m.MulMin)
	if e != nil {
		return e
	}
	if ok {
		if e = m.bounds.Copy(&m.MulMin, &m.MulCurr); e != nil {
			return e
		}
	}
	ok, e = m.bounds.Greater(&m.MulCurr, &m.MulMax)
	if e != nil {
		return e
	}
	if ok {
		return m.bounds.Swap(&m.MulMax, &m.MulCurr)
	}
	return nil
}
func (m *BinaryIntervalManager) Mul(a, b, c *BinaryInterval) error {
	if e := m.triple(a, b, c); e != nil {
		return e
	}
	if e := m.bounds.Mul(&a.lower, &b.lower, &m.MulCurr); e != nil {
		return e
	}
	if e := m.initMulMaxMin(); e != nil {
		return e
	}
	if e := m.bounds.Mul(&a.lower, &b.upper, &m.MulCurr); e != nil {
		return e
	}
	if e := m.updateMulMaxMin(); e != nil {
		return e
	}
	if e := m.bounds.Mul(&a.upper, &b.lower, &m.MulCurr); e != nil {
		return e
	}
	if e := m.updateMulMaxMin(); e != nil {
		return e
	}
	if e := m.bounds.Mul(&a.upper, &b.upper, &m.MulCurr); e != nil {
		return e
	}
	if e := m.updateMulMaxMin(); e != nil {
		return e
	}
	if e := m.bounds.Swap(&c.lower, &m.MulMin); e != nil {
		return e
	}
	return m.bounds.Swap(&c.upper, &m.MulMax)
}
func (m *BinaryIntervalManager) Power(a *BinaryInterval, n uint32, c *BinaryInterval) error {
	if e := m.pair(a, c); e != nil {
		return e
	}
	if e := m.bounds.Power(&a.lower, n, &c.lower); e != nil {
		return e
	}
	if e := m.bounds.Power(&a.upper, n, &c.upper); e != nil {
		return e
	}
	if n%2 == 1 {
		return nil
	}
	ok, e := m.bounds.IsNonNeg(&a.lower)
	if e != nil || ok {
		return e
	}
	ok, e = m.bounds.IsNeg(&a.upper)
	if e != nil {
		return e
	}
	if ok {
		return m.bounds.Swap(&c.lower, &c.upper)
	}
	ok, e = m.bounds.Greater(&c.lower, &c.upper)
	if e != nil {
		return e
	}
	if ok {
		if e = m.bounds.Swap(&c.lower, &c.upper); e != nil {
			return e
		}
	}
	return m.bounds.Reset(&c.lower)
}
