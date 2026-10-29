package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1565-1577,2933-2935.
// The standalone numerical core borrows services and materializes workspaces
// on demand. AlgebraicManager supplies eager source-ordered ownership around
// these same numerical operations.

import (
	"errors"
	"math"
	"reflect"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

var ErrAlgebraicOwner = errors.New("mpnum: invalid algebraic owner or value")
var ErrAlgebraicAssertion = errors.New("mpnum: algebraic source assertion")
var ErrAlgebraicLive = errors.New("mpnum: live caller algebraic owners")
var ErrAlgebraicPending = errors.New("mpnum: algebraic cleanup pending")
var algebraicOrigins atomic.Uint64

// Null is source rational zero. Owning nonzero values must not be shallow-copied.
type Algebraic struct {
	owner            uint64
	slot, generation uint32
}
type AlgebraicRootConfig struct {
	Rationals   *unumeral.RationalManager
	Polynomials *UPolynomialManager
	Factors     *UPolynomialFactorizationService
	Binary      *unumeral.BinaryRationalManager
	Intervals   *unumeral.BinaryIntervalManager
	Allocator   *umemory.SmallAllocator
	Checkpoint  func() bool
	Cooperation func() error
}

// Only checked locations live here. Numeral fields are encoded in real object
// bytes; coefficient arrays are actual owned UPolynomial typed backings.
type algebraicLocation struct {
	raw                                       umemory.ObjectRef
	coefficientInitialized, coefficientCursor uint32
	sourceGuard                               bool
	generation                                uint32
	kind, stage                               uint8
	live, initialized, published              bool
}
type AlgebraicRootCore struct {
	basicCellCopyRunning                                       bool
	cellCoefficientCopy                                        algebraicCellCoefficientCopy
	cellBoundsCall                                             algebraicCellBoundsCallScope
	copyCall                                                   algebraicCopyCallScope
	cellNeg                                                    algebraicCellNegNumeric
	cellEvaluation                                             [3]upolyBinaryEvaluationInvocationReceipt
	cellEvaluationRunning                                      bool
	isolationSquareFree                                        upolySquareFreeCallReceipt
	isolationNumeric                                           algebraicIsolationNumericScope
	factorExecuting                                            bool
	coefficientNormalization                                   upolyDefaultNormalizeReceipt
	univariateIsolationRunning                                 bool
	univariateIsolationOutput                                  *AlgebraicVector
	rootTrace                                                  algebraicRootTraceScope
	isolateRefinable                                           upolyRefinableInvocationReceipt
	toOwnedRational                                            algebraicOwnedRationalScope
	toRationalCopy                                             unumeral.IntegerCopyAllocationReceipt
	toRationalRunning                                          bool
	refinementQ                                                algebraicRefinementQScope
	predicateQ                                                 algebraicPredicateQScope
	constQ                                                     algebraicConstQScope
	refinementConversion                                       algebraicConversionScope
	refinement                                                 algebraicRefinementScope
	representationNeg                                          algebraicRepresentationNegNumeric
	representationNormalize                                    unumeral.IntegerNormalizeInvocationReceipt
	representationCopy                                         unumeral.IntegerCopyAllocationReceipt
	intervalDisplayInsertions                                  [2]unumeral.NumeralInsertionReceipt
	decimalDisplay                                             algebraicDecimalDisplayScope
	intervalDisplayRunning, decimalDisplayRunning              bool
	comparisonTraceRunning                                     bool
	savedIntervalExecuting                                     *algebraicSavedInterval
	predicateDiagnosticRunning                                 bool
	predicateTrace                                             algebraicPredicateTraceScope
	rationalCopy                                               unumeral.IntegerCopyAllocationReceipt
	manager                                                    *AlgebraicManager
	coefficientCopy                                            unumeral.IntegerCopyAllocationReceipt
	foreignCopy                                                algebraicForeignCopyScope
	foreignCopyBorrowers                                       uint32
	cellAllocation                                             umemory.SmallAllocatorAllocationOutcome
	cellPending                                                umemory.SmallAllocatorPendingAllocation
	cellAllocationRunning                                      bool
	zero                                                       unumeral.Rational
	rootConstruction                                           algebraicRootConstructionScope
	isolationSigns                                             algebraicIsolationSignsScope
	signEvaluationCaller                                       *algebraicIsolationSignsScope
	localSelection                                             *algebraicSelectionScope
	multivariateIsolationFrames                                []*algebraicMultivariateIsolationFrame
	multivariateIsolationDepth                                 uint32
	multivariateIsolationRunning                               bool
	isolateRootsVariables                                      PolynomialVariableDegrees
	polynomialCaller, localPolynomialCaller                    *polynomialAlgebraicEvaluationScope
	polynomialEvaluationBorrowers, polynomialEvaluationRunning uint32
	integerSet                                                 algebraicIntegerSetScope
	integerAdd                                                 algebraicIntegerAddScope
	localIntegerSet                                            *algebraicIntegerSetScope
	localIntegerAdd                                            *algebraicIntegerAddScope
	isolationConversion                                        algebraicIsolationConversionScope
	isolateTmp1                                                UPolynomial
	filterRootsRunning                                         bool
	signEvaluationManager                                      *MultivariatePolynomialManager
	signEvaluationRunning                                      bool
	signResultant                                              algebraicSignResultantScope
	evalSignTmp                                                UPolynomial
	signInterval                                               algebraicSignIntervalScope
	evalSignVariables                                          PolynomialVariableDegrees
	zeroAccuracy                                               int32
	signSubstitution                                           algebraicSignSubstitutionScope
	rationalSign                                               algebraicRationalSignScope
	rootDisplay                                                algebraicRootDisplayScope
	selection                                                  algebraicSelectionScope
	basicRootScope                                             algebraicBasicRootScope
	division                                                   algebraicDivisionScope
	binary                                                     algebraicBinaryScope
	multivariate                                               *MultivariatePolynomialManager
	resultant                                                  algebraicResultantPolynomialScope
	resultantVariables                                         [2]uint32
	resultantVariableCount                                     uint8
	arithmetic                                                 algebraicRationalArithmeticScope
	addTmp                                                     UPolynomial
	bounds                                                     algebraicBoundScope
	inversion                                                  algebraicInverseScope
	negation                                                   algebraicNegScope
	negationHistory                                            []algebraicNegHistory
	predicate                                                  algebraicPredicateScope
	isRationalTmp                                              unumeral.Int
	rootSelection                                              *AlgebraicVector
	self                                                       *AlgebraicRootCore
	config                                                     AlgebraicRootConfig
	origin, header                                             uint64
	locations                                                  []algebraicLocation
	closed, closing                                            bool
	factors                                                    *UPolynomialFactors
	roots, lowers, uppers                                      *BinaryRationalVector
	tmp2, tmp3                                                 UPolynomial
	pending                                                    Algebraic
	minMagnitude                                               int32
	factor                                                     bool
	factorParams                                               UPolynomialFactorParams
	compareCheap, comparePoly, compareRefine, compareSturm     uint32
	compareSequence                                            *UPolynomialSequence
	callers, vectors                                           uint32
	localQ                                                     unumeral.Rational
	localActive                                                bool
	localOriginal, localCleanup                                error
	constructionStage                                          uint8
}

// Scalar codec is actual Go-adapter extent: basic 24, root 56 bytes. It is not
// C++ sizeof. No duplicate owning numerals exist in the locator directory.
type algebraicRecord struct {
	value       unumeral.Rational
	poly        UPolynomial
	interval    unumeral.BinaryInterval
	flags, size uint32
}

func NewAlgebraicRootCore(c AlgebraicRootConfig) (*AlgebraicRootCore, error) {
	if c.Rationals == nil || c.Polynomials == nil || c.Factors == nil || c.Binary == nil || c.Intervals == nil || c.Allocator == nil || c.Checkpoint == nil || c.Cooperation == nil {
		return nil, arena.ErrConfig
	}
	if e := c.Rationals.CheckOwnerReady(); e != nil {
		return nil, e
	}
	z := c.Rationals.Integers()
	if c.Polynomials.config.Integer != z || c.Factors.manager != c.Polynomials || c.Binary.Integers() != z || c.Intervals.BorrowBounds() != c.Binary {
		return nil, arena.ErrConfig
	}
	if e := c.Polynomials.check(); e != nil {
		return nil, e
	}
	if e := c.Factors.check(); e != nil {
		return nil, e
	}
	if e := c.Binary.Open(); e != nil {
		return nil, e
	}
	if c.Allocator.RuntimeOwner() != z.RuntimeOwner() {
		return nil, arena.ErrConfig
	}
	if _, e := c.Allocator.NumFreeObjects(); e != nil {
		return nil, e
	}
	return allocateAlgebraicRootCore(c)
}
func allocateAlgebraicRootCore(c AlgebraicRootConfig) (*AlgebraicRootCore, error) {
	n := uint64(reflect.TypeOf(AlgebraicRootCore{}).Size())
	if e := c.Rationals.Integers().Budget().Reserve(n); e != nil {
		return nil, e
	}
	var id uint64
	for {
		old := algebraicOrigins.Load()
		if old == math.MaxUint64 {
			c.Rationals.Integers().Budget().Release(n)
			return nil, arena.ErrSlotLimit
		}
		if algebraicOrigins.CompareAndSwap(old, old+1) {
			id = old + 1
			break
		}
	}
	m := &AlgebraicRootCore{config: c, origin: id, header: n, zero: unumeral.NewRational(), minMagnitude: -16, factor: true, factorParams: UPolynomialFactorParams{MaxPrime: 31, PrimeTrials: 1, MaxSearchSize: 5000}}
	m.self = m
	return m, nil
}
func (m *AlgebraicRootCore) check() error {
	if m != nil && m.self == m && (m.basicCellCopyRunning || m.copyCall.callback() || m.cellBoundsCall.callback() || m.cellCoefficientCopy.callback()) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.cellEvaluationPending() || m.cellNeg.pending()) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.config.Polynomials != nil && m.config.Polynomials.squareFreeCallReceiptPending(&m.isolationSquareFree) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && !m.binary.running && m.config.Polynomials != nil && m.config.Polynomials.squareFreeCallReceiptPending(&m.binary.squareFree) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && !m.binary.running && m.binary.conversion.pending(m) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.savedIntervalExecuting != nil || m.factorExecuting || m.cellEvaluationRunning || m.cellNeg.running) {
		return ErrAlgebraicPending
	}

	if m != nil && m.self == m && (m.binary.cleaning || m.division.cleaning || m.integerAdd.cleaning) {
		return ErrAlgebraicPending
	}
	if m.inverseRefinementEvaluationPending() || m.inverseRefinementCopyPending() {
		return ErrAlgebraicPending
	}
	if m.rationalArithmeticEvaluationPending() {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.isolationSigns.traceVariablesConstructed && !m.isolationSigns.traceRunning {
		return ErrAlgebraicPending
	}
	if m.coefficientNormalizationPending() {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.rootTrace.running || m.rootTracePending()) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.config.Polynomials.convertEntryReceiptPending(&m.arithmetic.convertEntry) || m.config.Polynomials.convertEntryReceiptPending(&m.inversion.convertEntry)) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.config.Polynomials.refinableReceiptPending(&m.isolateRefinable) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.foreignCopyBorrowers != 0 || m.foreignCopy.active) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.intervalDisplayInsertions[0].Active() || m.intervalDisplayInsertions[1].Active()) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.rootDisplay.active && !m.rootDisplay.running {
		return ErrAlgebraicPending
	}
	// Display retains cell/coefficients across borrowed stream callbacks.
	// Public reentry must not delete or replace those inputs mid-format.
	if m != nil && m.self == m && (m.rootDisplay.running || m.intervalDisplayRunning || m.decimalDisplayRunning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.toOwnedRational.active && !m.toOwnedRational.running {
		return errors.Join(ErrAlgebraicPending, m.toOwnedRational.sourceCause)
	}
	if m != nil && m.self == m && (m.isolationSigns.traceRunning || m.rootTrace.running || m.comparisonTraceRunning || m.predicateDiagnosticRunning || m.toRationalRunning || m.toOwnedRational.executing) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.cellAllocationRunning || m.cellPending.Block() != (umemory.Block{})) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.binary.executing {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.resultant.executing {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.arithmetic.executing {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.bounds.executing || m.refinement.executing || m.refinementConversion.executing || m.refinementQ.executing || m.predicateQ.executing || m.integerSet.finishing || m.constQ.executing || (m.predicate.finishing || m.predicate.arithmeticExecuting)) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.inversion.executing || m.inversion.numericPending(m)) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.negationExecuting() {
		return ErrAlgebraicPending
	}
	if m == nil || m.self != m {
		return ErrAlgebraicOwner
	}
	if m.closed || m.closing || m.manager != nil && (!m.manager.completed || m.manager.sourceStarted) {
		return arena.ErrClosed
	}
	return m.config.Rationals.CheckOwnerReady()
}
func (m *AlgebraicRootCore) coefficientNormalizationPending() bool {
	return m != nil && m.self == m && m.config.Polynomials != nil && m.config.Polynomials.defaultNormalizationReceiptPending(&m.coefficientNormalization)
}
func (m *AlgebraicRootCore) location(a *Algebraic) (*algebraicLocation, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	return m.locationCleanup(a)
}
func (m *AlgebraicRootCore) locationCleanup(a *Algebraic) (*algebraicLocation, error) {
	if m == nil || m.self != m {
		return nil, ErrAlgebraicOwner
	}
	if m.closed {
		return nil, arena.ErrClosed
	}
	if a == nil {
		return nil, ErrAlgebraicOwner
	}
	if a.slot == 0 {
		if a.owner != 0 || a.generation != 0 {
			return nil, ErrAlgebraicOwner
		}
		return nil, nil
	}
	if a.owner != m.origin || uint64(a.slot) > uint64(len(m.locations)) {
		return nil, ErrAlgebraicOwner
	}
	l := &m.locations[a.slot-1]
	if !l.live || l.generation != a.generation {
		return nil, ErrAlgebraicOwner
	}
	return l, nil
}
func (m *AlgebraicRootCore) allocate(kind uint8) (Algebraic, error) {
	if e := m.check(); e != nil {
		return Algebraic{}, e
	}
	return m.allocateCell(kind)
}

