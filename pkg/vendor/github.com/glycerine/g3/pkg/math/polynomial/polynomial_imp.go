package polynomial

// Source: src/math/polynomial/polynomial.cpp:1821-1850,2303-2370,2460-2461,
// 3185,3214,3875,4070-4071,6234,6344,6865.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// Core supplies the actual shared numeral and control owners. Allocator selects
// the second source constructor; a supplied Monomials selects the first. The
// generated global allocator binding is represented by the real borrowed
// Integer Runtime when neither explicit source pointer is supplied.
type MultivariatePolynomialManagerConfig struct {
	Core          MultivariatePolynomialConfig
	Allocator     *umemory.SmallAllocator
	Factorization UPolynomialFactorizationConfig
	Trace         *diagnostic.TraceTags
}

// The core is the actual cell/arithmetic implementation object. These are real
// remaining imp members, not separately initialized arithmetic shadows. Embedded
// owners are charged by this class header and have no second standalone charge.
type MultivariatePolynomialManager struct {
	flipLeadingSignCall                                        polynomialNegInvocationReceipt
	factorPrelude, factorEnd                                   polynomialFactorDiagnosticScope
	algebraicIsolationBorrowers                                uint32
	algebraicEvaluation                                        polynomialAlgebraicEvaluationScope
	factorInline                                               polynomialFactorConstantInvocationReceipt
	factorFrames                                               []*polynomialFactorCoreFrame
	factorCoreInvocation                                       uint64
	factorRoot                                                 polynomialFactorCoreInvocationReceipt
	factorUnivariateHistory                                    []polynomialFactorUnivariateHistory
	factorUnivariate                                           polynomialFactorUnivariateScope
	factorNumeralVector                                        polynomialFactorNumeralVectorScope
	factorNumeralVectorHistory                                 []polynomialFactorNumeralVectorHistory
	factorConfig                                               UPolynomialFactorizationConfig
	factorService                                              *UPolynomialFactorizationService
	pscChain2                                                  polynomialPSCChain2Scope
	pscChain1                                                  polynomialPSCChain1Scope
	discriminant                                               polynomialDiscriminantScope
	subresultant                                               polynomialSubresultantChainScope
	conversion                                                 polynomialGlobalConversionScope
	toPolynomial                                               polynomialToPolynomialScope
	toPolynomialHistory                                        []polynomialToPolynomialHistory
	gcdFrames                                                  []*polynomialGCDFunctionScope
	gcdDepth                                                   uint32
	normalization                                              polynomialNormalizeScope
	cra                                                        polynomialCRAScope
	iccpFrames                                                 []*polynomialICCPScope
	iccpDepth                                                  uint32
	iccpShortcutContent                                        polynomialContentInvocationReceipt
	iccpConstant                                               polynomialICCPConstantScope
	monic                                                      polynomialMonicScope
	self                                                       *MultivariatePolynomialManager
	core                                                       *MultivariatePolynomialCore
	upm                                                        *UPolynomialManager
	degree2pos                                                 []uint32
	lexPermutation                                             []uint32
	varDegrees                                                 []uint32
	lexBuckets                                                 [][]uint32
	iccp                                                       PolynomialSOMVector
	newton                                                     PolynomialNewtonVector
	skeletons                                                  PolynomialSkeletonVector
	originalBudget                                             *arena.Budget
	memoryTraceTags                                            *diagnostic.TraceTags
	header                                                     uint64
	constructed                                                uint8
	memberStage, bodyStage, cleanupStage                       uint8
	useSparseGCD, usePRSGCD                                    bool
	completed, sourceStarted, sourceFinalized, closing, closed bool
	memoryTraceLive                                            bool
	originalCause, sourceCause, cleanupCause, unwindCause      error
	squareRoot                                                 polynomialSquareRootScope
	derivative                                                 polynomialDerivativeScope
	evaluation                                                 polynomialEvaluationScope
	composition                                                polynomialCompositionScope
	craLex                                                     polynomialLexSortInvocationReceipt
	lexTraceDisplay                                            polynomialDisplayInvocationReceipt
	lexTraceGeneration                                         uint64
	lexPending                                                 bool
	lexGeneration                                              uint64
	lexCause                                                   error
	modularIndex                                               uint8
	mmConstructed, mmDropDone                                  bool
	destructorTraceRunning                                     bool
}

