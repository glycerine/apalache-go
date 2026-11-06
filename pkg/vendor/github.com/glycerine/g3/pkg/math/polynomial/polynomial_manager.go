package polynomial

// Source: src/math/polynomial/polynomial.cpp:2308-2370,2481-2633.
// Selected multivariate cell/SOM prerequisite owner, not manager::imp.

import (
	"errors"
	"math"
	"reflect"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"

	"github.com/glycerine/g3/pkg/util/idgen"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

var ErrPolynomialOwner = unumeral.ErrNumericOwner
var ErrPolynomialAssertion = unumeral.ErrNumericAssertion
var ErrPolynomialPartial = errors.New("mpnum: unconstructed polynomial field")
var ErrPolynomialPending = unumeral.ErrNumericPending
var ErrPolynomialLive = errors.New("mpnum: live caller polynomials")
var polynomialOrigins atomic.Uint64

type MultivariatePolynomialConfig struct {
	Integer     *unumeral.IntegerManager
	Rationals   *unumeral.RationalManager
	Monomials   *MonomialManager
	Checkpoint  func() bool
	Cooperation func() error
}
type polynomialCreation struct {
	appendParent      *polynomialCheapAppendScope
	factorsParent     *PolynomialFactors
	conversionParent  *polynomialGlobalConversionScope
	compositionParent *polynomialCompositionScope
	pscOwner          uint64
	substitutionOwned bool
	translationOwned  bool
	generation        uint64
	derivativeParent  *polynomialDerivativeScope
	raw               umemory.ObjectRef
	value             Polynomial
	requested, id     uint32
	idConsumed        bool
	buffer            *polynomialSOM
}
type MultivariatePolynomialCore struct {
	pendingLexSort                                                                             *MultivariatePolynomialManager
	exactDivTrace                                                                              polynomialExactDivTraceScope
	quadraticTrace                                                                             *polynomialQuadraticTraceBorrow
	evaluationTrace                                                                            *polynomialEvaluationTraceScope
	dividesTrace                                                                               polynomialDividesTraceScope
	gcdTraceRunning                                                                            bool
	renameTraceRunning                                                                         bool
	algebraicTraceRunning                                                                      bool
	pendingSparse                                                                              *PolynomialSparseInterpolator
	sparseCaller                                                                               *PolynomialSparseInterpolator
	pendingCaches                                                                              *PolynomialCache
	cacheCaller                                                                                *PolynomialCache
	toPolynomialCaller                                                                         *MultivariatePolynomialManager
	quasiResultant                                                                             polynomialQuasiResultantScope
	quasiHistory                                                                               []polynomialQuasiResultantHistory
	variableNumeral                                                                            polynomialVariableNumeralScope
	variableNumeralHistory                                                                     []polynomialVariableNumeralHistory
	collectingVariables                                                                        bool
	somNormalizationOwner                                                                      polynomialSOMNormalizeInvocationReceipt
	normalizationOwner                                                                         polynomialNormalizeInvocationReceipt
	constantCoefficientInvocation                                                              polynomialConstantCoefficientScope
	nonzeroConstant                                                                            polynomialNonzeroConstantScope
	factorsOwner                                                                               *PolynomialFactors
	powerGeneration                                                                            uint64
	display                                                                                    polynomialDisplayState
	conversionOwner                                                                            *polynomialGlobalConversionScope
	dividesLocal                                                                               polynomialDividesScope
	contentLocal                                                                               polynomialContentScope
	pscReceipts                                                                                polynomialPSCReceipts
	generatedPSC                                                                               uint64
	pscClassic                                                                                 polynomialPSCClassicScope
	pscLocal                                                                                   polynomialPSCScope
	pscNext                                                                                    polynomialPSCNextScope
	lazardLocal                                                                                polynomialLazardScope
	variableLocal                                                                              polynomialVariableScope
	pscBuffer                                                                                  polynomialPSCRefBuffer
	translateBuffers                                                                           PolynomialSOMVector
	translation                                                                                polynomialTranslationScope
	substitution                                                                               polynomialSubstitutionState
	tmp1                                                                                       monomialTemp
	pseudoDivision                                                                             polynomialPseudoDivisionScope
	exactDivision                                                                              polynomialExactDivisionScope
	queryPositions                                                                             MonomialPositions
	powerLocal                                                                                 polynomialPowerScope
	newtonLocal                                                                                *PolynomialNewtonInterpolator
	generatedCalleeGeneration                                                                  uint64
	generatedCalleePending                                                                     bool
	directoryInitialized                                                                       bool
	linearGenerated                                                                            []bool
	linearGeneratedCleanup                                                                     uint32
	generatedMonomial                                                                          Monomial
	generatedActive                                                                            bool
	generatedBuffer                                                                            *polynomialSOM
	generatedIndex                                                                             uint32
	constructorUnwindStage                                                                     uint8
	constructorModularIndex                                                                    uint8
	constructorUnwinding, idsSourceDestroyed, directorySourceDestroyed, modularSourceDestroyed bool
	sourceStage                                                                                uint8
	sourceStarted                                                                              bool
	sourceCause                                                                                error
	sourceDecDone, sourceDeleteNeeded                                                          [2]bool
	polynomialNegScope
	factoryInvocation                                            polynomialFactoryScope
	arithmeticInvocation                                         uint64
	coefficientInvocation                                        polynomialCoefficientScope
	scalarDivision                                               polynomialScalarDivScope
	deletionHead                                                 *PolynomialDeletionHandler
	callbackDepth                                                uint32
	callbackValue                                                Polynomial
	linearConstant                                               unumeral.Int
	linearConstantActive                                         bool
	overloadResult                                               Polynomial
	constantScope                                                polynomialConstantScope
	singletonMonomial                                            Monomial
	singletonReference                                           bool
	singletonGeneration, constantGeneration                      uint64
	self                                                         *MultivariatePolynomialCore
	config                                                       MultivariatePolynomialConfig
	integer                                                      *unumeral.IntegerManager
	modular                                                      *unumeral.IntegerModularManager
	monomials                                                    *MonomialManager
	memory                                                       *umemory.Runtime
	ids                                                          idgen.Generator
	polynomials                                                  []Polynomial
	locations                                                    []polynomialLocation
	squareRootOwner                                              *polynomialSquareRootScope
	derivativeOwner                                              *polynomialDerivativeScope
	compositionOwner                                             *polynomialCompositionScope
	creationGeneration                                           uint64
	pending                                                      polynomialCreation
	som, som2, cheap, cheap2                                     polynomialSOM
	zero, unit                                                   Polynomial
	zeroNumeral                                                  unumeral.Int
	minusOne, constantCopy                                       unumeral.Int
	rationalTerms, linearTerms                                   polynomialNumerals
	linearMonomials                                              []Monomial
	origin, header                                               uint64
	live                                                         uint32
	closeStage                                                   uint8
	completed, closing, closed, monomialRetained, idsInitialized bool
	originalCause, cleanupCause                                  error
}

func NewMultivariatePolynomialCore(c MultivariatePolynomialConfig) (*MultivariatePolynomialCore, error) {
	m, e := preparePolynomialCore(c, true)
	if e != nil {
		return m, e
	}
	fail := func(e error) (*MultivariatePolynomialCore, error) {
		m.originalCause = e
		m.cleanupCause = m.unwindConstructorMembers()
		return m, m
	}
	if e = m.constructPolynomialCoreDefaults(); e != nil {
		return fail(e)
	}
	if e = m.initializePolynomialCoreBody(); e != nil {
		return fail(e)
	}
	return m, nil
}

// Preparation is a concrete member stage, not a completed polynomial::imp.
// The full source hierarchy must insert its distinct UP member before defaults.
func preparePolynomialCore(c MultivariatePolynomialConfig, requireMonomials bool) (*MultivariatePolynomialCore, error) {
	if c.Integer == nil || (requireMonomials && c.Monomials == nil) || c.Checkpoint == nil || c.Cooperation == nil {
		return nil, arena.ErrConfig
	}
	if e := polynomialIntegerReady(c.Integer); e != nil {
		return nil, e
	}
	if c.Monomials != nil {
		if e := c.Monomials.check(); e != nil {
			return nil, e
		}
		if c.Monomials.memory != c.Integer.RuntimeOwner() || c.Monomials.config.Budget != c.Integer.Budget() {
			return nil, arena.ErrConfig
		}
	}
	if c.Rationals != nil {
		if e := c.Rationals.CheckOwnerReady(); e != nil {
			return nil, e
		}
		if c.Rationals.Integers() != c.Integer {
			return nil, arena.ErrConfig
		}
	}
	h := uint64(reflect.TypeOf(MultivariatePolynomialCore{}).Size())
	if e := c.Integer.Budget().Reserve(h); e != nil {
		return nil, e
	}
	m := &MultivariatePolynomialCore{config: c, integer: c.Integer, monomials: c.Monomials, memory: c.Integer.RuntimeOwner(), header: h}
	m.self = m
	m.initSubstitutionMembers()
	m.initTranslationMember()
	for {
		old := polynomialOrigins.Load()
		if old == math.MaxUint64 {
			c.Integer.Budget().Release(h)
			return nil, arena.ErrSlotLimit
		}
		if polynomialOrigins.CompareAndSwap(old, old+1) {
			m.origin = old + 1
			break
		}
	}
	var e error
	m.modular, e = unumeral.NewIntegerModularManager(c.Integer)
	if e != nil {
		m.originalCause = e
		m.cleanupCause = m.unwindConstructorMembers()
		return m, m
	}
	return m, nil
}
func (m *MultivariatePolynomialCore) constructPolynomialCoreDefaults() error {
	m.directoryInitialized = true
	m.ids = idgen.New(0, func() bool { return m.memory.State().OutOfMemory })
	m.idsInitialized = true
	return nil
}
func (m *MultivariatePolynomialCore) initializePolynomialCoreBody() error {
	var e error
	if e = m.monomials.IncManagerRef(); e != nil {
		return e
	}
	m.monomialRetained = true

	m.som.init(m, false)
	m.som2.init(m, false)
	m.cheap.init(m, true)
	m.cheap2.init(m, true)
	m.zero, e = m.mkCell(0, nil, nil, nil, nil)
	if e != nil {
		return e
	}
	if e = m.modular.SetInt64(&m.zeroNumeral, 0); e != nil {
		return e
	}
	if e = m.incRefCore(m.zero); e != nil {
		return e
	}
	one := unumeral.NewInt(1)
	m.unit, e = m.mkConstCore(&one)
	if e != nil {
		return e
	}
	if e = m.incRefCore(m.unit); e != nil {
		return e
	}
	m.completed = true
	return nil
}
func (m *MultivariatePolynomialCore) Error() string {
	if m.originalCause != nil {
		return m.originalCause.Error()
	}
	if m.cleanupCause != nil {
		return m.cleanupCause.Error()
	}
	return ErrPolynomialPending.Error()
}
func (m *MultivariatePolynomialCore) Unwrap() error { return m.originalCause }
func (m *MultivariatePolynomialCore) check() error {
	if m.nonzeroConstantPending() && !m.nonzeroConstant.executing {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.collectingVariables || m.algebraicTraceRunning || m.renameTraceRunning || m.gcdTraceRunning || m.dividesTrace.running || m.quadraticTraceCallback() || m.evaluationTraceCallback()) {
		return ErrPolynomialPending
	}
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.closed || m.closing || m.sourceStarted || m.originalCause != nil {
		return arena.ErrClosed
	}
	if e := polynomialIntegerReady(m.integer); e != nil {
		return e
	}
	return m.monomials.check()
}
func (m *MultivariatePolynomialCore) incRefCore(p Polynomial) error {
	l, e := m.polynomialLocation(p)
	if e != nil {
		return e
	}
	if l.deletionStarted {
		return ErrPolynomialPending
	}
	r, e := m.readWord(p, 0)
	if e != nil {
		return e
	}
	return m.writeWord(p, 0, r+1)
}
func (m *MultivariatePolynomialCore) MkCoreTake(as []unumeral.Int, ms []Monomial) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if uint64(len(as)) > math.MaxUint32 {
		return Polynomial{}, arena.ErrBounds
	}
	return m.mkCell(uint32(len(as)), as, ms, nil, nil)
}
func (m *MultivariatePolynomialCore) mkConstCore(a *unumeral.Int) (Polynomial, error) {
	return m.mkConstCoreWithArithmeticReceipt(a, nil)
}
func (m *MultivariatePolynomialCore) mkConstCoreWithArithmeticReceipt(a *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) (Polynomial, error) {
	if m.singletonReference {
		return Polynomial{}, ErrPolynomialPending
	}
	u, e := m.monomials.Unit()
	if e != nil {
		return Polynomial{}, e
	}
	if e = m.monomials.IncRef(u); e != nil {
		return Polynomial{}, e
	}
	m.singletonMonomial = u
	m.singletonReference = true
	m.singletonGeneration++
	if receipt != nil && m.matchesArithmeticInvocation(receipt) && receipt.frame.executing && m.callbackDepth == 0 {
		receipt.frame.singleton = m.singletonMonomial
		receipt.frame.singletonGeneration = m.singletonGeneration
	}
	if f := m.factorsOwner; f != nil && f.executing && m.callbackDepth == 0 {
		f.singleton = m.singletonMonomial
		f.singletonGeneration = m.singletonGeneration
	}
	if s := m.compositionOwner; s != nil && s.executing {
		s.singleton = m.singletonMonomial
		s.singletonGeneration = m.singletonGeneration
	}
	if m.pscExecutingOwner() != 0 {
		m.pscReceipts.singleton = u
	}
	if receipt != nil {
		return m.mkCellWithReceipt(1, nil, []Monomial{u}, nil, a, &receipt.frame.factory)
	}
	return m.mkCell(1, nil, []Monomial{u}, nil, a)
}
func (m *MultivariatePolynomialCore) MkConstantTake(a *unumeral.Int) (Polynomial, error) {
	return m.mkConstantTakeWithArithmeticReceipt(a, nil)
}
func (m *MultivariatePolynomialCore) mkConstantTakeWithArithmeticReceipt(a *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if a == nil {
		return Polynomial{}, arena.ErrConfig
	}
	z, e := m.modular.IsZero(a)
	if e != nil {
		return Polynomial{}, e
	}
	if z {
		return m.zero, nil
	}
	one, e := m.modular.IsOne(a)
	if e != nil {
		return Polynomial{}, e
	}
	if one {
		return m.unit, nil
	}
	return m.mkConstCoreWithArithmeticReceipt(a, receipt)
}
func (m *MultivariatePolynomialCore) MkVariable(v, k uint32) (Polynomial, error) {
	return m.mkVariableWithReceipt(v, k, nil)
}
func (m *MultivariatePolynomialCore) mkVariableWithReceipt(v, k uint32, receipt *polynomialVariableInvocationReceipt) (result Polynomial, err error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if e := m.cellSourceAssert(2539, "is_valid(x)", func() (bool, error) { return m.monomials.IsValidVar(v) }); e != nil {
		return Polynomial{}, e
	}
	if m.singletonReference {
		return Polynomial{}, ErrPolynomialPending
	}
	as := [1]unumeral.Int{unumeral.NewInt(1)}
	var scope *polynomialVariableScope
	if receipt != nil {
		scope = &m.variableLocal
		if scope.active || scope.invocation == ^uint64(0) || m.callbackDepth != 0 {
			return Polynomial{}, ErrPolynomialPending
		}
		*scope = polynomialVariableScope{active: true, executing: true, invocation: scope.invocation + 1}
		*receipt = polynomialVariableInvocationReceipt{m, scope, scope.invocation}
		defer func() { scope.executing = false; scope.bodyCause = err }()
	}
	var outcome monomialFactoryOutcome
	b, e := m.generatedVariableWithOutcome(v, k, &outcome)
	if scope != nil {
		if outcome.started {
			scope.allocator = outcome.allocatorInvocation
		}
		if e != nil && outcome.started {
			scope.calleeGeneration = outcome.generation
		}
		if e == nil && outcome.created {
			scope.generated = b
		}
	}
	if e != nil {
		return Polynomial{}, e
	}
	if e = m.monomials.IncRef(b); e != nil {
		return Polynomial{}, e
	}
	m.singletonMonomial = b
	m.singletonReference = true
	m.singletonGeneration++
	if scope != nil {
		scope.generated = Monomial{}
		scope.singleton = b
		scope.singletonGeneration = m.singletonGeneration
	}
	if f := m.factorsOwner; f != nil && f.executing && m.callbackDepth == 0 {
		f.singleton = m.singletonMonomial
		f.singletonGeneration = m.singletonGeneration
	}
	if s := m.compositionOwner; s != nil && s.executing {
		s.singleton = m.singletonMonomial
		s.singletonGeneration = m.singletonGeneration
	}
	if m.pscExecutingOwner() != 0 {
		m.pscReceipts.singleton = b
	}
	m.transferGeneratedMonomial()
	if scope == nil {
		return m.mkCell(1, as[:], []Monomial{b}, nil, nil)
	}
	result, err = m.mkCellWithReceipts(1, as[:], []Monomial{b}, nil, nil, &scope.factory, nil)
	if err != nil {
		return result, err
	}
	if err = m.transferFactoryResult(&scope.factory, result); err != nil {
		return result, err
	}
	scope.active = false
	return result, nil
}
func (m *MultivariatePolynomialCore) MkTerms(as []unumeral.Int, ms []Monomial) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if e := m.som.reset(); e != nil {
		return Polynomial{}, e
	}
	for i := range as {
		if i >= len(ms) {
			return Polynomial{}, arena.ErrBounds
		}
		if e := m.som.add(&as[i], ms[i], false); e != nil {
			return Polynomial{}, e
		}
	}
	return m.som.mk(false)
}
func (m *MultivariatePolynomialCore) CloseChecked() error {
	if m.nonzeroConstantPending() {
		return ErrPolynomialPending
	}
	if m.cacheExecuting() || m.sparseExecuting(nil) {
		return ErrPolynomialPending
	}
	if e := m.pendingCacheCause(); e != nil {
		return e
	}
	if e := m.pendingSparseCause(); e != nil {
		return e
	}
	if m != nil && m.self == m && m.integer != nil && (m.factorConstantExecuting() || m.toPolynomialCallerExecuting() || numericIntegerManager_factorNumeralVectorCallerExecuting(m.integer) || numericIntegerManager_factorUnivariateCallerExecuting(m.integer) || numericIntegerManager_factorCoreCallerExecuting(m.integer)) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.quasiResultant.active || m.variableNumeral.active) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.collectingVariables || m.algebraicTraceRunning || m.renameTraceRunning || m.gcdTraceRunning || m.dividesTrace.running || m.quadraticTraceCallback() || m.evaluationTraceCallback()) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.normalizationPending() {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.variableLocal.active || m.pscClassic.active || m.pscClassic.executing) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.negativeActive && m.negativeExecuting {
		return ErrPolynomialPending
	}
	if m.arithmeticExecuting() {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.coefficientInvocation.active || m.constantCoefficientInvocation.active) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.displayPending() {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.dividesLocal.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.contentLocal.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.pscPending() {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.translation.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.substitution.scope.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.pseudoDivision.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.exactDivision.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.powerLocal.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.newtonLocal != nil {
		return ErrPolynomialPending
	}
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if m.originalCause != nil && m.constructorUnwinding {
		if e := m.unwindConstructorMembers(); e != nil {
			m.cleanupCause = e
			return e
		}
		m.cleanupCause = nil
	}
	if !m.closing {
		// Named cached cells can also carry real caller-owned references. The
		// owning adapter refuses before source actions, without a directory
		// sweep or changing the literal source destructor's later assertion.
		for i, p := range [2]Polynomial{m.zero, m.unit} {
			if p != (Polynomial{}) && int(m.sourceStage) <= i {
				l, e := m.polynomialLocation(p)
				if e != nil {
					return e
				}
				if l.deleteStage == 0 {
					r, e := m.readWord(p, 0)
					if e != nil {
						return e
					}
					if r > 1 {
						return ErrPolynomialLive
					}
				}
			}
		}
		private, e := m.factoryCallerGuard()
		if e != nil {
			return e
		}
		if m.zero != (Polynomial{}) && m.sourceStage == 0 {
			private++
		}
		if m.unit != (Polynomial{}) && m.sourceStage <= 1 {
			private++
		}
		if m.constantScope.active && m.constantScope.result != (Polynomial{}) && m.constantScope.result != m.zero && m.constantScope.result != m.unit {
			private++
		}
		if m.overloadResult != (Polynomial{}) && m.overloadResult != m.zero && m.overloadResult != m.unit {
			private++
		}
		if m.negativeResult != (Polynomial{}) && m.negativeResult != m.zero && m.negativeResult != m.unit {
			private++
		}
		if m.live > private {
			return ErrPolynomialLive
		}
		m.closing = true
	}
	if e := m.closeDisplayMetadata(); e != nil {
		return e
	}
	if e := m.cleanupCreation(); e != nil {
		m.cleanupCause = e
		return e
	}
	if m.overloadResult != (Polynomial{}) {
		if e := m.deleteCore(m.overloadResult); e != nil {
			return e
		}
		m.overloadResult = Polynomial{}
	}
	if m.linearConstantActive {
		if e := m.deleteNumeral(&m.linearConstant); e != nil {
			return e
		}
		m.linearConstantActive = false
	}
	if e := m.cleanupConstantScope(); e != nil {
		m.cleanupCause = e
		return e
	}
	if e := m.cleanupNegPhase(true); e != nil {
		m.cleanupCause = e
		return e
	}
	if e := m.cleanupScalarDivPhase(true); e != nil {
		m.cleanupCause = e
		return e
	}
	privates := [2]*Polynomial{&m.zero, &m.unit}
	for m.closeStage < 2 {
		p := privates[m.closeStage]
		if m.sourceStage > m.closeStage {
			*p = Polynomial{}
			m.closeStage++
			continue
		}
		if *p != (Polynomial{}) {
			l, e := m.polynomialLocation(*p)
			if e != nil {
				return e
			}
			if l.callbackCause != nil {
				// The source DEC already reached zero before its handler
				// refused. Explicit owning retry aborts this retained action;
				// neither replay the callback nor repeat the source DEC.
				if e = m.CleanupValue(p); e != nil {
					m.cleanupCause = e
					return e
				}
			} else if l.deleteStage == 0 {
				if e = m.decRefCore(*p); e != nil {
					m.cleanupCause = e
					return e
				}
			} else {
				if e = m.deleteCore(*p); e != nil {
					return e
				}
			}
			*p = Polynomial{}
		}
		m.closeStage++
	}
	buffers := [4]*polynomialSOM{&m.som, &m.som2, &m.cheap, &m.cheap2}
	for m.closeStage < 6 {
		if e := buffers[m.closeStage-2].close(); e != nil {
			return e
		}
		m.closeStage++
	}
	if m.closeStage == 6 {
		// The real modular child may have refused its cold header before
		// construction. No numeral-manager action exists in that prefix.
		if m.modular != nil {
			if e := m.deleteNumeral(&m.zeroNumeral); e != nil {
				return e
			}
		}
		m.closeStage++
	}
	for _, v := range [2]*polynomialNumerals{&m.rationalTerms, &m.linearTerms} {
		if e := v.close(m); e != nil {
			return e
		}
	}
	m.polynomials, m.linearMonomials = nil, nil
	if m.idsInitialized || m.idsSourceDestroyed {
		m.ids.Cleanup(0)
		m.idsInitialized = false
	}
	if e := m.translateBuffers.CloseChecked(); e != nil {
		return e
	}
	// Deletion traces can create formatter frames while cached polynomials and
	// SOM buffers are destroyed, after the initial cleanup of older frames.
	if e := m.closeDisplayMetadata(); e != nil {
		return e
	}
	if m.modular != nil && !m.modular.Closed() {
		for _, a := range [3]*unumeral.Int{&m.minusOne, &m.negativeCoefficient, &m.constantCopy} {
			if e := m.deleteNumeral(a); e != nil {
				return e
			}
		}
		if m.modularSourceDestroyed {
			// Actual source fields were destroyed during failed-constructor
			// unwind. This refunds only the retained Go adapter header.
			if e := m.modular.FinishSourceLifetime(); e != nil {
				return e
			}
		} else if e := m.modular.Close(); e != nil {
			return e
		}
	}
	if e := m.closeSubstitutionMembers(); e != nil {
		return e
	}
	if m.queryPositions.self != nil {
		if e := m.queryPositions.CloseChecked(); e != nil {
			return e
		}
	}
	if m.monomialRetained {
		before := m.monomials.managerRefs
		e := m.monomials.DecManagerRef()
		if m.monomials.managerRefs != before {
			m.monomialRetained = false
		}
		if e != nil {
			return e
		}
	}
	m.linearGenerated = nil
	m.locations = nil
	m.quasiHistory = nil
	m.variableNumeralHistory = nil
	m.integer.Budget().Release(m.header)
	m.header = 0
	m.closed = true
	return nil
}

func polynomialIntegerReady(m *unumeral.IntegerManager) error { return m.SourceReady() }

func (m *MultivariatePolynomialCore) SetZPUint64(p uint64) error {
	if e := m.check(); e != nil {
		return e
	}
	return m.modular.SetZPUint64(p)
}
func (m *MultivariatePolynomialCore) SetZ() error {
	if e := m.check(); e != nil {
		return e
	}
	return m.modular.SetZ()
}
