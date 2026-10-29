package numeral

// Source: src/util/mpq.h:199-201.
// Source: src/util/mpq.h; src/util/mpq.cpp.
// Source: src/util/mpq.h and src/util/mpq.cpp.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrUninitializedRational = errors.New("mpnum: rational has zero denominator; use NewRational")
var ErrRationalHWFConversionPending = errors.New("mpnum: hardware float rational conversion cleanup pending")

// Rational is the source pair of pointer-free, owner-local integer numerals.
// NewRational is required: a raw Go zero value has an invalid denominator.
// Big values must never be shallow-copied or used with another manager.
type Rational struct{ Num, Den Int }

func NewRational() Rational       { return Rational{Den: NewInt(1)} }
func (a *Rational) IsSmall() bool { return a.Num.IsSmall() && a.Den.IsSmall() }

type RationalConfig struct {
	Integers     IntegerConfig
	Synchronized bool
	onAllocated  func(*RationalManager)
}

// RationalManager owns an IntegerManager and the source scalar scratch fields.
// Synchronized selects source local-temporary algorithms; false selects the
// source reusable scratch algorithms. Both are owner-confined, not concurrent.
// Manager header bytes include all retained scalar rational scratch members.
type RationalManager struct {
	decimalStream                                  rationalDecimalStreamScope
	smt2Stream                                     rationalSMT2StreamScope
	stringScope                                    rationalOwnedStringScope
	mpfParent                                      *ArbitraryFloatManager
	lexical                                        rationalManagerLexicalScope
	RawArithmetic                                  rationalRawArithmeticScope
	RawGeneration                                  uint64
	prevPowerTemporary                             Int
	prevPowerPending                               bool
	addIntTemporary                                Int
	addIntPending                                  bool
	fusedTemporary                                 Rational
	fusedPending, fusedExecuting                   bool
	fusedCursor                                    uint8
	parseOuterTemporary, parseSuffixTemporary      Int
	parseOuterPending, parseSuffixPending          bool
	parseB, parseExp, parseTen                     rationalParserScopedValue
	hwfN, hwfD                                     Int
	hwfPending, hwfExecuting                       bool
	hwfCursor                                      uint8
	self                                           *RationalManager
	originalCause, CleanupCauseField               error
	cleanupStage                                   uint8
	Closing                                        bool
	SourceFinalizing, SourceFinalized, sourceGuard bool
	sourceCause                                    error
	Z                                              *IntegerManager
	NTmp, AddTmp1, AddTmp2                         Int
	AddMulTmp, LtTmp1, LtTmp2                      Rational
	Synchronized, closed                           bool
	BudgetField                                    *arena.Budget
	HeaderBytes                                    uint64
}

func NewRationalManager(config RationalConfig) (*RationalManager, error) {
	if config.Integers.Cells.Budget != config.Integers.Words.Descriptors.Budget {
		return nil, arena.ErrConfig
	}
	budget := config.Integers.Cells.Budget
	header := uint64(reflect.TypeOf(RationalManager{}).Size())
	if e := budget.Reserve(header); e != nil {
		return nil, e
	}
	m := &RationalManager{Synchronized: config.Synchronized, BudgetField: budget, HeaderBytes: header, AddMulTmp: NewRational(), LtTmp1: NewRational(), LtTmp2: NewRational()}
	m.self = m
	if config.onAllocated != nil {
		config.onAllocated(m) // source alloc(synch_mpq_manager) precedes child construction
	}
	z, e := NewIntegerManager(config.Integers)
	if e != nil {
		budget.Release(header)
		return nil, e
	}
	m.Z = z
	return m, nil
}
func (m *RationalManager) Integers() *IntegerManager { return m.Z }