// allocateCell is used only after the enclosing invocation validates its core.
// It preserves the ordinary allocation and publication sequence while public
// callback reentry remains excluded by that invocation's lifetime guard.
func (m *AlgebraicRootCore) allocateCell(kind uint8) (Algebraic, error) {
	i := len(m.locations)
	for j := range m.locations {
		if !m.locations[j].live && m.locations[j].generation != math.MaxUint32 {
			i = j
			break
		}
	}
	if i == len(m.locations) {
		if uint64(len(m.locations)) >= math.MaxUint32 || len(m.locations) == int(^uint(0)>>1) {
			return Algebraic{}, arena.ErrSlotLimit
		}
		m.locations = append(m.locations, algebraicLocation{})
	}
	l := &m.locations[i]
	l.generation++
	l.kind = kind
	l.stage = 0
	l.live = true
	l.initialized = false
	l.coefficientInitialized = 0
	l.coefficientCursor = 0
	l.sourceGuard = false
	l.published = false
	a := Algebraic{m.origin, uint32(i + 1), l.generation}
	n := uint64(24)
	if kind == 1 {
		n = 56
	}
	m.cellAllocationRunning = true
	defer func() { m.cellAllocationRunning = false }()
	raw, e := m.config.Allocator.AllocateWithOutcome(n, &m.cellAllocation)
	l.raw = raw
	if e != nil {
		if raw.LogicalAddress() == 0 {
			l.live = false
		}
		return a, e
	}
	r := algebraicRecord{value: unumeral.NewRational()}
	if e = m.store(l, &r); e != nil {
		return a, e
	}
	l.initialized = true
	return a, nil
}
func algebraicIntWords(a unumeral.Int) [3]uint32 {
	return a.NativeWords()
}
func algebraicWordsInt(w []uint32) unumeral.Int {
	return unumeral.IntFromNativeWords([3]uint32{uint32(int32(w[0])), uint32(uint64(w[1]) | uint64(w[2])<<32), uint32(uint64(uint64(w[1])|uint64(w[2])<<32) >> 32)})
}
func (m *AlgebraicRootCore) store(l *algebraicLocation, r *algebraicRecord) error {
	var w [14]uint32
	n := 6
	if l.kind == 0 {
		a, b := algebraicIntWords((*r.value.BorrowNumerator())), algebraicIntWords((*r.value.BorrowDenominator()))
		copy(w[:3], a[:])
		copy(w[3:6], b[:])
	} else {
		n = 14
		w[0] = uint32(r.poly.origin)
		w[1] = uint32(r.poly.origin >> 32)
		w[2] = r.poly.slot
		w[3] = r.poly.generation
		w[4] = r.flags
		w[5] = r.size
		a, b := algebraicIntWords((*(*r.interval.BorrowLower()).BorrowNumerator())), algebraicIntWords((*(*r.interval.BorrowUpper()).BorrowNumerator()))
		copy(w[6:9], a[:])
		w[9] = (*r.interval.BorrowLower()).K()
		copy(w[10:13], b[:])
		w[13] = (*r.interval.BorrowUpper()).K()
	}
	for i := 0; i < n; i++ {
		if e := m.config.Allocator.WriteUint32(l.raw, uint64(i)*4, w[i]); e != nil {
			return e
		}
	}
	return nil
}
func (m *AlgebraicRootCore) load(l *algebraicLocation) (algebraicRecord, error) {
	r := algebraicRecord{value: unumeral.NewRational()}
	if !l.initialized {
		return r, umemory.ErrRuntimeUninitialized
	}
	var w [14]uint32
	n := 6
	if l.kind == 1 {
		n = 14
	}
	for i := 0; i < n; i++ {
		v, e := m.config.Allocator.ReadUint32(l.raw, uint64(i)*4)
		if e != nil {
			return r, e
		}
		w[i] = v
	}
	if l.kind == 0 {
		r.value = unumeral.RationalFromBorrowedNumerals(algebraicWordsInt(w[:3]), algebraicWordsInt(w[3:6]))
	} else {
		r.poly = UPolynomial{uint64(w[0]) | uint64(w[1])<<32, w[2], w[3]}
		r.flags = w[4]
		r.size = w[5]
		r.interval = unumeral.BinaryIntervalFromBorrowedBounds(unumeral.BinaryRationalFromBorrowedNumeral(algebraicWordsInt(w[6:9]), w[9]), unumeral.BinaryRationalFromBorrowedNumeral(algebraicWordsInt(w[10:13]), w[13]))
	}
	return r, nil
}
func (m *AlgebraicRootCore) IsZero(a *Algebraic) (bool, error) {
	_, e := m.location(a)
	return a != nil && a.slot == 0, e
}
func (m *AlgebraicRootCore) Del(a *Algebraic) error {
	if m.negationProtectsCaller(a) {
		return ErrAlgebraicPending
	}
	l, e := m.location(a)
	if e != nil || l == nil {
		return e
	}
	if e = algebraicQueryTagAssertionSwitch(); e != nil {
		return e
	}
	return m.destroyCell(a, l, false)
}

