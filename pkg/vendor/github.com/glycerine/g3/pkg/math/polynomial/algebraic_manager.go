package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:64-159,2772-2787.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// Borrowed configuration services outlive the manager. A nil Allocator selects
// the source's private "algebraic" allocator; supplied allocators stay borrowed.
type AlgebraicManagerConfig struct {
	Rationals *unumeral.RationalService
	// Numerals borrows the concrete source qm(), including the private
	// unsynchronized owner in arith_decl_plugin::algebraic_numbers_wrapper.
	// Nil selects Rationals.Manager; Rationals also supplies diagnostics.
	Numerals      *unumeral.RationalManager
	Limit         *control.Reslimit
	Cooperation   *cooperate.Cooperation
	Params        *uparams.ParamsRef
	Globals       *uparams.GlobalParamsService
	Allocator     *umemory.SmallAllocator
	Factorization UPolynomialFactorizationConfig
}

// AlgebraicManager supplies the owning source manager around the numerical
// core. Arithmetic wrappers and promoted methods use these same eager members.
// A failed constructor can return a retained owner with an error; Close owns
// its unfinished cleanup and must precede release of the borrowed services.
type AlgebraicManager struct {
	*AlgebraicRootCore
	unwindStarted, unwindFinalized                                                      bool
	unwindStage                                                                         uint8
	unwindIndex                                                                         uint32
	unwindCause                                                                         error
	sourceStarted, sourceFinalized, sourceRunning                                       bool
	sourceStage                                                                         uint8
	sourceIndex                                                                         uint32
	sourceBindings                                                                      [2]unumeral.NumericCallerBinding
	sourceBound                                                                         uint8
	self                                                                                *AlgebraicManager
	config                                                                              AlgebraicManagerConfig
	params                                                                              uparams.ModuleParamsOwner
	allocator                                                                           *umemory.SmallAllocator
	binary                                                                              *unumeral.BinaryRationalManager
	intervals                                                                           *unumeral.BinaryIntervalManager
	polynomial                                                                          *MultivariatePolynomialManager
	univariate                                                                          *UPolynomialManager
	factorService                                                                       *UPolynomialFactorizationService
	header                                                                              uint64
	originalCause, sourceCause, cleanupCause                                            error
	ownAllocator, polynomialConstructed, coreMembersReady, rationalTemporaryConstructed bool
	privateFinalized, constructing, finishing, completed, closed                        bool
}