// checkOwnerReady guards the cold owner without reading either numeral field.
func (m *RationalManager) checkOwnerReady() error {
	if m == nil || m.self != m || m.Z == nil || m.Z.self != m.Z {
		return umemory.ErrRuntimeOwner
	}
	if m.closed || m.Closing || m.SourceFinalizing || m.SourceFinalized || m.Z.closed || m.Z.SourceFinalizing || m.Z.constructionFailed {
		return arena.ErrClosed
	}
	if m.parseOuterPending || m.parseSuffixPending || m.parseB.pending || m.parseExp.pending || m.parseTen.pending || m.prevPowerPending || m.addIntPending || (m.fusedPending && !m.fusedExecuting) {
		return ErrRationalLexicalPending
	}
	if m.hwfPending && !m.hwfExecuting {
		return ErrRationalHWFConversionPending
	}
	return nil
}
func (m *RationalManager) check(a *Rational) error {
	if e := m.checkOwnerReady(); e != nil {
		return e
	}
	if a == nil {
		return arena.ErrInvalidSlot
	}
	if e := m.Z.check(&a.Num); e != nil {
		return e
	}
	if e := m.Z.check(&a.Den); e != nil {
		return e
	}
	if a.Den.Val == 0 {
		return ErrUninitializedRational
	}
	return nil
}
func (m *RationalManager) checkCleanup(a *Rational) error {
	if e := m.checkOwnerReady(); e != nil {
		return e
	}
	if a == nil {
		return arena.ErrInvalidSlot
	}
	if e := m.Z.checkCleanup(&a.Num); e != nil {
		return e
	}
	if e := m.Z.checkCleanup(&a.Den); e != nil {
		return e
	}
	// Cleanup/reset acts on owned components, including finite source results
	// with denominator zero. This does not authorize ordinary numeral reads.
	return nil
}
func (m *RationalManager) drop(a *Rational) { m.Z.Del(&a.Num); m.Z.Del(&a.Den) }
func (m *RationalManager) Close() {
	if e := m.CloseChecked(); e != nil {
		panic(m)
	}
}
func (m *RationalManager) resetDenominator(a *Rational) error {
	if e := m.Z.Del(&a.Den); e != nil {
		return e
	}
	a.Den.Val = 1
	return nil
}
func isIntegerRational(a *Rational) bool { return a.Den.IsSmall() && a.Den.Val == 1 }
func isOneInteger(a *Int) bool           { return a.IsSmall() && a.Val == 1 }

func (m *RationalManager) Del(a *Rational) error {
	if e := m.checkCleanup(a); e != nil {
		return e
	}
	if e := m.Z.Del(&a.Num); e != nil {
		return e
	}
	return m.Z.Del(&a.Den)
}
func (m *RationalManager) Reset(a *Rational) error {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	if e := m.Z.Reset(&a.Num); e != nil {
		return e
	}
	return m.resetDenominator(a)
}
func (m *RationalManager) Neg(a *Rational) error {
	return m.negWithSourceContext(a, integerAllocationInvocationContext{})
}
func (m *RationalManager) negWithSourceContext(a *Rational, context integerAllocationInvocationContext) error {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	return m.Z.negWithAllocationContext(&a.Num, context)
}
func (m *RationalManager) Abs(a *Rational) error {
	return m.absWithAllocationContext(a, integerAllocationInvocationContext{})
}
func (m *RationalManager) absWithAllocationContext(a *Rational, context integerAllocationInvocationContext) error {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	return m.Z.absWithAllocationContext(&a.Num, context)
}
func (m *RationalManager) Move(dst, src *Rational) error {
	if e := m.checkScalarValue(dst); e != nil {
		return e
	}
	if e := m.checkScalarValue(src); e != nil {
		return e
	}
	if dst == src {
		return nil
	}
	if e := m.Z.Move(&dst.Num, &src.Num); e != nil {
		return e
	}
	return m.Z.Move(&dst.Den, &src.Den)
}
func (m *RationalManager) Swap(a, b *Rational) error {
	if e := m.checkScalarValue(a); e != nil {
		return e
	}
	if e := m.checkScalarValue(b); e != nil {
		return e
	}
	if e := m.Z.Swap(&a.Num, &b.Num); e != nil {
		return e
	}
	return m.Z.Swap(&a.Den, &b.Den)
}
func (m *RationalManager) Copy(dst, src *Rational) error {
	return m.copyWithAllocationContext(dst, src, integerAllocationInvocationContext{})
}
func (m *RationalManager) copyWithAllocationContext(dst, src *Rational, context integerAllocationInvocationContext) error {
	return m.copyWithAllocationReceipt(dst, src, context, nil)
}
func (m *RationalManager) copyWithAllocationReceipt(dst, src *Rational, context integerAllocationInvocationContext, receipt *integerCopyAllocationReceipt) error {
	if e := m.checkScalarValue(dst); e != nil {
		return e
	}
	if e := m.checkScalarValue(src); e != nil {
		return e
	}
	if e := m.Z.copyFromOwnerWithAllocationContext(&dst.Num, m.Z, &src.Num, receipt, context); e != nil {
		return e
	}
	return m.Z.copyFromOwnerWithAllocationContext(&dst.Den, m.Z, &src.Den, receipt, context)
}