// Identity-only private destruction remains callable after owning Close stops
// ordinary operations; it retains source Del guards and their completed cursor.
func (m *AlgebraicRootCore) delClosing(a *Algebraic) error {
	if m.negationProtectsCaller(a) {
		return ErrAlgebraicPending
	}
	l, e := m.locationCleanup(a)
	if e != nil || l == nil {
		return e
	}
	if e = algebraicQueryTagAssertionSwitch(); e != nil {
		return e
	}
	return m.destroyCell(a, l, false)
}

// The cursor is cold lifetime metadata for actual completed source actions.
// Owning abort skips only source-unconstructed slots; source Del preserves the
// endangered read and its guard history, without rebuilding an array.
func (m *AlgebraicRootCore) destroyCell(a *Algebraic, l *algebraicLocation, owning bool) error {
	if !owning && !l.initialized {
		l.sourceGuard = true
		return arena.ErrInvalidSlot
	}

	if l.initialized {
		r, e := m.load(l)
		if e != nil {
			return e
		}
		z := m.config.Rationals.Integers()
		if l.kind == 0 {
			for l.stage < 2 {
				n := &(*r.value.BorrowNumerator())
				if l.stage == 1 {
					n = &(*r.value.BorrowDenominator())
				}
				e = deleteAlgebraicCellInteger(z, n, owning)
				if e = m.save(l, &r, e); e != nil {
					return e
				}
				l.stage++
			}
		} else {
			if l.stage == 0 {
				if e = m.destroyPolynomial(l, &r, owning); e != nil {
					return e
				}
				l.stage = 1
			}
			for l.stage < 3 {
				n := &(*(*r.interval.BorrowLower()).BorrowNumerator())
				if l.stage == 2 {
					n = &(*(*r.interval.BorrowUpper()).BorrowNumerator())
				}
				e = deleteAlgebraicCellInteger(z, n, owning)
				if e = m.save(l, &r, e); e != nil {
					return e
				}
				l.stage++
			}
		}
	}
	n := uint64(24)
	if l.kind == 1 {
		n = 56
	}
	if l.raw.LogicalAddress() != 0 {
		progress, e := m.config.Allocator.DeallocateTracked(n, l.raw)
		if progress.PhysicalReleased || progress.PooledLinked {
			l.raw = umemory.ObjectRef{}
			l.initialized = false
		}
		if e != nil {
			return e
		}
	}
	l.live = false
	if l.published {
		m.callers--
		l.published = false
	}
	*a = Algebraic{}
	return nil
}
func (m *AlgebraicRootCore) deleteLocation(a *Algebraic, l *algebraicLocation) error {
	return m.destroyCell(a, l, true)
}
func (m *AlgebraicRootCore) destroyPolynomial(l *algebraicLocation, r *algebraicRecord, owning bool) error {
	p := m.config.Polynomials
	n := r.size
	if owning {
		n = l.coefficientInitialized
	}
	for l.coefficientCursor < n {
		if r.poly.slot == 0 || l.coefficientCursor >= l.coefficientInitialized {
			l.sourceGuard = true
			return arena.ErrBounds
		}
		v, e := m.sourcePolynomial(r)
		if e != nil {
			return e
		}
		a, e := p.read(v, l.coefficientCursor)
		if e != nil {
			l.sourceGuard = true
			return e
		}
		if e = deleteAlgebraicCellInteger(p.config.Integer, a, owning); e != nil {
			return e
		}
		l.coefficientCursor++
	}
	if r.poly.slot != 0 {
		d, e := p.descriptor(r.poly)
		if e != nil {
			return e
		}
		if owning {
			values := p.values(&d.state)
			for i := l.coefficientCursor; uint64(i) < uint64(len(values)); i++ {
				if e = p.config.Integer.Del(&values[i]); e != nil {
					return e
				}
			}
		}
		p.freeBacking(&d.state)
		d.live = false
		if d.generation != math.MaxUint32 {
			d.generation++
		}
		p.live--
		r.poly = UPolynomial{}
	}
	r.size = 0
	l.coefficientCursor = 0
	l.coefficientInitialized = 0
	return m.store(l, r)
}
func (m *AlgebraicRootCore) beginQ() error {
	if m.config.Rationals.Integers().MatchesCopyAllocationInvocation(&m.rationalCopy) {
		return ErrAlgebraicPending
	}
	if m.negation.active {
		return ErrAlgebraicPending
	}
	if m.localActive {
		return ErrAlgebraicPending
	}
	m.localQ = unumeral.NewRational()
	m.localActive = true
	m.localPolynomialCaller = m.polynomialCaller
	m.localSelection = nil
	if m.selection.running {
		m.localSelection = &m.selection
	}
	m.localIntegerSet = nil
	m.localIntegerAdd = nil
	if m.integerSet.running {
		m.localIntegerSet = &m.integerSet
	}
	if m.integerAdd.running {
		m.localIntegerAdd = &m.integerAdd
	}
	m.localOriginal = nil
	m.localCleanup = nil
	return nil
}
func (m *AlgebraicRootCore) finishQ(cause error) error {
	if m.constQ.active {
		return errors.Join(cause, ErrAlgebraicPending)
	}
	if !m.localActive {
		return cause
	}
	m.localOriginal = cause
	e := m.config.Rationals.Del(&m.localQ)
	if e != nil {
		m.localCleanup = e
		if cause != nil {
			return cause
		}
		return e
	}
	m.localActive = false
	m.localOriginal = nil
	m.localCleanup = nil
	return cause
}
func (m *AlgebraicRootCore) publish(a *Algebraic, next Algebraic) {
	*a = next
	l := &m.locations[next.slot-1]
	if !l.published {
		l.published = true
		m.callers++
	}
	if m.pending == next {
		m.pending = Algebraic{}
		m.constructionStage = 0
	}
}
func (m *AlgebraicRootCore) save(l *algebraicLocation, r *algebraicRecord, cause error) error {
	e := m.store(l, r)
	if e != nil {
		return e
	}
	return cause
}