func NewAlgebraicManager(c AlgebraicManagerConfig) (*AlgebraicManager, error) {
	if c.Rationals == nil || c.Limit == nil || c.Cooperation == nil || c.Params == nil || c.Globals == nil || c.Globals.Context == nil {
		return nil, arena.ErrConfig
	}
	if c.Limit.Closed() {
		return nil, arena.ErrClosed
	}
	if c.Numerals == nil {
		c.Numerals = c.Rationals.Manager
	}
	if c.Numerals == nil {
		return nil, arena.ErrConfig
	}
	if e := c.Numerals.CheckOwnerReady(); e != nil {
		return nil, e
	}
	if _, e := c.Cooperation.Enabled(); e != nil {
		return nil, e
	}
	b := c.Numerals.Budget()
	n := uint64(reflect.TypeOf(AlgebraicManager{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	m := &AlgebraicManager{config: c, header: n, constructing: true}
	m.self = m
	m.params = uparams.ModuleParamsOwner{Budget: c.Globals.Config.Budget}
	m.params.Self = &m.params
	fail := func(e error) (*AlgebraicManager, error) {
		m.originalCause = e
		m.constructing = false
		m.sourceCause = m.unwindConstructorSource()
		return m, m
	}
	var e error
	m.allocator = c.Allocator
	if m.allocator == nil {
		m.ownAllocator = true // source assignment precedes allocator construction
		m.allocator, e = umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: c.Numerals.Integers().RuntimeOwner(), Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: polynomialSourceDebug}})
		if e != nil {
			return fail(e)
		}
	} else if m.allocator.RuntimeOwner() != c.Numerals.Integers().RuntimeOwner() {
		return fail(arena.ErrConfig)
	}
	m.AlgebraicRootCore, e = allocateAlgebraicRootCore(AlgebraicRootConfig{Rationals: c.Numerals, Allocator: m.allocator, Checkpoint: c.Limit.Inc, Cooperation: func() error { return c.Cooperation.Cooperate("algebraic") }})
	if e != nil {
		return fail(e)
	}
	core := m.AlgebraicRootCore
	core.manager = m
	m.binary, e = unumeral.NewBinaryRationalManager(c.Numerals.Integers(), unumeral.BinaryRationalConfig{Diagnostics: c.Rationals})
	if e != nil {
		return fail(e)
	}
	core.config.Binary = m.binary
	m.intervals, e = unumeral.NewBinaryIntervalManager(m.binary)
	if e != nil {
		return fail(e)
	}
	core.config.Intervals = m.intervals
	m.polynomial, e = NewMultivariatePolynomialManager(MultivariatePolynomialManagerConfig{Core: MultivariatePolynomialConfig{Integer: c.Numerals.Integers(), Rationals: c.Numerals, Checkpoint: c.Limit.Inc, Cooperation: func() error { return c.Cooperation.Cooperate("polynomial") }}, Allocator: m.allocator, Factorization: c.Factorization})
	core.multivariate = m.polynomial
	if e != nil {
		return fail(e)
	}
	m.polynomialConstructed = true
	upConfig := UPolynomialConfig{Integer: c.Numerals.Integers(), Checkpoint: c.Limit.Inc, Cooperation: func() error { return c.Cooperation.Cooperate("upolynomial") }}
	if c.Numerals == c.Rationals.Manager {
		m.univariate, e = NewUPolynomialManagerFromRationalService(c.Rationals, upConfig)
	} else {
		m.univariate, e = NewUPolynomialManager(upConfig)
	}
	if e != nil {
		return fail(e)
	}
	core.config.Polynomials = m.univariate
	m.factorService, e = NewUPolynomialFactorizationService(m.univariate, c.Factorization)
	if e != nil {
		return fail(e)
	}
	core.config.Factors = m.factorService
	m.coreMembersReady = true
	core.zero = unumeral.NewRational()
	core.isRationalTmp = unumeral.NewInt(0)
	m.rationalTemporaryConstructed = true
	for _, p := range []*UPolynomial{&core.isolateTmp1, &core.tmp2, &core.tmp3, &core.evalSignTmp} {
		*p, e = m.univariate.NewPolynomial()
		if e != nil {
			return fail(e)
		}
	}
	core.factors, e = m.univariate.NewFactors()
	if e != nil {
		return fail(e)
	}
	for _, v := range []**BinaryRationalVector{&core.roots, &core.lowers, &core.uppers} {
		*v, e = numericBinaryRationalManager_NewVector(m.binary)
		if e != nil {
			return fail(e)
		}
	}
	core.addTmp, e = m.univariate.NewPolynomial()
	if e != nil {
		return fail(e)
	}
	if e = m.params.Bind(c.Globals.Context, c.Globals); e != nil {
		return fail(e)
	}
	if e = m.readConstructorParams(); e != nil {
		return fail(e)
	}
	core.compareCheap, core.compareSturm, core.compareRefine, core.comparePoly = 0, 0, 0, 0
	for core.resultantVariableCount < 2 {
		x, e := m.polynomial.core.monomials.MkVar()
		if e != nil {
			return fail(e)
		}
		core.resultantVariables[core.resultantVariableCount] = x
		core.resultantVariableCount++
	}
	m.config.Params = nil // the generated helper borrowed this only during construction
	m.constructing = false
	m.completed = true
	return m, nil
}
func (m *AlgebraicManager) Error() string {
	if m.originalCause != nil {
		return m.originalCause.Error()
	}
	if m.cleanupCause != nil {
		return m.cleanupCause.Error()
	}
	return ErrAlgebraicPending.Error()
}
func (m *AlgebraicManager) Unwrap() error { return m.originalCause }
func (m *AlgebraicManager) ready() error {
	if m == nil || m.self != m {
		return ErrAlgebraicOwner
	}
	if m.sourceStarted || !m.completed || m.closed || m.finishing || m.constructing {
		return arena.ErrClosed
	}
	return m.AlgebraicRootCore.check()
}