// checkSetter authorizes explicit initialization/reinitialization, including
// inline denominator0. Failed/stale Integer handles remain ordinary errors;
// arithmetic and source-read checks retain their separate denominator policy.
func (m *RationalManager) checkSetter(a *Rational) error {
	return m.checkIntegerFields(a)
}

// checkIntegerFields validates actual component ownership without imposing an
// ordinary rational-read domain. Source Mul/Div can consume and publish zero
// denominators; their arithmetic determines the first endangered operation.
func (m *RationalManager) checkIntegerFields(a *Rational) error {
	if e := m.checkOwnerReady(); e != nil {
		return e
	}
	if a == nil {
		return arena.ErrInvalidSlot
	}
	if e := m.Z.check(&a.Num); e != nil {
		return e
	}
	return m.Z.check(&a.Den)
}
func (m *RationalManager) SetInt32(a *Rational, n int32) error {
	if e := m.checkSetter(a); e != nil {
		return e
	}
	if e := m.Z.SetInt32(&a.Num, n); e != nil {
		return e
	}
	return m.resetDenominator(a)
}
func (m *RationalManager) SetUint32(a *Rational, n uint32) error {
	if e := m.checkSetter(a); e != nil {
		return e
	}
	if e := m.Z.SetUint32(&a.Num, n); e != nil {
		return e
	}
	return m.resetDenominator(a)
}
func (m *RationalManager) SetInt64(a *Rational, n int64) error {
	return m.setInt64WithAllocationContext(a, n, integerAllocationInvocationContext{})
}
func (m *RationalManager) setInt64WithAllocationContext(a *Rational, n int64, context integerAllocationInvocationContext) error {
	if e := m.checkSetter(a); e != nil {
		return e
	}
	if e := m.Z.setInt64WithAllocationContext(&a.Num, n, context); e != nil {
		return e
	}
	return m.resetDenominator(a)
}
func (m *RationalManager) SetFraction(a *Rational, n int64, d uint64) error {
	if e := m.checkSetter(a); e != nil {
		return e
	}
	if e := rationalFractionDenominatorAssertion(d != 0, 690); e != nil {
		return e
	}
	if e := m.Z.SetInt64(&a.Num, n); e != nil {
		return e
	}
	if e := m.Z.SetUint64(&a.Den, d); e != nil {
		return e
	}
	return m.normalize(a)
}

// SetFractionSigned is the source signed32 pair overload. Widening the two
// local negations defines intended INT_MIN magnitudes where C++ unary signed
// negation is undefined; it preserves the existing service adapter.
func (m *RationalManager) SetFractionSigned(a *Rational, n, d int32) error {
	if e := m.checkSetter(a); e != nil {
		return e
	}
	if e := rationalFractionDenominatorAssertion(d != 0, 679); e != nil {
		return e
	}
	nn, dd := int64(n), int64(d)
	if dd < 0 {
		nn = -nn
		dd = -dd
	}
	if e := m.Z.SetInt64(&a.Num, nn); e != nil {
		return e
	}
	if e := m.Z.SetInt64(&a.Den, dd); e != nil {
		return e
	}
	return m.normalize(a)
}
func (m *RationalManager) SetParts(a *Rational, n, d *Int) error {
	return m.setPartsFromIntegerOwner(a, n, d, m.Z)
}
func (m *RationalManager) setPartsFromIntegerOwner(a *Rational, n, d *Int, sourceOwner *IntegerManager) error {
	return m.setPartsFromIntegerOwnerWithAllocationContext(a, n, d, sourceOwner, integerAllocationInvocationContext{})
}
func (m *RationalManager) setPartsFromIntegerOwnerWithAllocationContext(a *Rational, n, d *Int, sourceOwner *IntegerManager, context integerAllocationInvocationContext) error {
	return m.setPartsWithInvocation(a, n, d, sourceOwner, context, nil)
}