// Const source overload makes the scoped copy BEFORE mutable setter actions.
func (m *AlgebraicRootCore) SetRational(a *Algebraic, q *unumeral.Rational) (err error) {
	if _, err = m.location(a); err != nil {
		return err
	}
	if err = m.beginConstQ(); err != nil {
		return err
	}
	defer func() { m.constQ.running = false; err = m.finishConstQ(err) }()
	err = m.copyConstQ(q)
	if err != nil {
		return err
	}
	return m.SetTake(a, &m.localQ)
}
func (m *AlgebraicRootCore) SetTake(a *Algebraic, q *unumeral.Rational) error {
	if m != nil && m.self == m && m.config.Rationals.Integers().MatchesCopyAllocationInvocation(&m.rationalCopy) {
		return ErrAlgebraicPending
	}
	l, e := m.location(a)
	if e != nil {
		return e
	}
	zero, e := m.config.Rationals.IsZero(q)
	if e != nil {
		return e
	}
	if zero {
		if e = m.Del(a); e != nil {
			return e
		}
		return m.assertCellResetZero(a)
	}
	if l != nil && l.kind == 1 {
		if e = m.Del(a); e != nil {
			return e
		}
		l = nil
	}
	if l != nil {
		// anum::to_basic at h:372 precedes the mutable value SET.
		if e = algebraicQueryTagAssertionSwitch(); e != nil {
			return e
		}
		r, e := m.load(l)
		if e != nil {
			return e
		}
		m.basicCellCopyRunning = true
		defer func() { m.basicCellCopyRunning = false }()
		e = m.config.Rationals.CopyWithAllocationReceipt(&r.value, q, &m.rationalCopy)
		return m.save(l, &r, e)
	}
	if m.pending.slot != 0 {
		return ErrAlgebraicPending
	}
	next, e := m.allocate(0)
	if next.slot != 0 && m.locations[next.slot-1].live {
		m.pending = next
	}
	if e != nil {
		return e
	}
	l = &m.locations[next.slot-1]
	r, e := m.load(l)
	if e != nil {
		return e
	}
	e = m.config.Rationals.Swap(&r.value, q)
	if e = m.save(l, &r, e); e != nil {
		return e
	}
	m.publish(a, next)
	return nil
}
func (m *AlgebraicRootCore) copyPolynomial(l *algebraicLocation, r *algebraicRecord, v UPolynomialView) error {
	return m.copyPolynomialFromOwner(l, r, m.config.Polynomials, v)
}