func NewMultivariatePolynomialManager(c MultivariatePolynomialManagerConfig) (*MultivariatePolynomialManager, error) {
	if c.Allocator != nil && c.Core.Monomials != nil {
		return nil, arena.ErrConfig
	}
	if c.Core.Integer == nil {
		return nil, arena.ErrConfig
	}
	if e := polynomialIntegerReady(c.Core.Integer); e != nil {
		return nil, e
	}
	if c.Allocator != nil {
		if c.Allocator.RuntimeOwner() != c.Core.Integer.RuntimeOwner() || c.Allocator.RuntimeOwner().Budget() != c.Core.Integer.Budget() {
			return nil, arena.ErrConfig
		}
	}
	h := uint64(reflect.TypeOf(MultivariatePolynomialManager{}).Size())
	if e := c.Core.Integer.Budget().Reserve(h); e != nil {
		return nil, e
	}
	m := &MultivariatePolynomialManager{header: h, originalBudget: c.Core.Integer.Budget(), factorConfig: c.Factorization, memoryTraceTags: c.Trace}
	m.self = m
	m.memoryTraceLive = true
	line := 6931
	if c.Allocator != nil {
		line = 6935
	}
	m.tracePolynomialImpAllocated(line)
	fail := func(e error) (*MultivariatePolynomialManager, error) {
		if m.core != nil && e == m.core {
			e = m.core.originalCause
		}
		m.originalCause = e
		if m.core != nil {
			m.core.originalCause = e
		}
		m.unwindCause = m.unwindMembers()
		m.cleanupCause = m.unwindCause
		return m, m
	}
	var e error
	m.core, e = preparePolynomialCore(c.Core, false)
	if e != nil {
		return fail(e)
	}
	m.constructed = 1
	m.upm, e = NewUPolynomialManager(UPolynomialConfig{Integer: c.Core.Integer, Checkpoint: c.Core.Checkpoint, Cooperation: c.Core.Cooperation})
	if e != nil {
		return fail(e)
	}
	m.constructed = 2
	if e = m.core.constructPolynomialCoreDefaults(); e != nil {
		return fail(e)
	}
	m.constructed = 3
	// Source declaration order: directory, ID, m2pos, then actual imp tmp1.

	m.constructed = 4
	for _, v := range [2]*PolynomialSOMVector{&m.iccp, &m.core.translateBuffers} {
		v.self = v
		v.storage = m.core
	}
	m.newton.self = &m.newton
	m.newton.storage = m.core
	m.skeletons.self = &m.skeletons
	m.skeletons.owner = m.core
	d := &m.core.substitution
	d.max.self = &d.max
	d.max.core = m.core
	d.max.runtime = m.core.memory
	d.max.constructed = true
	variableEmbedded(&d.max.maxDegree, m.core.memory, true)
	variableEmbedded(&d.max.variables, m.core.memory, true)
	d.positions.self = &d.positions
	d.positions.runtime = m.core.memory
	d.positions.constructed = true
	variableEmbedded(&d.positions.positions, m.core.memory, true)
	m.constructed = 5
	if m.core.monomials == nil {
		line := 2330
		if c.Allocator != nil {
			line = 2341
		}
		m.core.monomials, e = NewMonomialManager(MonomialManagerConfig{Budget: c.Core.Integer.Budget(), Memory: c.Core.Integer.RuntimeOwner(), Allocator: c.Allocator, Trace: c.Trace, polynomialImpAllocationLine: line})
		m.mmConstructed = m.core.monomials != nil
		if e != nil {
			return fail(e)
		}
	}
	m.core.queryPositions.self = &m.core.queryPositions
	m.core.queryPositions.manager = m.core.monomials
	if e = m.core.initializePolynomialCoreBody(); e != nil {
		return fail(e)
	}
	// Source init publishes flags only after zero/unit and their INC operations.
	m.useSparseGCD = true
	m.usePRSGCD = false
	m.completed = true
	return m, nil
}
func (m *MultivariatePolynomialManager) Error() string {
	if m.originalCause != nil {
		return m.originalCause.Error()
	}
	if m.cleanupCause != nil {
		return m.cleanupCause.Error()
	}
	if m.sourceCause != nil {
		return m.sourceCause.Error()
	}
	return ErrPolynomialPending.Error()
}
func (m *MultivariatePolynomialManager) Unwrap() error { return m.originalCause }
func (m *MultivariatePolynomialManager) identity() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	return nil
}
func (m *MultivariatePolynomialManager) ready() error {
	if e := m.identity(); e != nil {
		return e
	}
	if !m.completed || m.closing || m.sourceStarted {
		return arena.ErrClosed
	}
	return m.core.check()
}