// RationalPartsInvocation retains only allocations actually issued by SET and
// the reached synchronized normalization scope.
type RationalPartsInvocation struct {
	Numerator, Denominator IntegerCopyAllocationReceipt
	Normalize              RationalNormalizeInvocationReceipt
}

func (m *RationalManager) SetPartsWithInvocation(a *Rational, n, d *Int, context NumericAllocationContext, receipt *RationalPartsInvocation) error {
	return m.setPartsWithInvocation(a, n, d, m.Z, context, receipt)
}
func (m *RationalManager) setPartsWithInvocation(a *Rational, n, d *Int, sourceOwner *IntegerManager, context integerAllocationInvocationContext, receipt *RationalPartsInvocation) error {
	if e := m.checkSetter(a); e != nil {
		return e
	}
	if sourceOwner == nil || sourceOwner.self != sourceOwner {
		return umemory.ErrRuntimeOwner
	}
	if e := sourceOwner.check(d); e != nil {
		return e
	}
	negative := d.Val < 0 // source branch decision precedes both caller Copies
	copyContext := context
	var nr, dr *IntegerCopyAllocationReceipt
	var norm *RationalNormalizeInvocationReceipt
	if receipt != nil {
		nr, dr, norm = &receipt.Numerator, &receipt.Denominator, &receipt.Normalize
		// The copy receipt owns its exact physical allocation independently.
		copyContext = integerAllocationInvocationContext{}
	}
	if e := m.Z.copyFromOwnerWithAllocationContext(&a.Num, sourceOwner, n, nr, copyContext); e != nil {
		return e
	}
	if e := m.Z.copyFromOwnerWithAllocationContext(&a.Den, sourceOwner, d, dr, copyContext); e != nil {
		return e
	}
	if negative {
		if e := m.Z.negWithAllocationContext(&a.Num, context); e != nil {
			return e
		}
		if e := m.Z.negWithAllocationContext(&a.Den, context); e != nil {
			return e
		}
	}
	return m.normalizeWithAllocationContextAndReceipt(a, context, norm)
}
func (m *RationalManager) normalize(a *Rational) (_rawErr error) {
	return m.normalizeWithAllocationContext(a, integerAllocationInvocationContext{})
}
func (m *RationalManager) normalizeWithAllocationContext(a *Rational, context integerAllocationInvocationContext) (_rawErr error) {
	return m.normalizeWithAllocationContextAndReceipt(a, context, nil)
}
func (m *RationalManager) normalizeWithAllocationContextAndReceipt(a *Rational, context integerAllocationInvocationContext, receipt *RationalNormalizeInvocationReceipt) (_rawErr error) {
	if call := context.rationalPower; call != nil && call.valid() && call.executing && call.manager == m && call.power.executing {
		receipt = &call.normalize
	}

	tmp := &m.NTmp
	if m.Synchronized {
		r := &m.RawArithmetic
		if r.NormActive {
			return ErrRationalRawArithmeticPending
		}
		generation, e := m.beginRawGeneration()
		if e != nil {
			return e
		}
		r.normGeneration = generation
		if c := context.rationalServiceFrame(m.Z); c != nil {
			c.normGeneration = generation
		}
		r.Norm = Int{}
		r.NormActive = true
		r.normEnd = false
		r.normGuard = false
		r.normCause = nil
		tmp = &r.Norm
		if receipt != nil {
			*receipt = RationalNormalizeInvocationReceipt{manager: m, generation: generation, executing: true}
			defer func() {
				if r.normGeneration == generation && r.normEnd && r.normCause != nil {
					receipt.sourceCause = r.normCause
				}
				receipt.executing = false
			}()
		}
		defer m.finishRawNormalize(&_rawErr)
	}
	if e := m.Z.gcdWithAllocationContext(&a.Num, &a.Den, tmp, context); e != nil {
		return e
	}
	if !isOneInteger(tmp) {
		if e := m.Z.divWithAllocationContext(&a.Num, tmp, &a.Num, context); e != nil {
			return e
		}
		if e := m.Z.divWithAllocationContext(&a.Den, tmp, &a.Den, context); e != nil {
			return e
		}
	}
	if m.Synchronized {
		return m.rawNormalizeEnd()
	}
	return nil
}