// copyPolynomialFromOwner follows copy_poly's ascending placement/copy order.
// The view and its numeral cells belong to source, while backing allocation and
// any accepted copy receipt belong to the receiving core.
func (m *AlgebraicRootCore) copyPolynomialFromOwner(l *algebraicLocation, r *algebraicRecord, source *UPolynomialManager, v UPolynomialView) error {
	if m.config.Rationals.Integers().MatchesCopyAllocationInvocation(&m.coefficientCopy) {
		return ErrAlgebraicPending
	}
	if e := m.assertCellCopyPolynomialEmpty(r); e != nil {
		return e
	}
	return m.copyPolynomialDataFromOwner(l, r, source, v)
}

// mk_algebraic_cell copies its inline placement loop without copy_poly's gates.
func (m *AlgebraicRootCore) copyPolynomialDataFromOwner(l *algebraicLocation, r *algebraicRecord, source *UPolynomialManager, v UPolynomialView) error {
	if m.config.Rationals.Integers().MatchesCopyAllocationInvocation(&m.coefficientCopy) {
		return ErrAlgebraicPending
	}
	r.size = v.count
	l.coefficientInitialized = 0
	l.coefficientCursor = 0
	if e := m.store(l, r); e != nil {
		return e
	}
	p := m.config.Polynomials
	var q UPolynomial
	var e error
	// The executing coefficient-copy caller owns this factory continuation; ordinary
	// public factory calls remain refused during its borrowed execution.
	if m.copyCall.NumericBound(true) || m.cellCoefficientFactoryContinuation() {
		q, e = p.newPolynomial()
	} else {
		q, e = p.NewPolynomial()
	}
	if e != nil {
		return e
	}
	r.poly = q
	if e = m.store(l, r); e != nil {
		return e
	}
	d, e := p.descriptor(q)
	if e != nil {
		return e
	}
	if e = p.reserve(&d.state, v.count); e != nil {
		return e
	}
	// Typed backing has zeroed physical spare capacity; only this explicit
	// prefix denotes source placement construction, then ascending numeral copy.
	for i := uint32(0); i < v.count; i++ {
		l.coefficientInitialized = i + 1
		a, e := source.read(v, i)
		if e != nil {
			return e
		}
		before := m.coefficientCopy
		e = m.config.Rationals.Integers().CopyFromOwnerWithReceipt(&p.values(&d.state)[i], source.config.Integer, a, &m.coefficientCopy)
		// This key changes only when this actual SET accepts an allocation.
		if m.copyCall.running && m.coefficientCopy != before {
			m.copyCall.coefficients = m.coefficientCopy
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func (m *AlgebraicRootCore) deletePolynomial(l *algebraicLocation, r *algebraicRecord) error {
	return m.destroyPolynomial(l, r, false)
}
func (m *AlgebraicRootCore) updateSign(l *algebraicLocation, r *algebraicRecord) error {
	v, e := m.sourcePolynomial(r)
	if e != nil {
		return e
	}
	sl, e := m.evalCellSign(v, r.interval.BorrowLower(), 0)
	if e != nil {
		return e
	}
	if e = m.assertCellLowerSign(sl); e != nil {
		return e
	}
	if e = m.assertCellUpperSign(v, r.interval.BorrowUpper(), sl); e != nil {
		return e
	}
	r.flags &^= 2
	if sl < 0 {
		r.flags |= 2
	}
	return m.store(l, r)
}
func (m *AlgebraicRootCore) normalizeCoefficients(l *algebraicLocation, r *algebraicRecord) error {
	if e := m.assertCellNormalizeDegree(r); e != nil {
		return e
	}
	v, e := m.sourcePolynomial(r)
	if e != nil {
		return e
	}
	if e = m.config.Polynomials.normalizeViewWithReceipt(v, &m.coefficientNormalization); e != nil {
		return e
	}
	if r.size == 0 {
		return arena.ErrBounds
	}
	lead, e := m.config.Polynomials.read(v, r.size-1)
	if e != nil {
		return e
	}
	negative, e := m.config.Rationals.Integers().IsNeg(lead)
	if e != nil {
		return e
	}
	if negative {
		if e = m.negateCellCoefficients(v); e != nil {
			return e
		}
		r.flags ^= 2
	}
	return m.store(l, r)
}
func (m *AlgebraicRootCore) SetRoot(a *Algebraic, p UPolynomialView, lower, upper *unumeral.BinaryRational, minimal bool) error {
	if e := m.setRoot(a, p, lower, upper, minimal, true); e != nil {
		return e
	}
	return m.traceSetRoot(a)
}

// Isolation calls mk_algebraic_cell directly. Its constructor ends before
// imp::set's later post-assignment sign assertion.
func (m *AlgebraicRootCore) mkAlgebraicCell(p UPolynomialView, lower, upper *unumeral.BinaryRational, minimal bool) (Algebraic, error) {
	var a Algebraic
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return a, e
	}
	if enabled && p.count <= 2 {
		if e = m.cellAssertionFailure(402, "sz > 2"); e != nil {
			return a, e
		}
	}
	e = m.setRoot(&a, p, lower, upper, minimal, false)
	return a, e
}
func (m *AlgebraicRootCore) setRoot(a *Algebraic, p UPolynomialView, lower, upper *unumeral.BinaryRational, minimal, verifyAssignment bool) (err error) {
	if err = m.check(); err != nil {
		return err
	}
	if m.cellBoundsCall.pending() {
		return ErrAlgebraicPending
	}
	if verifyAssignment {
		if err = m.assertCellDegree(p.count, 477, "sz > 1", 1); err != nil {
			return err
		}
	}
	l, err := m.locationCleanup(a)
	if err != nil {
		return err
	}
	if p.count < 2 {
		return arena.ErrBounds
	}
	if p.count == 2 {
		if err = m.beginQ(); err != nil {
			return err
		}
		defer func() { err = m.finishQ(err) }()
		x, e := m.config.Polynomials.read(p, 0)
		if e != nil {
			return e
		}
		y, e := m.config.Polynomials.read(p, 1)
		if e != nil {
			return e
		}
		if err = m.config.Rationals.SetParts(&m.localQ, x, y); err != nil {
			return err
		}
		if err = m.config.Rationals.Neg(&m.localQ); err != nil {
			return err
		}
		return m.SetTake(a, &m.localQ)
	}
	existing := l != nil && l.kind == 1
	if !existing {
		if err = m.Del(a); err != nil {
			return err
		}
		if m.pending.slot != 0 {
			return ErrAlgebraicPending
		}
		if verifyAssignment {
			if err = m.assertCellDegree(p.count, 402, "sz > 2", 2); err != nil {
				return err
			}
		}
		next, e := m.allocate(1)
		if next.slot != 0 && m.locations[next.slot-1].live {
			m.pending = next
		}
		if e != nil {
			return e
		}
		l = &m.locations[next.slot-1]
	}
	if existing {
		if err = m.assertCellDegree(p.count, 491, "sz > 2", 2); err != nil {
			return err
		}
		if err = algebraicQueryTagAssertionSwitch(); err != nil {
			return err
		}
	}
	r, err := m.load(l)
	if err != nil {
		return err
	}
	if existing {
		if err = m.deletePolynomial(l, &r); err != nil {
			return err
		}
	}
	err = m.copyCellPolynomial(l, &r, p, existing)
	if err != nil {
		return err
	}
	if err = m.setCellBounds(&r.interval, lower, upper); err != nil {
		return m.save(l, &r, err)
	}
	if err = m.store(l, &r); err != nil {
		return err
	}
	if existing {
		r.flags &= 2
		if minimal {
			r.flags |= 5
		}
		if err = m.store(l, &r); err != nil {
			return err
		}
	}
	if err = m.updateSign(l, &r); err != nil {
		return err
	}
	if !existing {
		r.flags &= 2
		if minimal {
			r.flags |= 1
		}
		if err = m.store(l, &r); err != nil {
			return err
		}
		if err = m.assertNewCellFlags(&r); err != nil {
			return err
		}
		if minimal {
			r.flags |= 4
			if err = m.store(l, &r); err != nil {
				return err
			}
		}
	}
	if err = m.normalizeCoefficients(l, &r); err != nil {
		return err
	}
	if !existing {
		m.publish(a, m.pending)
	}
	if verifyAssignment {
		if err = m.assertAssignedCellSign(&r); err != nil {
			return err
		}
	}
	return nil
}
func (m *AlgebraicRootCore) copyCellValue(dst, src *Algebraic) error {
	if m != nil && m.self == m && m.config.Rationals.Integers().MatchesCopyAllocationInvocation(&m.coefficientCopy) {
		return ErrAlgebraicPending
	}
	if e := m.check(); e != nil {
		return e
	}
	sl, e := m.locationCleanup(src)
	if e != nil {
		return e
	}
	dl, e := m.locationCleanup(dst)
	if e != nil {
		return e
	}
	// Go handle admission precedes the source cursor; self-assignment reads
	// neither cell payload nor an assertion switch.
	if dst == src {
		return nil
	}
	sourceBasic, targetBasic := sl == nil || sl.kind == 0, dl == nil || dl.kind == 0
	// The selected branch reaches one SASSERT (cpp:514/518/528/533).
	if _, _, e = algebraicAssertionSwitch(); e != nil {
		return e
	}
	if sourceBasic {
		if !targetBasic {
			if e = m.Del(dst); e != nil {
				return e
			}
		}
		q, e := m.copySourceBasicValue(m, sl)
		if e != nil {
			return e
		}
		return m.setRationalFromCell(dst, q)
	}
	if targetBasic {
		if e = m.Del(dst); e != nil {
			return e
		}
		next, e := m.allocate(1)
		if e != nil {
			if next.slot != 0 && m.locations[next.slot-1].live {
				m.pending = next
			}
			return e
		}
		m.publish(dst, next)
		dl = &m.locations[dst.slot-1]
	} else {
		if e = algebraicQueryTagAssertionSwitch(); e != nil {
			return e
		}
		old, e := m.load(dl)
		if e != nil {
			return e
		}
		if e = m.deletePolynomial(dl, &old); e != nil {
			return e
		}
		if e = algebraicQueryTagAssertionSwitch(); e != nil {
			return e
		}
		e = unumeral.SourceBinaryIntervalDel(m.config.Intervals, &old.interval)
		if e = m.save(dl, &old, e); e != nil {
			return e
		}
		// copy(a.to_algebraic(), b.to_algebraic()) reaches both accessors.
		if e = algebraicQueryTagAssertionSwitch(); e != nil {
			return e
		}
	}
	if e = algebraicQueryTagAssertionSwitch(); e != nil {
		return e
	}
	r, e := m.copySourcePolynomialFields(sl)
	if e != nil {
		return e
	}
	out, e := m.load(dl)
	if e != nil {
		return e
	}
	v, e := m.sourcePolynomial(&r)
	if e != nil {
		return e
	}
	if e = m.copyPolynomial(dl, &out, v); e != nil {
		return e
	}
	r.interval, e = m.copySourceIntervalFields(sl)
	if e != nil {
		return e
	}
	e = m.config.Intervals.WithAllocationContext(m.copyCall.context()).CopyFromOwnerTracked(&out.interval, m.config.Intervals, &r.interval, &m.copyCall.bounds)
	if e = m.save(dl, &out, e); e != nil {
		return e
	}
	out.flags, e = m.copySourceFlags(sl)
	if e != nil {
		return e
	}
	return m.store(dl, &out)
}

func (m *AlgebraicRootCore) CleanupRetained() error {
	if m != nil && m.self == m {
		for _, r := range []*upolyConvertEntryReceipt{&m.arithmetic.convertEntry, &m.inversion.convertEntry} {
			if m.config.Polynomials.convertEntryReceiptPending(r) && (m.config.Polynomials.convertEntry.NumericExecuting() || m.config.Polynomials.intervalBounds.NumericExecuting()) {
				return ErrAlgebraicPending
			}
		}
	}
	if m != nil && m.self == m {
		for _, r := range []*upolyRefinableInvocationReceipt{&m.isolateRefinable, &m.isolationConversion.refinable, &m.rootConstruction.conversion.refinable, &m.binary.refinable} {
			if m.config.Polynomials.refinableReceiptPending(r) && m.config.Polynomials.refinable.NumericExecuting() {
				return ErrAlgebraicPending
			}
		}
	}
	if m != nil && m.self == m && m.manager != nil && (m.manager.constructing || m.manager.finishing) {
		return ErrAlgebraicPending
	}
	return m.cleanupRetainedCore()
}
func (m *AlgebraicRootCore) cleanupRetainedCore() error {
	if m != nil && m.self == m && (m.copyCall.running || m.copyCall.cleaning || m.cellBoundsCall.running || m.cellBoundsCall.cleaning || m.cellCoefficientCopy.running || m.basicCellCopyRunning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.savedIntervalExecuting != nil || m.factorExecuting || m.cellEvaluationRunning || m.cellNeg.running) {
		return ErrAlgebraicPending
	}

	if m != nil && m.self == m && (m.foreignCopyBorrowers != 0 || m.foreignCopy.running || m.foreignCopy.cleaning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.foreignCopy.active {
		if e := m.cleanupForeignCopy(); e != nil {
			return e
		}
	}
	if m != nil && m.self == m && m.toOwnedRational.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.rootTrace.running || m.comparisonTraceRunning || m.predicateDiagnosticRunning || m.toRationalRunning || m.toOwnedRational.executing) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.cellAllocationRunning {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.rootConstruction.running || m.rootConstruction.finishing) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.isolationSigns.running || m.isolationSigns.finishing) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.multivariateIsolationRunning || m.univariateIsolationRunning || m.isolationNumeric.executing) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.polynomialEvaluationRunning != 0 {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.integerSet.running || m.integerAdd.running || m.integerAdd.cleaning || m.constQ.running) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.isolationConversion.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.filterRootsRunning {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.signEvaluationRunning {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.signResultant.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.signInterval.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.signSubstitution.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.rationalSign.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.rootDisplay.running || m.intervalDisplayRunning || m.decimalDisplayRunning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.selection.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.basicRootScope.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.division.running || m.division.cleaning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.binary.running || m.binary.cleaning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.resultant.executing {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.arithmetic.executing {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.bounds.executing || m.refinement.executing || m.refinementConversion.executing || m.refinementQ.executing || m.predicateQ.executing || m.integerSet.finishing || m.constQ.executing || (m.predicate.finishing || m.predicate.arithmeticExecuting)) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.inversion.executing {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.negationExecuting() {
		return ErrAlgebraicPending
	}
	if m == nil || m.self != m || m.closed {
		return ErrAlgebraicOwner
	}
	if e := m.cleanupRootTrace(); e != nil {
		return e
	}
	if e := m.cleanupCellEvaluations(); e != nil {
		return e
	}
	if e := m.cleanupCellNeg(); e != nil {
		return e
	}
	if e := m.cleanupCellBoundsCall(); e != nil {
		return e
	}
	if e := m.cleanupCopyCall(); e != nil {
		return e
	}
	if p := m.config.Polynomials; p != nil {
		if e := p.cleanupSquareFreeCall(&m.isolationSquareFree); e != nil {
			return e
		}
	}
	if e := m.cleanupIsolationNumeric(); e != nil {
		return e
	}
	if e := m.config.Polynomials.cleanupDefaultNormalizationReceipt(&m.coefficientNormalization); e != nil {
		return e
	}
	for _, r := range []*upolyRefinableInvocationReceipt{&m.isolateRefinable, &m.isolationConversion.refinable, &m.rootConstruction.conversion.refinable, &m.binary.refinable} {
		if e := m.config.Polynomials.cleanupRefinableReceipt(r); e != nil {
			return e
		}
	}
	for _, r := range []*upolyConvertEntryReceipt{&m.arithmetic.convertEntry, &m.inversion.convertEntry} {
		if e := m.config.Polynomials.cleanupConvertEntryReceipt(r); e != nil {
			return e
		}
	}
	if e := m.cleanupCellAllocation(); e != nil {
		return e
	}
	if e := m.representationNeg.roots.Cleanup(m.config.Rationals.Integers()); e != nil {
		return e
	}
	if p := m.config.Polynomials; p != nil {
		if e := p.modular.CleanupPNormalizeInvocation(&m.representationNormalize, true); e != nil {
			return e
		}
	}
	if e := m.cleanupToOwnedRational(); e != nil {
		return e
	}
	if e := m.config.Rationals.Integers().CleanupCopyAllocationInvocation(&m.toRationalCopy); e != nil {
		return e
	}
	if e := m.config.Rationals.Integers().CleanupCopyAllocationInvocation(&m.representationCopy); e != nil {
		return e
	}
	if e := m.config.Rationals.Integers().CleanupCopyAllocationInvocation(&m.rationalCopy); e != nil {
		return e
	}
	if e := m.cleanupConstQ(); e != nil {
		return e
	}
	if e := m.cleanupPredicateTrace(); e != nil {
		return e
	}
	if e := m.config.Rationals.Integers().CleanupCopyAllocationInvocation(&m.coefficientCopy); e != nil {
		return e
	}
	if e := m.cleanupResultantPolynomial(); e != nil {
		return e
	}
	if e := m.cleanupBinary(); e != nil {
		return e
	}
	if e := m.cleanupRationalArithmetic(); e != nil {
		return e
	}
	if e := m.cleanupInverse(); e != nil {
		return e
	}
	if e := m.cleanupBasicRoot(); e != nil {
		return e
	}
	if e := m.cleanupRootDisplay(); e != nil {
		return e
	}
	if e := m.cleanupRationalSign(); e != nil {
		return e
	}
	if e := m.cleanupSignResultant(); e != nil {
		return e
	}
	if e := m.cleanupSignInterval(); e != nil {
		return e
	}
	if e := m.cleanupSignSubstitution(); e != nil {
		return e
	}
	if e := m.cleanupIsolationConversion(); e != nil {
		return e
	}
	if e := m.cleanupDivision(); e != nil {
		return e
	}
	if e := m.cleanupIntervalDisplay(); e != nil {
		return e
	}
	if e := m.cleanupDecimalDisplay(); e != nil {
		return e
	}
	if e := m.cleanupBounds(); e != nil {
		return e
	}
	if e := m.refinement.cleanup(); e != nil {
		return e
	}
	if e := m.refinementConversion.cleanup(); e != nil {
		return e
	}
	if e := m.refinementQ.cleanup(m); e != nil {
		return e
	}
	if e := m.cleanupPredicateQ(); e != nil {
		return e
	}
	if e := m.CleanupNegation(); e != nil {
		return e
	}
	if m.localActive {
		if e := m.finishQ(nil); e != nil {
			return e
		}
	}
	if e := m.cleanupIntegerSet(); e != nil {
		return e
	}
	if e := m.cleanupIntegerAdd(true); e != nil {
		return e
	}
	if e := m.cleanupSelection(); e != nil {
		return e
	}
	if e := m.finishIsolationSigns(true); e != nil {
		return e
	}
	if m.pending.slot != 0 {
		l := &m.locations[m.pending.slot-1]
		if e := m.deleteLocation(&m.pending, l); e != nil {
			return e
		}
		m.constructionStage = 0
	}
	if e := m.cleanupPredicate(); e != nil {
		return e
	}
	if e := m.comparisonCleanup(); e != nil {
		return e
	}
	if e := m.cleanupRootConstruction(); e != nil {
		return e
	}
	return m.cleanupMultivariateIsolation()
}
func (m *AlgebraicRootCore) Close() error {
	if m != nil && m.self == m && m.manager != nil {
		return m.manager.Close()
	}
	return m.closeCoreMembers()
}
func (m *AlgebraicRootCore) checkDestructorExecution() error {
	if m != nil && m.self == m && (m.copyCall.running || m.copyCall.cleaning || m.cellBoundsCall.running || m.cellBoundsCall.cleaning || m.cellCoefficientCopy.running || m.basicCellCopyRunning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.savedIntervalExecuting != nil || m.factorExecuting || m.cellEvaluationRunning || m.cellNeg.running) {
		return ErrAlgebraicPending
	}

	if m != nil && m.self == m && (m.foreignCopyBorrowers != 0 || m.foreignCopy.running || m.foreignCopy.cleaning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.toOwnedRational.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.rootTrace.running || m.comparisonTraceRunning || m.predicateDiagnosticRunning || m.toRationalRunning || m.toOwnedRational.executing) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.cellAllocationRunning {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.rootConstruction.running || m.rootConstruction.finishing) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.isolationSigns.running || m.isolationSigns.finishing) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.multivariateIsolationRunning || m.univariateIsolationRunning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.polynomialEvaluationBorrowers != 0 || m.polynomialEvaluationRunning != 0) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.integerSet.running || m.integerAdd.running || m.integerAdd.cleaning || m.constQ.running) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.isolationConversion.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.filterRootsRunning {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.signEvaluationRunning {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.signResultant.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.signInterval.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.signSubstitution.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.rationalSign.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.rootDisplay.running || m.intervalDisplayRunning || m.decimalDisplayRunning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.selection.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.basicRootScope.running {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.division.running || m.division.cleaning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.binary.running || m.binary.cleaning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.resultant.executing {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.arithmetic.executing {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.bounds.executing || m.refinement.executing || m.refinementConversion.executing || m.refinementQ.executing || m.predicateQ.executing || m.integerSet.finishing || m.constQ.executing || (m.predicate.finishing || m.predicate.arithmeticExecuting)) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.inversion.executing || m.inversion.numericPending(m)) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.negationExecuting() {
		return ErrAlgebraicPending
	}
	if m == nil || m.self != m {
		return ErrAlgebraicOwner
	}
	if m.manager != nil && m.manager.sourceRunning {
		return ErrAlgebraicPending
	}
	return nil
}
func (m *AlgebraicRootCore) closeCoreMembers() error {
	if e := m.checkDestructorExecution(); e != nil {
		return e
	}
	if m.closed {
		return nil
	}
	// Only named private selection elements are excluded from caller ownership.
	var privateCallers, privateVectors uint32
	if m.rootSelection != nil {
		privateVectors = 1
		for i := uint32(0); i < uint32(len(m.rootSelection.values)); i++ {
			if m.rootSelection.values[i].slot != 0 {
				privateCallers++
			}
		}
	}
	if m.callers != privateCallers || m.vectors != privateVectors {
		return ErrAlgebraicLive
	}
	if m.rootConstruction.active {
		if e := m.cleanupRetainedCore(); e != nil {
			return e
		}
	}
	if e := m.CleanupNegation(); e != nil {
		return e
	}
	m.closing = true
	// The later lexical comparison sequence is destroyed before older owners.
	if e := m.comparisonCleanup(); e != nil {
		return e
	}
	if m.rootSelection != nil {
		if e := m.rootSelection.closeValues(true); e != nil {
			return e
		}
		m.rootSelection = nil
	}
	if e := m.cleanupRetainedCore(); e != nil {
		return e
	}
	// m_add_tmp is younger than the isolation vector members.
	if m.addTmp.slot != 0 {
		if e := m.config.Polynomials.DelPolynomial(&m.addTmp); e != nil {
			return e
		}
	}
	// Reverse declaration order of the selected actual source members.
	for _, v := range []**BinaryRationalVector{&m.uppers, &m.lowers, &m.roots} {
		if *v != nil {
			// These are this owner's actual private vectors. An earlier source
			// reset/drop may have stopped after consuming a physical NUM block;
			// owning teardown drains that accepted child before ending its owner.
			if e := (*v).CleanupManagerReset(); e != nil {
				return e
			}
			if e := (*v).CleanupDrop(); e != nil {
				return e
			}
			if e := (*v).Close(); e != nil {
				return e
			}
			*v = nil
		}
	}
	if m.factors != nil {
		if e := m.factors.Close(); e != nil {
			return e
		}
		m.factors = nil
	}
	if m.evalSignTmp.slot != 0 {
		if e := m.config.Polynomials.DelPolynomial(&m.evalSignTmp); e != nil {
			return e
		}
	}
	for _, p := range []*UPolynomial{&m.tmp3, &m.tmp2, &m.isolateTmp1} {
		if p.slot != 0 {
			if e := m.config.Polynomials.DelPolynomial(p); e != nil {
				return e
			}
		}
	}
	if m.manager == nil || m.manager.rationalTemporaryConstructed {
		if e := m.config.Rationals.Integers().Del(&m.isRationalTmp); e != nil {
			return e
		}
	}
	if m.manager != nil {
		return nil
	}
	if m.multivariate != nil {
		if e := m.multivariate.CloseChecked(); e != nil {
			return e
		}
		m.multivariate = nil
	}
	return m.releaseCoreStorage()
}
func (m *AlgebraicRootCore) releaseCoreStorage() error {
	b := m.config.Rationals.Budget()
	m.negationHistory = nil
	m.evalSignVariables.values = nil
	m.isolateRootsVariables.values = nil
	b.Release(uint64(len(m.multivariateIsolationFrames)) * uint64(reflect.TypeOf(algebraicMultivariateIsolationFrame{}).Size()))
	m.multivariateIsolationFrames = nil
	b.Release(m.header)
	m.locations = nil
	m.header = 0
	m.closed = true
	return nil
}

func numericRationalService_NewAlgebraicRootCore(s *unumeral.RationalService, c AlgebraicRootConfig) (out *AlgebraicRootCore, cause error) {
	cause = s.WithNumericBorrow(func(b *unumeral.RationalNumericBorrow) error {
		if e := b.Ready(); e != nil {
			return e
		}
		if c.Rationals != nil && c.Rationals != b.Rationals() {
			return ErrAlgebraicOwner
		}
		c.Rationals = b.Rationals()
		var e error
		out, e = NewAlgebraicRootCore(c)
		return e
	})
	return out, cause
}
func (m *AlgebraicRootCore) sourcePolynomial(r *algebraicRecord) (UPolynomialView, error) {
	v, e := m.config.Polynomials.wholeView(r.poly)
	if e != nil {
		return v, e
	}
	v.count = r.size
	return v, nil
}

// CleanupValue is the explicit owning abort of one caller-provided reachable
// value. It does not repair source metadata or discover roots from locations.
func (m *AlgebraicRootCore) CleanupValue(a *Algebraic) error {
	if m.negationProtectsCaller(a) {
		return ErrAlgebraicPending
	}
	l, e := m.location(a)
	if e != nil || l == nil {
		return e
	}
	return m.deleteLocation(a, l)
}
