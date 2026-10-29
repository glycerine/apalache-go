package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1324-1329,432-513.
// Source: src/math/polynomial/upolynomial.cpp:1298-1321.
// Source: src/math/polynomial/upolynomial.cpp:1734-1764.
// Source: src/math/polynomial/upolynomial.h; src/math/polynomial/upolynomial.cpp.

import (
	"errors"
	"math"
	"reflect"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

var ErrUPolynomialOwner = unumeral.ErrNumericCallerOwner
var ErrUPolynomialLive = errors.New("mpnum: live polynomial owners")
var ErrUPolynomialCanceled = errors.New("mpnum: polynomial resource canceled")
var ErrUPolynomialAssertion = unumeral.ErrNumericCallerAssertion
var upolyOrigins atomic.Uint64

type UPolynomialConfig struct {
	Integer     *unumeral.IntegerManager
	Checkpoint  func() bool
	Cooperation func() error
}

// The two callbacks borrow actual source resource/cooperation owners. They
// are required, even though operations without a checkpoint do not call them.
// Cold ownership and every actual typed backing are charged to Integer's budget.
type UPolynomialManager struct {
	sourceFactors                           *UPolynomialFactors
	intervalBounds                          upolyIntervalBoundsScope
	convertEntry                            upolyConvertEntryScope
	refinable                               upolyRefinableScope
	squareFreeCall                          upolySquareFreeCallScope
	defaultNormalization                    upolyDefaultNormalizeScope
	factorTrace                             *upolyFactorTraceBorrow
	factorsDisplay, pendingFactorsDisplay   *upolyFactorsDisplayScope
	sexprArithmetic, pendingSexprArithmetic *sexprPolynomialArithmeticScope
	rationalInput                           upolyRationalInputScope
	algebraicCopyBorrowers                  uint32
	sturmIsolation                          upolySturmIsolationScope
	sturmPending                            *upolySequenceSturmScope
	rootID                                  upolyRootIDScope
	rationalContext                         UPolynomialRationalContext
	oneHalfRoot                             upolyOneHalfRootScope
	oneHalfRootHistory                      *upolyOneHalfRootHistory
	oneHalfRootHistoryBytes                 uint64
	knuth                                   upolyKnuthScope
	knuthHistory                            *upolyKnuthHistory
	knuthHistoryBytes                       uint64
	rootUpperBound                          upolyRootUpperBoundScope
	removeOneHalfRoot                       upolyRemoveOneHalfRootScope
	rootUpperBoundHistory                   *upolyRootUpperBoundHistory
	removeOneHalfRootHistory                *upolyRemoveOneHalfRootHistory
	removeOneHalfRootFirstHistory           upolyRemoveOneHalfRootHistory
	rootUpperBoundHistoryBytes              uint64
	removeOneHalfRootHistoryBytes           uint64
	integerEvaluation                       upolyIntegerEvaluationScope
	integerEvaluationHistory                *upolyIntegerEvaluationHistory
	integerEvaluationHistoryBytes           uint64
	rationalEvaluation                      upolyRationalEvaluationScope
	rationalEvaluationHistory               *upolyRationalEvaluationHistory
	rationalEvaluationHistoryBytes          uint64
	binaryEvaluation                        upolyBinaryEvaluationScope
	binaryEvaluationHistory                 *upolyBinaryEvaluationHistory
	binaryEvaluationHistoryBytes            uint64
	algebraicNegCaller                      *AlgebraicRootCore
	display                                 upolyDisplayScope
	displaySequence                         *UPolynomialSequence
	currentDisplay, displayFrames           *upolyDisplayScope
	displayNonce                            uint64

	factorFrontendCaller         *upolyFactorFrontendScope
	factorFrontendConstantCaller *UPolynomialFactorizationService
	primePreparationCaller       *upolyFactorPrimePreparationScope
	preparationCaller            *upolyFactorPreparationScope
	henselPairCaller             *upolyHenselPairScope
	henselCallerGuard            *upolyHenselCallerScope
	henselCallerInvocation       uint64
	callbackDepth                uint32
	mignotteCaller               *upolyMignotteScope
	trialCaller                  *upolyRecombinationTrialScope
	recombinationCaller          *upolyRecombinationStepScope
	primeTrialCaller             *upolyPrimeTrialScope
	squareFreeCaller             *upolyFactorSquareFreeScope
	self                         *UPolynomialManager
	pendingBerlekamp             *upolyBerlekampMatrix
	factorField                  *upolyFactorFieldScope
	factorFieldFrames            [2]upolyFactorFieldScope
	craTrace                     upolyCRATrace
	craValues                    [12]unumeral.Int
	pendingCRATrace              *upolyCRATrace
	modGCDScalars                upolyModGCDTraceScalars
	pendingFactorScalarTrace     *upolyFactorFrontendScope
	pendingFactorCoreTrace       *upolyDisplayInvocationReceipt
	euclidTrace                  upolyDisplayInvocationReceipt
	pendingEuclidTrace           *upolyDisplayInvocationReceipt
	modGCDTrace                  upolyDisplayInvocationReceipt
	pendingModGCDTrace           *upolyDisplayInvocationReceipt
	exactDivTrace                upolyDisplayInvocationReceipt
	pendingExactDivTrace         *upolyDisplayInvocationReceipt
	origin                       uint64
	config                       UPolynomialConfig
	modular                      *unumeral.IntegerModularManager
	// Actual source members: 16 core and five derived numeral vectors.
	basic, div1, div2, exactDiv, gcd1, gcd2, cra   upolyVectorState
	mgcd                                           [6]upolyVectorState
	sqf1, sqf2, pw                                 upolyVectorState
	db, dbab1, dbab2, tr, push                     upolyVectorState
	pages                                          []*upolyDescriptorPage
	store                                          *UPolynomialCoefficientStore
	bindings                                       []upolyStoreBinding
	live                                           uint32
	liveSequences                                  uint32
	liveFactors                                    uint32
	header                                         uint64
	closed, closing                                bool
	sourceFinalizing, sourceFinalized, sourceGuard bool
	sourceStage                                    uint8
	sourceIndex                                    uint32
	sourceCause                                    error
}

func NewUPolynomialManager(c UPolynomialConfig) (*UPolynomialManager, error) {
	return newUPolynomialManager(c, nil)
}
func newUPolynomialManager(c UPolynomialConfig, store *UPolynomialCoefficientStore) (*UPolynomialManager, error) {
	if c.Integer == nil || c.Checkpoint == nil || c.Cooperation == nil {
		return nil, arena.ErrConfig
	}
	if c.Integer.Closed() {
		return nil, arena.ErrClosed
	}
	n := uint64(reflect.TypeOf(UPolynomialManager{}).Size())
	if e := c.Integer.Budget().Reserve(n); e != nil {
		return nil, e
	}
	z, e := unumeral.NewIntegerModularManager(c.Integer)
	if e != nil {
		c.Integer.Budget().Release(n)
		return nil, e
	}
	var id uint64
	for {
		old := upolyOrigins.Load()
		if old == math.MaxUint64 {
			z.Close()
			c.Integer.Budget().Release(n)
			return nil, arena.ErrSlotLimit
		}
		if upolyOrigins.CompareAndSwap(old, old+1) {
			id = old + 1
			break
		}
	}
	if store == nil {
		store, e = newUPolynomialCoefficientStore(c.Integer, id)
	} else {
		e = store.retain()
	}
	if e != nil {
		z.Close()
		c.Integer.Budget().Release(n)
		return nil, e
	}
	m := &UPolynomialManager{origin: id, config: c, modular: z, header: n, store: store}
	m.self = m
	z.CallerExecuting = func() bool { return numericIntegerModularManager_upolynomialCallerExecuting(z) }
	return m, nil
}
func (m *UPolynomialManager) check() error {
	if m != nil && m.self == m && m.sourceFactors != nil && m.sourceFactors.NumericExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.squareFreeCall.active && (!m.squareFreeCall.executing || m.squareFreeCall.callback()) && !m.squareFreeCall.sourceReadAllowed() {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.factorField != nil && !m.factorField.executing && !m.factorField.sourceExecuting && !m.factorField.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.intervalBounds.active && (!m.intervalBounds.executing || m.intervalBounds.borrow.callback()) && !m.intervalBounds.sourceExecuting {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.convertEntry.active && (!m.convertEntry.executing || m.convertEntry.borrow.callback()) {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.refinable.active && (!m.refinable.executing || m.refinable.borrow.callback()) && !m.refinable.sourceReadAllowed() {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.defaultNormalization.active && (!m.defaultNormalization.executing || m.defaultNormalization.borrow.callback()) && !m.defaultNormalization.sourceReadAllowed() {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.pendingFactorScalarTrace != nil && m.pendingFactorScalarTrace.scalarTracePending() && !m.pendingFactorScalarTrace.scalarTraceSourceReadAllowed() {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.pendingFactorCoreTrace != nil && m.displayReceiptPending(m.pendingFactorCoreTrace) && !m.pendingFactorCoreTrace.frame.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.pendingEuclidTrace != nil && m.displayReceiptPending(m.pendingEuclidTrace) && !m.pendingEuclidTrace.frame.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.modGCDScalars.executing && (m.callbackDepth != 0 || m.config.Integer.RuntimeOwner().FailureTransportActive() || m.config.Integer.BorrowTraceTags().OutputCallbackActive()) {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.modGCDScalars.ownRemaining != 0 && !m.modGCDScalars.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.pendingModGCDTrace != nil && m.displayReceiptPending(m.pendingModGCDTrace) && !m.pendingModGCDTrace.frame.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.pendingCRATrace != nil && m.pendingCRATrace.pending() && !m.pendingCRATrace.executing && !m.pendingCRATrace.sourceReadAllowed() {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.pendingExactDivTrace != nil && m.pendingExactDivTrace.owner.displayReceiptPending(m.pendingExactDivTrace) && !m.pendingExactDivTrace.frame.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.pendingBerlekamp != nil {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.factorTrace != nil && m.factorTrace.executing && m.factorTrace.callback() {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && (m.pendingFactorsDisplay != nil && m.pendingFactorsDisplay.pending() || m.factorsDisplay != nil && m.factorsDisplay.NumericExecuting() && m.factorsDisplay.callback()) {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && (m.pendingSexprArithmetic != nil || m.sexprArithmeticExecuting() && (m.callbackDepth != 0 || m.config.Integer.RuntimeOwner().FailureTransportActive() || m.config.Integer.BorrowTraceTags().OutputCallbackActive())) {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.rationalInput.running {
		return ErrUPolynomialCleanupPending
	}
	if m == nil || m.self != m || m.config.Integer == nil {
		return ErrUPolynomialOwner
	}
	if m.closed || m.closing || m.sourceFinalizing || m.config.Integer.Closed() {
		return arena.ErrClosed
	}
	return nil
}
func (m *UPolynomialManager) IntegerManager() *unumeral.IntegerManager {
	if m == nil || m.self != m {
		return nil
	}
	return m.config.Integer
}
func (m *UPolynomialManager) checkpoint() error {
	m.callbackDepth++
	defer func() { m.callbackDepth-- }()
	if !m.config.Checkpoint() {
		return upolynomialCancellationError
	}
	return m.config.Cooperation()
}
func (m *UPolynomialManager) descriptor(p UPolynomial) (*upolyDescriptor, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	if p.origin != m.origin || p.slot == 0 {
		return nil, ErrUPolynomialOwner
	}
	i := p.slot - 1
	if uint64(i/32) >= uint64(len(m.pages)) {
		return nil, ErrUPolynomialOwner
	}
	d := &m.pages[i/32].rows[i%32]
	if !d.live || d.generation != p.generation {
		return nil, ErrUPolynomialOwner
	}
	return d, nil
}
func (m *UPolynomialManager) NewPolynomial() (UPolynomial, error) {
	if m != nil && m.factorTrace != nil && m.factorTrace.executing {
		return UPolynomial{}, ErrUPolynomialCleanupPending
	}
	if m != nil && m.factorsDisplay != nil && m.factorsDisplay.NumericExecuting() {
		return UPolynomial{}, ErrUPolynomialCleanupPending
	}
	if m.sexprArithmeticExecuting() {
		return UPolynomial{}, ErrUPolynomialCleanupPending
	}
	return m.newPolynomial()
}
func (m *UPolynomialManager) newPolynomial() (UPolynomial, error) {
	if e := m.check(); e != nil {
		return UPolynomial{}, e
	}
	for i, page := range m.pages {
		for j := range page.rows {
			d := &page.rows[j]
			if !d.live && d.generation != math.MaxUint32 {
				if d.generation == 0 {
					d.generation = 1
				}
				d.live = true
				m.live++
				return UPolynomial{m.origin, uint32(i*32 + j + 1), d.generation}, nil
			}
		}
	}
	if uint64(len(m.pages)+1)*32 > math.MaxUint32 {
		return UPolynomial{}, arena.ErrSlotLimit
	}
	page := &upolyDescriptorPage{}
	m.pages = append(m.pages, page)
	page.rows[0] = upolyDescriptor{generation: 1, live: true}
	m.live++
	return UPolynomial{m.origin, uint32((len(m.pages)-1)*32 + 1), 1}, nil
}
func (m *UPolynomialManager) DelPolynomial(p *UPolynomial) error {
	if m != nil && m.factorTrace != nil && m.factorTrace.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.factorsDisplay != nil && m.factorsDisplay.NumericExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if m.sexprArithmeticExecuting() {
		return ErrUPolynomialCleanupPending
	}
	return m.delPolynomial(p)
}
func (m *UPolynomialManager) delPolynomial(p *UPolynomial) error {
	if p == nil {
		return ErrUPolynomialOwner
	}
	d, e := m.descriptor(*p)
	if e != nil {
		return e
	}
	qiScope := &m.sturmIsolation
	if qiScope.owner == m && qiScope.active && qiScope.target.origin == m.origin && qiScope.target.backing == d.state.backing && qiScope.target.generation == d.state.generation {
		return ErrUPolynomialCleanupPending
	}
	qrid := &m.rootID
	if qrid.owner == m && qrid.active && qrid.target.origin == m.origin && qrid.target.backing == d.state.backing && qrid.target.generation == d.state.generation && (qrid.executing || qrid.finishing || qrid.sourceStop != nil || qrid.childStop != nil) {
		return ErrUPolynomialCleanupPending
	}
	qh := &m.oneHalfRoot
	if qh.owner == m && qh.active && !qh.falseAcknowledged && qh.target.origin == m.origin && qh.target.backing == d.state.backing && qh.target.generation == d.state.generation && (qh.executing || qh.finishing || qh.sourceCause != nil) {
		return ErrUPolynomialCleanupPending
	}
	qk := &m.knuth
	if qk.owner == m && qk.active && !qk.falseAcknowledged && qk.target.origin == m.origin && qk.target.backing == d.state.backing && qk.target.generation == d.state.generation && (qk.executing || qk.finishing || qk.sourceCause != nil) {
		return ErrUPolynomialCleanupPending
	}
	qx := &m.removeOneHalfRoot
	if qx.owner == m && qx.active && qx.output == *p && (qx.executing || qx.finishing) {
		return ErrUPolynomialCleanupPending
	}
	if qx.owner == m && qx.active && !qx.falseAcknowledged && qx.target.origin == m.origin && qx.target.backing == d.state.backing && qx.target.generation == d.state.generation && (qx.executing || qx.finishing || qx.sourceCause != nil) {
		return ErrUPolynomialCleanupPending
	}
	qu := &m.rootUpperBound
	if qu.owner == m && qu.active && !qu.falseAcknowledged && qu.target.origin == m.origin && qu.target.backing == d.state.backing && qu.target.generation == d.state.generation && (qu.executing || qu.finishing || qu.sourceCause != nil) {
		return ErrUPolynomialCleanupPending
	}

	qi := &m.integerEvaluation
	if qi.owner == m && qi.active && !qi.falseAcknowledged && qi.target.origin == m.origin && qi.target.backing == d.state.backing && qi.target.generation == d.state.generation && (qi.executing || qi.finishing || qi.sourceCause != nil) {
		return ErrUPolynomialCleanupPending
	}

	qr := &m.rationalEvaluation
	if qr.owner == m && qr.active && !qr.falseAcknowledged && qr.target.origin == m.origin && qr.target.backing == d.state.backing && qr.target.generation == d.state.generation && (qr.executing || qr.finishing || qr.sourceCause != nil) {
		return ErrUPolynomialCleanupPending
	}

	q := &m.binaryEvaluation
	if q.owner == m && q.active && !q.falseAcknowledged && q.target.origin == m.origin && q.target.backing == d.state.backing && q.target.generation == d.state.generation && (q.executing || q.finishing || q.sourceCause != nil) {
		return ErrUPolynomialCleanupPending
	}
	if e = m.reset(&d.state); e != nil {
		return e
	}
	m.freeBacking(&d.state)
	d.live = false
	if d.generation != math.MaxUint32 {
		d.generation++
	}
	m.live--
	*p = UPolynomial{}
	return nil
}
func (m *UPolynomialManager) Close() error { return m.closeWithHensel(nil) }
func (m *UPolynomialManager) closeWithHensel(r *upolyHenselPairInvocationReceipt) error {
	return m.closeWithHenselOwners(r, nil)
}
func (m *UPolynomialManager) closeWithHenselOwners(r *upolyHenselPairInvocationReceipt, c *upolyHenselCallerScope) error {
	return m.closeWithCallerOwners(r, c, nil)
}
func (m *UPolynomialManager) closeWithCallerOwners(r *upolyHenselPairInvocationReceipt, c *upolyHenselCallerScope, p *upolyFactorPrimePreparationInvocationReceipt) error {
	if m != nil && m.self == m && m.sourceFactors != nil && m.sourceFactors.NumericExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m {
		if e := m.cleanupSquareFreeCall(nil); e != nil {
			return e
		}
	}
	if m != nil && m.factorField != nil {
		if e := m.CleanupFactorFieldFactorization(); e != nil {
			return e
		}
	}
	if m != nil && m.intervalBounds.active {
		if e := m.CleanupIntervalConversion(); e != nil {
			return e
		}
	}
	if m != nil && m.convertEntry.active {
		if e := m.CleanupConversionEntryTrace(); e != nil {
			return e
		}
	}
	if m != nil && m.refinable.active {
		if e := m.CleanupRefinable(); e != nil {
			return e
		}
	}
	if m != nil && m.defaultNormalization.active {
		if e := m.CleanupNormalization(); e != nil {
			return e
		}
	}
	if m != nil && m.modGCDScalars.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.modGCDScalars.ownRemaining != 0 {
		if e := m.CleanupModularGCDTrace(); e != nil {
			return e
		}
	}
	if m != nil && m.craTrace.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.pendingCRATrace != nil && m.pendingCRATrace.pending() && m.pendingCRATrace != &m.craTrace {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.pendingCRATrace == &m.craTrace && m.craTrace.pending() {
		if e := m.CleanupCRATrace(); e != nil {
			return e
		}
	}
	if m != nil && m.factorTrace != nil && m.factorTrace.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && (m.factorsDisplay != nil && m.factorsDisplay.NumericExecuting() || m.pendingFactorsDisplay != nil && m.pendingFactorsDisplay.pending()) {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && (m.sexprArithmeticExecuting() || m.pendingSexprArithmetic != nil) {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.rationalInput.running {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.algebraicCopyBorrowers != 0 {
		return ErrUPolynomialCleanupPending
	}
	if m.sturmIsolationExecuting() || m.sequenceSturmExecuting() || m.rootIDExecuting() || m.removeOneHalfRootExecuting() || m.oneHalfRootExecuting() || m.knuthExecuting() || m.rootUpperBoundExecuting() || m.integerEvaluationExecuting() || m.rationalEvaluationExecuting() || m.binaryEvaluationExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.algebraicNegCaller != nil && m.algebraicNegCaller.negationExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if m == nil || m.self != m || m.config.Integer == nil {
		return ErrUPolynomialOwner
	}
	if m.mignotteCallPending() && !m.henselDestructorAuthorized(r, true) && !m.henselCallerDestructorAuthorized(c, true) && !m.primePreparationDestructorAuthorized(p, true) {
		return ErrUPolynomialCleanupPending
	}
	if m.closed || m.config.Integer.Closed() {
		return arena.ErrClosed
	}
	if m.live != 0 || m.liveSequences != 0 || m.liveFactors != 0 {
		return ErrUPolynomialLive
	}
	if m != nil && m.self == m && m.pendingBerlekamp != nil {
		if e := m.cleanupBerlekamp(); e != nil {
			return e
		}
	}
	if e := m.CleanupRationalInput(); e != nil {
		return e
	}
	if e := m.CleanupSturmIsolation(); e != nil {
		return e
	}
	if e := m.CleanupRootID(); e != nil {
		return e
	}
	if e := m.CleanupOneHalfRoot(); e != nil {
		return e
	}
	if e := m.CleanupKnuthBounds(); e != nil {
		return e
	}
	if e := m.CleanupRemoveOneHalfRoot(); e != nil {
		return e
	}
	if e := m.CleanupRootUpperBound(); e != nil {
		return e
	}
	if e := m.CleanupIntegerEvaluation(); e != nil {
		return e
	}
	if e := m.CleanupRationalEvaluation(); e != nil {
		return e
	}
	if e := m.CleanupBinaryEvaluation(); e != nil {
		return e
	}
	if e := m.CleanupDisplay(); e != nil {
		return e
	}
	m.closing = true
	if m.sourceFinalizing && !m.sourceFinalized && m.sourceCause == nil {
		if e := m.sourceFinalizeWithCallerOwners(r, c, p); e != nil {
			return e
		}
	}
	// Owning cleanup acknowledges actual retained release cursors separately;
	// stopped literal source cause/stage/history are never repaired or advanced.
	// Derived destructor reset order, followed by implicit reverse members.
	derived := [5]*upolyVectorState{&m.db, &m.dbab1, &m.dbab2, &m.tr, &m.push}
	for _, s := range derived {
		if e := m.reset(s); e != nil {
			return e
		}
	}
	for i := len(derived) - 1; i >= 0; i-- {
		m.freeBacking(derived[i])
	}
	core := [16]*upolyVectorState{&m.basic, &m.div1, &m.div2, &m.exactDiv, &m.gcd1, &m.gcd2, &m.cra, &m.mgcd[0], &m.mgcd[1], &m.mgcd[2], &m.mgcd[3], &m.mgcd[4], &m.mgcd[5], &m.sqf1, &m.sqf2, &m.pw}
	for _, s := range core {
		if e := m.reset(s); e != nil {
			return e
		}
	}
	for i := len(core) - 1; i >= 0; i-- {
		m.freeBacking(core[i])
	}
	if e := m.modular.Close(); e != nil {
		return e
	}
	b := m.config.Integer.Budget()
	m.store.release()
	for _, binding := range m.bindings {
		binding.store.release()
	}
	m.bindings = nil
	for q := m.displayFrames; q != nil; {
		next := q.next
		b.Release(q.header)
		q.header = 0
		q.next = nil
		q = next
	}
	m.displayFrames = nil
	b.Release(m.oneHalfRootHistoryBytes)
	m.oneHalfRootHistoryBytes = 0
	m.oneHalfRootHistory = nil
	b.Release(m.knuthHistoryBytes)
	m.knuthHistoryBytes = 0
	m.knuthHistory = nil
	b.Release(m.removeOneHalfRootHistoryBytes)
	m.removeOneHalfRootHistoryBytes = 0
	m.removeOneHalfRootHistory = nil
	b.Release(m.rootUpperBoundHistoryBytes)
	m.rootUpperBoundHistoryBytes = 0
	m.rootUpperBoundHistory = nil
	b.Release(m.integerEvaluationHistoryBytes)
	m.integerEvaluationHistoryBytes = 0
	m.integerEvaluationHistory = nil
	b.Release(m.rationalEvaluationHistoryBytes)
	m.rationalEvaluationHistoryBytes = 0
	m.rationalEvaluationHistory = nil
	b.Release(m.binaryEvaluationHistoryBytes)
	m.binaryEvaluationHistoryBytes = 0
	m.binaryEvaluationHistory = nil
	b.Release(m.header)
	m.pages = nil

	m.closed = true
	return nil
}