// Inv copies into the actual destination before the source in-place action.
func (m *RationalManager) Inv(a, out *Rational) error {
	if e := m.Copy(out, a); e != nil {
		return e
	}
	return m.InvInPlace(out)
}

// InvInPlace retains the source debug assertion and release finite zero Swap.
// A resulting denominator0 is invalid for ordinary reads but remains owned and
// can be destroyed or explicitly Reset using cleanup-only validation.
func (m *RationalManager) InvInPlace(a *Rational) error {
	return m.invInPlaceWithAllocationContext(a, integerAllocationInvocationContext{})
}
func (m *RationalManager) invInPlaceWithAllocationContext(a *Rational, context integerAllocationInvocationContext) error {
	if e := m.check(a); e != nil {
		return e
	}
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled && a.Num.Val == 0 {
			if e := diagnostic.DebugAssertionFailure(d, "src/util/mpq.h", 444, "!is_zero(a)"); e != nil {
				return e
			}
		}
	}
	if a.Num.Val < 0 {
		if e := m.Z.negWithAllocationContext(&a.Num, context); e != nil {
			return e
		}
		if e := m.Z.negWithAllocationContext(&a.Den, context); e != nil {
			return e
		}
	}
	return m.Z.Swap(&a.Num, &a.Den)
}

func (m *RationalManager) Equal(a, b *Rational) (bool, error) {
	if e := m.checkScalarValue(a); e != nil {
		return false, e
	}
	if e := m.checkScalarValue(b); e != nil {
		return false, e
	}
	n, e := m.Z.Compare(&a.Num, &b.Num)
	if e != nil || n != 0 {
		return false, e
	}
	d, e := m.Z.Compare(&a.Den, &b.Den)
	return d == 0, e
}

// Compare preserves source rat_lt's sign shortcuts and exact cross-products.
func (m *RationalManager) Compare(a, b *Rational) (int, error) {
	return m.compareWithAllocationContext(a, b, integerAllocationInvocationContext{})
}
func rationalCompareSourceAssertion(predicate bool, line int32, expression string) error {
	if !modularSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil {
		return e
	}
	if enabled && !predicate {
		return diagnostic.DebugAssertionFailure(d, "src/util/mpq.cpp", line, expression)
	}
	return nil
}
func (m *RationalManager) compareWithAllocationContext(a, b *Rational, context integerAllocationInvocationContext) (_rawValue int, _rawErr error) {
	if e := m.checkScalarValue(a); e != nil {
		return 0, e
	}
	if e := m.checkScalarValue(b); e != nil {
		return 0, e
	}
	if isIntegerRational(a) && isIntegerRational(b) {
		return m.Z.Compare(&a.Num, &b.Num)
	}
	sa, sb := a.Num.Val, b.Num.Val
	if sa < 0 && sb >= 0 {
		return -1, nil
	}
	if sa == 0 {
		if sb > 0 {
			return -1, nil
		}
		if e := rationalCompareSourceAssertion(sb <= 0, 49, "sign_b <= 0"); e != nil {
			return 0, e
		}
		if sb < 0 {
			return 1, nil
		}
		return 0, nil
	}
	if sa > 0 {
		if e := rationalCompareSourceAssertion(sa > 0, 52, "sign_a > 0"); e != nil {
			return 0, e
		}
		if sb <= 0 {
			return 1, nil
		}
	}
	if sa < 0 && sb == 0 {
		return -1, nil
	}
	if e := rationalCompareSourceAssertion((sa > 0 && sb > 0) || (sa < 0 && sb < 0), 55, "(sign_a > 0 && sign_b > 0) || (sign_a < 0 && sign_b < 0)"); e != nil {
		return 0, e
	}
	one, two := &m.LtTmp1, &m.LtTmp2
	if m.Synchronized {
		r := &m.RawArithmetic
		if r.CompareActive {
			return 0, ErrRationalRawArithmeticPending
		}
		generation, e := m.beginRawGeneration()
		if e != nil {
			return 0, e
		}
		r.compareGeneration = generation
		if c := context.rationalServiceFrame(m.Z); c != nil {
			c.compareGeneration = generation
		}
		r.CompareOne, r.CompareTwo = NewRational(), NewRational()
		r.CompareActive = true
		r.compareEnd = false
		r.compareGuard = false
		r.compareCursor = 0
		r.compareCause = nil
		one, two = &r.CompareOne, &r.CompareTwo
		defer m.finishRawCompare(&_rawErr)
	}
	if e := m.Z.mulWithAllocationContext(&a.Num, &b.Den, &one.Num, context); e != nil {
		return 0, e
	}
	if e := m.resetDenominator(one); e != nil {
		return 0, e
	}
	if e := m.Z.mulWithAllocationContext(&b.Num, &a.Den, &two.Num, context); e != nil {
		return 0, e
	}
	if e := m.resetDenominator(two); e != nil {
		return 0, e
	}
	value, e := m.Z.Compare(&one.Num, &two.Num)
	if e != nil {
		return 0, e
	}
	if m.Synchronized {
		if e := m.rawCompareEnd(); e != nil {
			return 0, e
		}
	}
	return value, nil
}