// UpdateParams is deliberately empty in the public C++ implementation. Only
// construction invokes imp::updt_params and reads generated module getters.
func (m *AlgebraicManager) UpdateParams(*uparams.ParamsRef) error {
	if m == nil || m.self != m {
		return ErrAlgebraicOwner
	}
	if m.sourceStarted || m.closed || !m.completed {
		return arena.ErrClosed
	}
	return nil
}
func (m *AlgebraicManager) ResetStatistics() error {
	if e := m.ready(); e != nil {
		return e
	}
	c := m.AlgebraicRootCore
	c.compareCheap, c.compareSturm, c.compareRefine, c.comparePoly = 0, 0, 0, 0
	return nil
}
func (m *AlgebraicManager) CollectStatistics(s *statistics.Statistics) error {
	if e := m.ready(); e != nil {
		return e
	}
	if s == nil {
		return arena.ErrConfig
	}
	c := m.AlgebraicRootCore
	s.UpdateUint("algebraic compare cheap", c.compareCheap)
	s.UpdateUint("algebraic compare sturm", c.compareSturm)
	s.UpdateUint("algebraic compare refine", c.compareRefine)
	s.UpdateUint("algebraic compare poly", c.comparePoly)
	return nil
}
func (m *AlgebraicManager) Close() error {
	if m == nil || m.self != m {
		return ErrAlgebraicOwner
	}
	if m.closed {
		return nil
	}
	if m.AlgebraicRootCore != nil && (m.foreignCopyBorrowers != 0 || m.foreignCopy.running || m.foreignCopy.cleaning) {
		return ErrAlgebraicPending
	}
	if m.sourceRunning || m.constructing || m.finishing || m.AlgebraicRootCore != nil && (m.univariateIsolationRunning || m.factorExecuting || m.cellEvaluationRunning || m.cellNeg.running || m.copyCall.running || m.copyCall.cleaning || m.cellBoundsCall.running || m.cellBoundsCall.cleaning || m.cellCoefficientCopy.running || m.basicCellCopyRunning || m.savedIntervalExecuting != nil || m.AlgebraicRootCore.binary.running || m.AlgebraicRootCore.binary.cleaning || m.division.cleaning || m.integerAdd.cleaning || m.rootTrace.running || m.comparisonTraceRunning || m.predicateDiagnosticRunning || m.toRationalRunning || m.toOwnedRational.running || m.toOwnedRational.executing || m.refinement.executing || m.refinementConversion.executing || m.refinementQ.executing || m.predicateQ.executing || m.integerSet.running || m.integerSet.finishing || m.constQ.running || m.constQ.executing || (m.predicate.finishing || m.predicate.arithmeticExecuting)) {
		return ErrAlgebraicPending
	}
	e := m.closeOwners()
	if e != nil {
		m.cleanupCause = e
	}
	return e
}
func (m *AlgebraicManager) closeOwners() error {
	if m.finishing {
		return ErrAlgebraicPending
	}
	m.finishing = true
	defer func() { m.finishing = false }()
	if m.params.Helpers != nil {
		if e := m.params.CloseHelpers(); e != nil {
			return e
		}
	}
	if m.coreMembersReady && !m.privateFinalized {
		var e error
		if m.sourceStarted || m.unwindStarted {
			e = m.cleanupSourceMembers()
		} else {
			e = m.AlgebraicRootCore.closeCoreMembers()
		}
		if e != nil {
			return e
		}
		m.privateFinalized = true
	}
	if m.factorService != nil {
		if e := m.factorService.Close(); e != nil {
			return e
		}
		m.factorService = nil
	}
	if m.univariate != nil {
		if e := m.univariate.Close(); e != nil {
			return e
		}
		m.univariate = nil
	}
	if m.polynomial != nil {
		if e := m.polynomial.CloseChecked(); e != nil {
			return e
		}
		m.polynomial = nil
		if m.AlgebraicRootCore != nil {
			m.AlgebraicRootCore.multivariate = nil
		}
	}
	if m.intervals != nil {
		if e := m.intervals.CloseChecked(); e != nil {
			return e
		}
		m.intervals = nil
	}
	if m.binary != nil {
		if e := m.binary.CloseChecked(); e != nil {
			return e
		}
		m.binary = nil
	}
	if m.AlgebraicRootCore != nil {
		if e := m.AlgebraicRootCore.releaseCoreStorage(); e != nil {
			return e
		}
	}
	if m.ownAllocator && m.allocator != nil {
		if e := m.allocator.Close(); e != nil {
			return e
		}
		m.allocator = nil
	}
	m.config.Numerals.Budget().Release(m.header)
	m.header = 0
	m.closed = true
	m.completed = false
	return nil
}

// RationalManager and BinaryRationalManager return the actual source qm()/bqm()
// owners, without starting a second numerical scope during an arithmetic call.
func (m *AlgebraicManager) RationalManager() (*unumeral.RationalManager, error) {
	if m == nil || m.self != m {
		return nil, ErrAlgebraicOwner
	}
	if m.sourceStarted || !m.completed || m.closed || m.finishing {
		return nil, arena.ErrClosed
	}
	return m.config.Numerals, nil
}
func (m *AlgebraicManager) BinaryRationalManager() (*unumeral.BinaryRationalManager, error) {
	if m == nil || m.self != m {
		return nil, ErrAlgebraicOwner
	}
	if m.sourceStarted || !m.completed || m.closed || m.finishing {
		return nil, arena.ErrClosed
	}
	return m.binary, nil
}