func (m *RationalManager) Add(a, b, out *Rational) error { return m.addSub(a, b, out, false) }
func (m *RationalManager) Sub(a, b, out *Rational) error { return m.addSub(a, b, out, true) }
func (m *RationalManager) addSub(a, b, out *Rational, subtract bool) error {
	return m.addSubWithAllocationContext(a, b, out, subtract, integerAllocationInvocationContext{})
}
func (m *RationalManager) addSubWithAllocationContext(a, b, out *Rational, subtract bool, context integerAllocationInvocationContext) (_rawErr error) {
	for _, n := range []*Rational{a, b, out} {
		if e := m.checkScalarValue(n); e != nil {
			return e
		}
	}
	op := "+"
	if subtract {
		op = "-"
	}
	m.traceMPQBinary(a, b, out, op, "mpq", true)
	defer func() {
		if _rawErr == nil {
			m.traceMPQBinary(a, b, out, op, "mpq", false)
		}
	}()
	if isIntegerRational(a) && isIntegerRational(b) {
		var e error
		if subtract {
			e = m.Z.addSubWithAllocationContext(&a.Num, &b.Num, &out.Num, true, context)
		} else {
			e = m.Z.addSubWithAllocationContext(&a.Num, &b.Num, &out.Num, false, context)
		}
		if e != nil {
			return e
		}
		if e = m.resetDenominator(out); e != nil {
			return e
		}
		return nil
	}
	m.traceMPQBinary(a, b, out, op, "rat_mpq", true)
	defer func() {
		if _rawErr == nil {
			m.traceMPQBinary(a, b, out, op, "rat_mpq", false)
		}
	}()
	one, two := &m.AddTmp1, &m.AddTmp2
	if m.Synchronized {
		r := &m.RawArithmetic
		if r.AddActive {
			return ErrRationalRawArithmeticPending
		}
		generation, e := m.beginRawGeneration()
		if e != nil {
			return e
		}
		r.addGeneration = generation
		if c := context.rationalServiceFrame(m.Z); c != nil {
			c.addGeneration = generation
		}
		r.AddOne = Int{}
		r.AddTwo = Int{}
		r.AddActive = true
		r.addEnd = false
		r.addGuard = false
		r.addCursor = 0
		r.addCause = nil
		one, two = &r.AddOne, &r.AddTwo
		defer m.finishRawAdd(&_rawErr)
	}
	if e := m.Z.mulWithAllocationContext(&a.Num, &b.Den, one, context); e != nil {
		return e
	}
	if e := m.Z.mulWithAllocationContext(&b.Num, &a.Den, two, context); e != nil {
		return e
	}
	if e := m.Z.mulWithAllocationContext(&a.Den, &b.Den, &out.Den, context); e != nil {
		return e
	}
	var e error
	if subtract {
		e = m.Z.addSubWithAllocationContext(one, two, &out.Num, true, context)
	} else {
		e = m.Z.addSubWithAllocationContext(one, two, &out.Num, false, context)
	}
	if e != nil {
		return e
	}
	if e = m.normalizeWithAllocationContext(out, context); e != nil {
		return e
	}
	if m.Synchronized {
		return m.rawAddEnd()
	}
	return nil
}
func (m *RationalManager) Mul(a, b, out *Rational) error {
	return m.mulWithAllocationContext(a, b, out, integerAllocationInvocationContext{})
}
func (m *RationalManager) mulWithAllocationContext(a, b, out *Rational, context integerAllocationInvocationContext) (err error) {
	for _, n := range []*Rational{a, b, out} {
		if e := m.checkScalarValue(n); e != nil {
			return e
		}
	}
	m.traceMPQBinary(a, b, out, "*", "mpq", true)
	defer func() {
		if err == nil {
			m.traceMPQBinary(a, b, out, "*", "mpq", false)
		}
	}()
	// Source chooses this branch before output aliases can mutate operands.
	integers, e := m.IsInt(a)
	if e != nil {
		return e
	}
	if integers {
		integers, e = m.IsInt(b)
		if e != nil {
			return e
		}
	}
	if !integers {
		m.traceMPQBinary(a, b, out, "*", "rat_mpq", true)
		defer func() {
			if err == nil {
				m.traceMPQBinary(a, b, out, "*", "rat_mpq", false)
			}
		}()
	}
	if e := m.Z.mulWithAllocationContext(&a.Num, &b.Num, &out.Num, context); e != nil {
		return e
	}
	if integers {
		return m.resetDenominator(out)
	}
	if e := m.Z.mulWithAllocationContext(&a.Den, &b.Den, &out.Den, context); e != nil {
		return e
	}
	return m.normalizeWithAllocationContext(out, context)
}

// Div preserves the actual divisor/destination branch and component order.
// Errors retain completed caller fields; no staged pair masks normalization.
func (m *RationalManager) Div(a, b, out *Rational) error {
	return m.divWithAllocationContext(a, b, out, integerAllocationInvocationContext{})
}
func (m *RationalManager) divWithAllocationContext(a, b, out *Rational, context integerAllocationInvocationContext) (_rawErr error) {
	for _, n := range []*Rational{a, b, out} {
		if e := m.checkScalarValue(n); e != nil {
			return e
		}
	}
	m.traceMPQBinary(a, b, out, "/", "mpq", true)
	defer func() {
		if _rawErr == nil {
			m.traceMPQBinary(a, b, out, "/", "mpq", false)
		}
	}()
	if b == out {
		r := &m.RawArithmetic
		if r.DivActive {
			return ErrRationalRawArithmeticPending
		}
		generation, e := m.beginRawGeneration()
		if e != nil {
			return e
		}
		r.divGeneration = generation
		r.Div = Int{}
		r.DivActive, r.divEnd, r.divGuard = true, false, false
		r.divCursor, r.divCause = 0, nil
		defer m.finishRawDiv(&_rawErr)
		if e := m.Z.mulWithAllocationContext(&a.Num, &b.Den, &r.Div, context); e != nil {
			return e
		}
		if e := m.Z.mulWithAllocationContext(&a.Den, &b.Num, &out.Den, context); e != nil {
			return e
		}
		if e := m.Z.copyFromOwnerWithAllocationContext(&out.Num, m.Z, &r.Div, nil, context); e != nil {
			return e
		}
		if e := m.rawDivEnd(); e != nil {
			return e
		}
	} else {
		if e := m.Z.mulWithAllocationContext(&a.Num, &b.Den, &out.Num, context); e != nil {
			return e
		}
		if e := m.Z.mulWithAllocationContext(&a.Den, &b.Num, &out.Den, context); e != nil {
			return e
		}
	}
	if out.Den.Val < 0 {
		if e := m.Z.negWithAllocationContext(&out.Num, context); e != nil {
			return e
		}
		if e := m.Z.negWithAllocationContext(&out.Den, context); e != nil {
			return e
		}
	}
	return m.normalizeWithAllocationContext(out, context)
}
