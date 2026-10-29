package smtcore

// Source: src/smt/smt_kernel.h and src/smt/smt_kernel.cpp retained assertion,
// scope, setup, check, and context-access facade.

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/ast/normal_forms"
	"github.com/glycerine/g3/pkg/ast/pattern"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/ast/rewriter/bit_blaster"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type SMTKernelAssertion struct {
	Formula ast.ASTHandle
	Proof   ast.ASTHandle
}

type SMTKernel struct {
	univariateNRA      *smtNRAWorkspace
	nraInputs          []ast.ASTRef
	nraScopes          []int
	nraInputsValid     bool
	queueWarnings      *diagnostic.WarningService // Borrowed factory diagnostics; nil uses process warnings.
	caseSplitQueue     *SMTActivityCaseSplitQueue
	booleanSetupSeeded bool
	smtParamsBorrowed  bool

	manager          *ast.ASTManager
	checker          *SMTCheckerContext
	relevancy        *SMTRelevancyPropagator
	repairRelevancy  bool
	internalizer     *SMTInternalizerState
	assertions       []SMTKernelAssertion
	assertedHead     int
	scopes           []int
	unsatCore        []ast.ASTHandle
	unsatProof       ast.ASTRef
	assumptions      map[SMTLiteral]ast.ASTHandle
	lastFailure      SMTFailure
	unknown          string
	labels           []usymbol.Symbol
	assignments      []ast.ASTHandle
	relevantLiterals []ast.ASTHandle
	relevantLabeled  []ast.ASTHandle
	guessed          []ast.ASTHandle
	callback         solver.ProgressCallback
	modelConfig      model.ModelEvaluatorConfig
	generated        *model.Model
	preprocessor     *rewriter.TheoryRewriter
	patternInference *pattern.InferenceRewriter
	bvSharing        *rewriter.MaximizeBVSharing
	assertedMacros   *macros.Manager
	definedNames     *normal_forms.DefinedNames
	quantifierCosts  *SMTQICostModel
	modelCooperation *cooperate.Cooperation
	paramContext     *uparams.ParamsContext
	paramGlobals     *uparams.GlobalParamsService
	smtParams        *SMTParams
	smtState         SMTParamsState
	arithState       SMTArithState
	bvState          SMTBVState
	preprocessState  SMTPreprocessorState
	produceModel     bool
	preprocess       bool
	timerClock       func() (uint64, error)
	closed           bool

	// Source m_has_quantifiers is sticky, including across scope pops.
	assertedHasQuantifiers   bool
	assertedSubstitution     ast.ASTExprSubstitution
	assertedScopedSubst      ast.ASTScopedExprSubstitution
	assertedInconsistent     bool
	assertedScopeConflicts   []bool
	assertedParams           uparams.ParamsRef
	assertedParamsReady      bool
	assertedElimAnd          bool
	assertedPhaseReady       bool
	preprocessingSetupDone   bool
	setupLogic               string
	sequenceSetupPlaceholder SMTTheoryInternalizer
	copiedArithmeticPlugin   bool
	pendingSetupLRA          *SMTLRATheory
	copiedArithmeticOwner    *SMTArithParams
	copiedArithmeticConfig   SMTArithState
	copiedBVOwner            *SMTBVParams
	copiedBitBlasterOwner    *bit_blaster.SMTBitBlasterParams
	copiedArrayPlugin        bool
	copiedArrayOwner         *SMTArrayParams
}

func (k *SMTKernel) readPreprocessorState(context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef) (state SMTPreprocessorState, result error) {
	value, err := NewSMTPreprocessorParams(context, params, globals, k.manager.Budget())
	if err != nil {
		if value != nil {
			result = errors.Join(err, value.Close())
			return state, result
		}
		return state, err
	}
	defer func() { result = errors.Join(result, value.Close()) }()
	return value.State()
}

func NewSMTKernel(manager *ast.ASTManager, quantifiers *SMTQuantifierRegistry) (*SMTKernel, error) {
	checker, err := NewSMTCheckerContext(manager)
	if err != nil {
		return nil, err
	}
	relevancy, err := NewSMTRelevancyPropagator(checker, true)
	if err != nil {
		return nil, err
	}
	internalizer, err := NewSMTInternalizerState(manager, checker, quantifiers)
	if err != nil {
		return nil, err
	}
	proofs, err := manager.ProofsEnabled()
	if err != nil {
		return nil, errors.Join(err, internalizer.Close())
	}
	if !proofs {
		if err = internalizer.SetBinaryClauseOptimization(true); err != nil {
			return nil, errors.Join(err, internalizer.Close())
		}
	}
	if err = internalizer.SetRelevancy(relevancy); err != nil {
		return nil, errors.Join(err, internalizer.Close())
	}
	proof, err := manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, errors.Join(err, internalizer.Close())
	}
	macroManager, err := macros.NewManager(manager)
	if err != nil {
		return nil, errors.Join(err, manager.DelRef(&proof), internalizer.Close())
	}
	names, err := normal_forms.NewDefinedNames(manager)
	if err != nil {
		return nil, errors.Join(err, macroManager.Close(), manager.DelRef(&proof), internalizer.Close())
	}
	sharing, err := rewriter.NewMaximizeBVSharing(manager)
	if err != nil {
		if sharing != nil {
			err = errors.Join(err, sharing.Close())
		}
		return nil, errors.Join(err, names.Close(), macroManager.Close(), manager.DelRef(&proof), internalizer.Close())
	}
	kernel := &SMTKernel{
		nraInputsValid: true,
		manager:        manager, checker: checker, relevancy: relevancy, internalizer: internalizer, unsatProof: proof, assertedMacros: macroManager, definedNames: names, bvSharing: sharing,
		smtState: SMTParamsState{
			MaxConflicts: ^uint32(0), InvDecay: 1.052, RandomInitialActivity: 1, RandomVarFreq: 0.01, CaseSplitStrategy: 1, RestartStrategy: 1, RestartInitial: 100, RestartFactor: 1.1,
			RestartAdaptive: true, AgilityFactor: 0.9999, RestartAgilityThreshold: 0.18, SimplifyClauses: true,
		},
	}
	internalizer.assertedHasQuantifiers = &kernel.assertedHasQuantifiers
	internalizer.assertedInconsistent = &kernel.assertedInconsistent
	internalizer.assertedFormulas = &kernel.assertions
	kernel.assertedSubstitution, err = manager.NewExprSubstitution()
	if err != nil {
		return nil, errors.Join(err, kernel.Close())
	}
	kernel.assertedScopedSubst, err = manager.NewScopedExprSubstitution(kernel.assertedSubstitution)
	if err != nil {
		return nil, errors.Join(err, kernel.Close())
	}
	return kernel, nil
}

// ConfigurePreprocessing installs the source theory-rewrite pass used before
// asserted formulas enter the retained formula stack. It must be configured
// before the first assertion is stored.
func (k *SMTKernel) ConfigurePreprocessing(context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef) error {
	return k.configurePreprocessing(context, globals, params, nil)
}

func (k *SMTKernel) configurePreprocessing(context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef, sourceParams *SMTParams) error {
	return k.configurePreprocessingParams(context, globals, params, sourceParams, false)
}

// Nested source kernels borrow context::m_fparams. Solver copies retain the
// existing owned-clone path; the String expression solver uses the borrowed
// path and must close before the parent parameter owner.
func (k *SMTKernel) configurePreprocessingParams(context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef, sourceParams *SMTParams, borrowSource bool) error {
	if err := k.open(); err != nil {
		return err
	}
	if borrowSource && sourceParams == nil {
		return uparams.ErrParamsOwner
	}
	if len(k.assertions) != 0 || k.assertedHead != 0 || len(k.scopes) != 0 || k.preprocessor != nil || k.smtParams != nil || k.assertedParamsReady {
		return ast.ErrASTDomain
	}
	if context == nil || globals == nil || context.SymbolsField != k.manager.Symbols || globals.Context != context {
		return uparams.ErrParamsOwner
	}
	k.paramContext = context
	k.paramGlobals = globals
	local, err := context.NewValue()
	if err != nil {
		return err
	}
	k.assertedParams, k.assertedParamsReady = local, true
	if params != nil {
		if err = context.Append(&k.assertedParams, params); err != nil {
			return err
		}
	}
	var full *SMTParams
	if sourceParams == nil {
		full, err = NewSMTParams(context, params, globals, k.manager.Budget())
	} else if sourceParams.Context != context || sourceParams.Globals != globals {
		return uparams.ErrParamsOwner
	} else if borrowSource {
		full = sourceParams
		_, err = full.State()
	} else {
		full, err = sourceParams.Clone()
	}
	if err != nil {
		if full != nil {
			err = errors.Join(err, closeSMTKernelParams(full, borrowSource))
		}
		return err
	}
	// Source constructs the inference functor with the initial params before
	// smt_solver applies its full update. Preserve constructor-only policies.
	patternOptions, err := full.preprocess.pattern.InferenceOptions(nil)
	if err != nil {
		return errors.Join(err, closeSMTKernelParams(full, borrowSource))
	}

	// context constructs its queue against the borrowed constructor fparams.
	// The later solver update can restore case_split without replacing that queue.
	if k.caseSplitQueue == nil {
		if err = k.initializeQueueHeuristics(&full.state); err != nil {
			return errors.Join(err, closeSMTKernelParams(full, borrowSource))
		}
	}
	// smt_solver constructs smt_params and its context first, then invokes the
	// full updt_params overload. That second step updates embedded bases such as
	// dyn_ack_params, whose constructors intentionally saw an empty params_ref.
	if sourceParams == nil {
		if err = full.Update(params); err != nil {
			return errors.Join(err, closeSMTKernelParams(full, borrowSource))
		}
	}
	fullState, err := full.State()
	if err != nil {
		return errors.Join(err, closeSMTKernelParams(full, borrowSource))
	}
	dynamic, err := full.DynamicAckState()
	if err != nil {
		return errors.Join(err, closeSMTKernelParams(full, borrowSource))
	}
	arithmetic, err := full.ArithmeticState()
	if err != nil {
		return errors.Join(err, closeSMTKernelParams(full, borrowSource))
	}
	bitVector, err := full.BitVectorState()
	if err != nil {
		return errors.Join(err, closeSMTKernelParams(full, borrowSource))
	}
	quantifiers, err := full.QuantifierState()
	if err != nil {
		return errors.Join(err, closeSMTKernelParams(full, borrowSource))
	}
	proofs, err := k.manager.ProofsEnabled()
	if err != nil {
		return errors.Join(err, closeSMTKernelParams(full, borrowSource))
	}
	if err = k.internalizer.SetBinaryClauseOptimization(fullState.BinaryClauseOpt && !proofs); err != nil {
		return errors.Join(err, closeSMTKernelParams(full, borrowSource))
	}
	if err = k.internalizer.ConfigureDynamicAck(dynamic); err != nil {
		return errors.Join(err, closeSMTKernelParams(full, borrowSource))
	}
	maintenance := SMTClauseMaintenanceConfig{
		Strategy: fullState.LemmaGcStrategy, Half: fullState.LemmaGcHalf,
		Recent: fullState.RecentLemmasSize, Initial: fullState.LemmaGcInitial,
		Factor: fullState.LemmaGcFactor, InverseDecay: fullState.InvClauseDecay,
		NewOldRatio: fullState.NewOldRatio, NewActivity: fullState.NewClauseActivity,
		OldActivity: fullState.OldClauseActivity, NewRelevancy: fullState.NewClauseRelevancy,
		OldRelevancy: fullState.OldClauseRelevancy,
	}
	if err = k.internalizer.ConfigureClauseMaintenance(maintenance); err != nil {
		return errors.Join(err, closeSMTKernelParams(full, borrowSource))
	}
	if err = k.configureQueueHeuristics(fullState, false); err != nil {
		return errors.Join(err, closeSMTKernelParams(full, borrowSource))
	}
	k.relevancy.enabled = fullState.RelevancyLvl > 0
	k.internalizer.relevancyLevel = fullState.RelevancyLvl
	k.internalizer.newCore2thEq = fullState.NewCore2thEq
	k.smtParams = full
	k.smtParamsBorrowed = borrowSource
	k.internalizer.smtParams = full
	k.caseSplitQueue.liveParams = full
	k.internalizer.contextParams = &k.assertedParams
	k.patternInference, err = pattern.NewInferenceRewriterWithOptions(k.manager, patternOptions)
	if err != nil {
		return err
	}
	if err = full.preprocess.pattern.UpdateInferenceRewriter(k.patternInference, nil); err != nil {
		return err
	}
	k.smtState = fullState
	k.arithState = arithmetic
	k.bvState = bitVector
	var state SMTPreprocessorState
	if sourceParams == nil {
		state, err = k.readPreprocessorState(context, globals, params)
	} else {
		state, err = full.preprocess.State()
	}
	if err != nil {
		return err
	}
	k.preprocessState = state
	name := "preprocess"
	enabled, err := context.GetBoolName(params, &name, true)
	if err != nil {
		return err
	}
	if sourceParams != nil {
		enabled = fullState.Preprocess
	}
	value, err := rewriter.NewTheoryRewriter(k.manager, context, globals, params)
	// Construction may return a partial owner. Publish it before inspecting
	// the error so kernel destruction can retire the allocated configuration.
	k.preprocessor = value
	if err != nil {
		return err
	}
	k.internalizer.theoryRewriter = value
	if err = value.SetSubstitution(&k.assertedSubstitution); err != nil {
		return err
	}
	k.preprocess = enabled
	if err = k.setAssertedEliminateAnd(false); err != nil {
		return err
	}
	if k.modelConfig.Memory == nil {
		k.modelCooperation, err = cooperate.NewCooperation(k.manager.Budget())
		if err != nil {
			return err
		}
		k.modelConfig = model.ModelEvaluatorConfig{
			Context: context, Globals: globals, Memory: k.manager.NumeralService().RuntimeOwner(), Cooperation: k.modelCooperation,
		}
	}
	if err = k.configureQuantifiers(full, fullState, quantifiers); err != nil {
		return err
	}
	return nil
}

func closeSMTKernelParams(full *SMTParams, borrowed bool) error {
	if full == nil || borrowed {
		return nil
	}
	return full.Close()
}

func (k *SMTKernel) configureQuantifiers(full *SMTParams, state SMTParamsState, qi SMTQIState) error {
	if k == nil || k.internalizer == nil || k.preprocessor == nil || full == nil {
		return ast.ErrASTHandle
	}
	if k.internalizer.quantifierManager != nil {
		return ast.ErrASTDomain
	}
	if k.internalizer.quantifiers == nil {
		costText, err := full.QuantifierCostText()
		if err != nil {
			return err
		}
		newGenerationText, err := full.QuantifierNewGenerationText()
		if err != nil {
			return err
		}
		costs, err := NewSMTQICostModel(k.manager, costText, newGenerationText, nil)
		if err != nil {
			return err
		}
		schedule := NewSMTQIQueueSchedule(k.manager, float32(qi.EagerThreshold), float32(qi.LazyThreshold), qi.ConservativeFinalCheck)
		registry, err := NewSMTQuantifierRegistry(k.manager, schedule, costs, qi.MaxInstances)
		if err != nil {
			return errors.Join(err, costs.Close())
		}
		k.internalizer.quantifiers = registry
		k.quantifierCosts = costs
	}
	mbqiID, err := qi.MBQIId.String()
	if err != nil {
		return err
	}
	_, err = NewSMTQuantifierManager(k.internalizer, k.preprocessor, SMTQuantifierManagerConfig{
		MaxEagerMultipatterns:       qi.MaxEagerMultipatterns,
		MaxLazyMultipatternMatching: qi.MaxLazyMultipatternMatching,
		LazyInstantiation:           qi.LazyInstantiation,
		Ematching:                   state.Ematching,
		PromoteUnsat:                qi.PromoteUnsat,
		QuickUnsat:                  qi.QuickChecker >= 1,
		QuickNotSat:                 qi.QuickChecker >= 2,
		LazyQuick:                   qi.LazyQuickChecker,
		MBQI:                        qi.MBQI,
		MaxCexs:                     qi.MaxCexs,
		MaxIterations:               qi.MaxIterations,
		ForceTemplate:               qi.ForceTemplate,
		MBQIID:                      mbqiID,
		MBQITrace:                   qi.Trace,
		CandidateModel:              k.buildCandidateModelGenerator,
	})
	return err
}

func (k *SMTKernel) buildCandidateModelGenerator() (*SMTModelGenerator, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	generator, err := NewSMTModelGenerator(k.manager)
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (*SMTModelGenerator, error) {
		return nil, errors.Join(cause, generator.Close())
	}
	if err = generator.SetContext(k.checker); err != nil {
		return fail(err)
	}
	generator.searchFailure = k.lastFailure
	if err = generator.InitializeModel(k.modelConfig); err != nil {
		return fail(err)
	}
	if err = generator.BuildStateValues(k.internalizer); err != nil {
		return fail(fmt.Errorf("build state model values: %w", err))
	}
	if err = generator.registerAssertedMacros(k.assertedMacros); err != nil {
		return fail(err)
	}
	if manager := k.internalizer.quantifierManager; manager != nil && manager.modelFinder != nil {
		if err = manager.modelFinder.FixModel(generator.model); err != nil {
			return fail(err)
		}
	}
	// Source context::mk_proto_model completes partial interpretations after
	// quantifier model adjustment, including arrays with unspecified defaults.
	if err = generator.model.CompletePartialFunctions(); err != nil {
		return fail(err)
	}
	if err = generator.model.Cleanup(); err != nil {
		return fail(err)
	}
	if k.smtState.ModelCompact {
		if err = generator.model.Compress(); err != nil {
			return fail(err)
		}
	}
	return generator, nil
}

// UpdateParams refreshes the assertion rewriter and the hidden preprocessing
// switch using the kernel's borrowed parameter services.
func (k *SMTKernel) UpdateParams(params *uparams.ParamsRef) error {
	if err := k.open(); err != nil {
		return err
	}
	if k.paramContext == nil || k.paramGlobals == nil || k.smtParams == nil {
		return uparams.ErrParamsOwner
	}
	if err := k.smtParams.Update(params); err != nil {
		return err
	}
	paramsState, err := k.smtParams.State()
	if err != nil {
		return err
	}
	dynamic, err := k.smtParams.DynamicAckState()
	if err != nil {
		return err
	}
	arithmetic, err := k.smtParams.ArithmeticState()
	if err != nil {
		return err
	}
	bitVector, err := k.smtParams.BitVectorState()
	if err != nil {
		return err
	}
	quantifiers, err := k.smtParams.QuantifierState()
	if err != nil {
		return err
	}
	mbqiID, err := quantifiers.MBQIId.String()
	if err != nil {
		return err
	}
	if err = k.internalizer.UpdateDynamicAck(dynamic); err != nil {
		return err
	}
	if err = k.configureQueueHeuristics(paramsState, false); err != nil {
		return err
	}
	if !k.relevancy.enabled && paramsState.RelevancyLvl > 0 {
		k.repairRelevancy = true
	}
	k.relevancy.enabled = paramsState.RelevancyLvl > 0
	k.internalizer.relevancyLevel = paramsState.RelevancyLvl
	k.internalizer.newCore2thEq = paramsState.NewCore2thEq
	k.smtState = paramsState
	if k.patternInference != nil {
		if err = k.smtParams.preprocess.pattern.UpdateInferenceRewriter(k.patternInference, nil); err != nil {
			return err
		}
	}
	k.arithState = arithmetic
	k.bvState = bitVector
	if k.internalizer.quantifierManager != nil {
		manager := k.internalizer.quantifierManager
		manager.mbqiTrace = quantifiers.Trace
		if err = manager.UpdateMBQI(quantifiers.MBQI, mbqiID, quantifiers.MaxCexs, quantifiers.MaxIterations, quantifiers.ForceTemplate); err != nil {
			return err
		}
		if !manager.ematching && paramsState.Ematching {
			manager.mam.dirty = true
		}
		manager.ematching = paramsState.Ematching
		manager.maxEagerMultipatterns = quantifiers.MaxEagerMultipatterns
		manager.maxLazyMultipatternMatching = quantifiers.MaxLazyMultipatternMatching
		manager.lazyInstantiation = quantifiers.LazyInstantiation
		manager.registry.maxInstances = quantifiers.MaxInstances
		manager.registry.schedule.lazyThreshold = float32(quantifiers.LazyThreshold)
		// Source qi_queue caches its eager threshold and parsed cost/new-gen
		// functions in the constructor; those remain unchanged on updates.
	}
	if err = k.configureArithmeticTheories(arithmetic); err != nil {
		return err
	}
	if err = k.configureBitVectorTheories(bitVector, k.smtParams.preprocess.bit); err != nil {
		return err
	}
	if err = k.configureArrayTheories(k.smtParams.array.state); err != nil {
		return err
	}
	name := "preprocess"
	enabled, err := k.paramContext.GetBoolName(params, &name, true)
	if err != nil {
		return err
	}
	// Source updates the existing embedded parameter object. Its hidden
	// fields include policy selected by setup and have no public getters.
	state, err := k.smtParams.preprocess.State()
	if err != nil {
		return err
	}
	if params != nil {
		if err = k.paramContext.Append(&k.assertedParams, params); err != nil {
			return err
		}
	}
	// Quantifier instantiation borrows this rewriter independently of the
	// assertion-preprocessing switch. Preserve its lifetime when that switch
	// is disabled, including across later parameter updates.
	if k.preprocessor == nil {
		k.preprocessor, err = rewriter.NewTheoryRewriter(k.manager, k.paramContext, k.paramGlobals, params)
	} else {
		err = k.preprocessor.UpdateParams(params)
	}
	if err == nil {
		err = k.preprocessor.SetSubstitution(&k.assertedSubstitution)
	}
	if err == nil {
		k.internalizer.theoryRewriter = k.preprocessor
		k.preprocess = enabled
		k.preprocessState = state
		k.assertedPhaseReady = false
		err = k.setAssertedEliminateAnd(false)
	}
	return err
}

func (k *SMTKernel) open() error {
	if k == nil || k.manager == nil || k.internalizer == nil || k.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (k *SMTKernel) Internalizer() (*SMTInternalizerState, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	return k.internalizer, nil
}

func (k *SMTKernel) RegisterTheory(theory SMTTheoryInternalizer) error {
	if err := k.open(); err != nil {
		return err
	}
	if datatype, ok := theory.(*SMTDatatypeTheory); ok && k.smtParams != nil && datatype.parameterOwner == nil {
		datatype.parameterOwner = &k.smtParams.datatype
	}
	if pb, ok := theory.(*SMTPseudoBooleanTheory); ok && k.smtParams != nil && !pb.configured {
		pb.parameterOwner = &k.smtParams.pb
	}
	if array := smtArrayCore(theory); array != nil && k.smtParams != nil {
		config := k.smtParams.array.state
		array.parameterOwner = &k.smtParams.array
		if k.copiedArrayOwner != nil {
			config = k.copiedArrayOwner.state
			array.parameterOwner = k.copiedArrayOwner
		}
		if err := array.ConfigureArray(config); err != nil {
			return err
		}
	}
	if bv, ok := theory.(*SMTBitVectorTheory); ok && k.smtParams != nil {
		bv.bitBlasterParams = k.smtParams.preprocess.bit
		if k.copiedBitBlasterOwner != nil {
			bv.bitBlasterParams = *k.copiedBitBlasterOwner
		}
	}
	if configurable, ok := theory.(interface{ ConfigureBitVector(SMTBVState) error }); ok {
		config := k.bvState
		if k.copiedBVOwner != nil {
			config = k.copiedBVOwner.state
		}
		if err := configurable.ConfigureBitVector(config); err != nil {
			return err
		}
	}
	if configurable, ok := theory.(interface{ ConfigureArithmetic(SMTArithState) error }); ok {
		config := k.arithState
		if k.copiedArithmeticOwner != nil {
			config = k.copiedArithmeticOwner.state
		}
		if err := configurable.ConfigureArithmetic(config); err != nil {
			return err
		}
	}
	return k.internalizer.RegisterTheory(theory)
}

func (k *SMTKernel) Assert(formula, proof ast.ASTHandle) (err error) {
	if k == nil || k.manager == nil {
		return ast.ErrASTHandle
	}
	if (k.setupLogic != "QF_NRA" && k.setupLogic != "QF_NIA") || !k.nraInputsValid || k.assertedFormulasInconsistent() {
		return k.assertFormulaCore(formula, proof)
	}
	ref, err := k.manager.NewRef(formula)
	if err != nil {
		return err
	}
	if err = k.assertFormulaCore(formula, proof); err != nil {
		k.nraInputsValid = false
		return errors.Join(err, k.manager.DelRef(&ref))
	}
	k.nraInputs = append(k.nraInputs, ref)
	return nil
}
func (k *SMTKernel) assertFormulaCore(formula, proof ast.ASTHandle) error {
	if err := k.open(); err != nil {
		return err
	}
	sort, err := k.manager.GetSort(formula)
	if err != nil {
		return err
	}
	boolean, err := k.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if sort != boolean {
		return ast.ErrASTDomain
	}
	if !proof.IsNull() {
		if isProof, err := k.manager.IsProof(proof); err != nil || !isProof {
			if err != nil {
				return err
			}
			return ast.ErrASTDomain
		}
	}
	if proof.IsNull() {
		proofs, err := k.manager.ProofsEnabled()
		if err != nil {
			return err
		}
		if proofs {
			proof, err = k.manager.MkProofAsserted(formula)
			if err != nil {
				return err
			}
		}
	}
	// The source owns expr_ref/proof_ref inputs before changing rewriter
	// parameters and flushing caches. Keep borrowed inputs alive across that
	// flush even when the previous rewriter cache supplied their last pin.
	input, err := k.manager.NewRef(formula)
	if err != nil {
		return err
	}
	defer k.manager.DelRef(&input)
	inputProof, err := k.manager.NewRef(proof)
	if err != nil {
		return err
	}
	defer k.manager.DelRef(&inputProof)
	if k.assertedFormulasInconsistent() {
		return nil
	}
	if k.preprocess {
		if err = k.traceAssertedExpression("assert_expr_bug", "assert_expr", 151, "", formula); err != nil {
			return err
		}
	}
	hasQuantifiers, err := k.manager.HasQuantifiers(formula)
	if err != nil {
		return err
	}
	if err = k.setAssertedEliminateAnd(false); err != nil {
		return err
	}
	formula, proof, err = k.preprocessAssertion(formula, proof)
	if err != nil {
		return err
	}
	if k.preprocess {
		if err = k.traceAssertedExpression("assert_expr_bug", "assert_expr", 160, "after...\n", formula); err != nil {
			return err
		}
	}
	if k.preprocess {
		defer k.manager.DecRef(formula)
		if !proof.IsNull() {
			defer k.manager.DecRef(proof)
		}
	}
	k.assertedHasQuantifiers = k.assertedHasQuantifiers || hasQuantifiers
	limit := len(k.assertions)
	wasInconsistent := k.assertedInconsistent
	if err = k.pushAssertion(formula, proof); err == nil {
		err = k.traceAssertedBody("asserted_formulas_bug", "assert_expr", 166, func(out io.Writer) error {
			if _, traceErr := io.WriteString(out, "after assert_expr\n"); traceErr != nil {
				return traceErr
			}
			return k.DisplayAssertedFormulas(out)
		})
		if err == nil {
			return nil
		}
	}
	for index := len(k.assertions); index > limit; index-- {
		assertion := &k.assertions[index-1]
		if !assertion.Proof.IsNull() {
			err = errors.Join(err, k.manager.DecRef(assertion.Proof))
		}
		err = errors.Join(err, k.manager.DecRef(assertion.Formula))
	}
	k.assertions = k.assertions[:limit]
	k.assertedInconsistent = wasInconsistent
	return err
}

func (k *SMTKernel) Assertions() []SMTKernelAssertion {
	if k == nil {
		return nil
	}
	return append([]SMTKernelAssertion(nil), k.assertions...)
}

func (k *SMTKernel) internalizePending() error {
	// context::get_cancel_flag is a counted reslimit::inc, including when
	// the asserted manager is already inconsistent or has no pending rows.
	if canceled, err := k.manager.Canceled(); canceled || err != nil {
		return err
	}
	ready, err := k.reduceAssertedPending()
	if err != nil || !ready {
		return err
	}
	if k.assertedFormulasInconsistent() {
		if k.internalizer.Inconsistent() {
			return nil
		}
		proof, err := k.AssertionInconsistencyProof()
		if err != nil {
			return err
		}
		// Set the Boolean conflict directly: pending expressions must never
		// enter theory/quantifier internalization on this source branch.
		if err := k.internalizer.assignRootLiteral(SMTFalseLiteral, proof); err != nil {
			return err
		}
		if !proof.IsNull() {
			return k.manager.SetRef(k.unsatProof, proof)
		}
		return nil
	}
	head := k.assertedHead
	for head < len(k.assertions) {
		if canceled, err := k.manager.Canceled(); canceled || err != nil {
			return errors.Join(err, k.commitAssertedTo(head))
		}
		assertion := k.assertions[head]
		if err := k.internalizer.Assert(assertion.Formula, assertion.Proof, 0); err != nil {
			return errors.Join(err, k.commitAssertedTo(head))
		}
		head++
	}
	return k.commitAssertedTo(head)
}

// Source commit forbids declarations and retains each completed row's binding
// in the same scoped substitution borrowed by subsequent assertion rewriting.
func (k *SMTKernel) commitAssertedTo(head int) error {
	if head < k.assertedHead || head > len(k.assertions) {
		return ast.ErrASTDomain
	}
	rows := make([]ast.ASTHandle, 0, head-k.assertedHead)
	for _, row := range k.assertions[k.assertedHead:head] {
		rows = append(rows, row.Formula)
	}
	if err := k.assertedMacros.MarkForbidden(rows); err != nil {
		return err
	}
	depth := make(map[ast.ASTHandle]uint32)
	for _, row := range k.assertions[k.assertedHead:head] {
		if err := k.addAssertionSubstitution(row, depth); err != nil {
			return err
		}
	}
	k.assertedHead = head
	return nil
}

func (k *SMTKernel) Push() error {
	if err := k.open(); err != nil {
		return err
	}
	if k.smtParams != nil {
		if err := k.configureArrayTheories(k.smtParams.array.state); err != nil {
			return err
		}
	}
	if k.copiedBVOwner != nil {
		if err := k.configureBitVectorTheories(k.bvState, k.smtParams.preprocess.bit); err != nil {
			return err
		}
	}
	if k.copiedArithmeticOwner != nil {
		if err := k.configureArithmeticTheories(k.arithState); err != nil {
			return err
		}
	}
	if err := k.setupAssertedBounds(false); err != nil {
		return err
	}
	if err := k.configureQueueHeuristics(k.smtState, !k.booleanSetupSeeded); err != nil {
		return err
	}
	k.booleanSetupSeeded = true
	if err := k.internalizer.SetupTheories(); err != nil {
		return err
	}
	wasConsistent := !k.internalizer.Inconsistent()
	if err := k.internalizePending(); err != nil {
		return err
	}
	// Source context::push propagates the base assertions before saving a
	// scope. Otherwise relevance discovered during the next check is scoped
	// away, and surviving arithmetic terms can disappear from later models.
	if _, err := k.internalizer.PropagateCore(); err != nil {
		return err
	}
	if wasConsistent && k.internalizer.Inconsistent() {
		if _, err := k.resolveCheckConflict(); err != nil {
			return err
		}
	}
	if err := k.assertedSourceAssertion(178, "inconsistent() || m_qhead == m_formulas.size() || m.canceled()", func() (bool, error) {
		if k.assertedInconsistent || k.assertedHead == len(k.assertions) {
			return true, nil
		}
		return k.manager.Canceled()
	}); err != nil {
		return err
	}
	if err := k.traceAssertedBody("asserted_formulas_scopes", "push_scope", 179, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "before push: %d\n", len(k.scopes))
		return err
	}); err != nil {
		return err
	}
	if err := k.manager.ScopedExprSubstitutionPush(k.assertedScopedSubst); err != nil {
		return err
	}
	if err := k.assertedMacros.PushScope(); err != nil {
		return errors.Join(err, k.manager.ScopedExprSubstitutionPop(k.assertedScopedSubst, 1))
	}
	if err := k.definedNames.Push(); err != nil {
		return errors.Join(err, k.assertedMacros.PopScope(1), k.manager.ScopedExprSubstitutionPop(k.assertedScopedSubst, 1))
	}
	if err := k.bvSharing.PushScope(); err != nil {
		return errors.Join(err, k.definedNames.Pop(1), k.assertedMacros.PopScope(1), k.manager.ScopedExprSubstitutionPop(k.assertedScopedSubst, 1))
	}
	if err := k.internalizer.PushScope(); err != nil {
		return errors.Join(err, k.bvSharing.PopScope(1), k.definedNames.Pop(1), k.assertedMacros.PopScope(1), k.manager.ScopedExprSubstitutionPop(k.assertedScopedSubst, 1))
	}
	// asserted_formulas::push_scope commits even an inconsistent manager.
	if err := k.commitAssertedTo(len(k.assertions)); err != nil {
		return errors.Join(err, k.internalizer.PopScopes(1), k.bvSharing.PopScope(1), k.definedNames.Pop(1), k.assertedMacros.PopScope(1), k.manager.ScopedExprSubstitutionPop(k.assertedScopedSubst, 1))
	}
	k.scopes = append(k.scopes, len(k.assertions))
	k.nraScopes = append(k.nraScopes, len(k.nraInputs))
	k.assertedScopeConflicts = append(k.assertedScopeConflicts, k.assertedInconsistent)
	if err := k.assertedSourceAssertion(184, "inconsistent() || s.m_formulas_lim == m_qhead || m.canceled()", func() (bool, error) {
		if k.assertedInconsistent || k.scopes[len(k.scopes)-1] == k.assertedHead {
			return true, nil
		}
		return k.manager.Canceled()
	}); err != nil {
		return err
	}
	return k.traceAssertedBody("asserted_formulas_scopes", "push_scope", 190, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "after push: %d\n", len(k.scopes))
		return err
	})
}

func (k *SMTKernel) Pop(count uint32) error {
	if err := k.open(); err != nil {
		return err
	}
	if count == 0 || count > uint32(len(k.scopes)) {
		return ast.ErrASTDomain
	}
	if err := k.traceAssertedBody("asserted_formulas_scopes", "pop_scope", 194, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "before pop %d of %d\n", count, len(k.scopes))
		return err
	}); err != nil {
		return err
	}
	level := len(k.scopes) - int(count)
	limit := k.scopes[level]
	if err := k.internalizer.PopScopes(count); err != nil {
		return err
	}
	if err := k.bvSharing.PopScope(count); err != nil {
		return err
	}
	if err := k.assertedMacros.PopScope(count); err != nil {
		return err
	}
	if err := k.definedNames.Pop(count); err != nil {
		return err
	}
	if err := k.manager.ScopedExprSubstitutionPop(k.assertedScopedSubst, count); err != nil {
		return err
	}
	for index := len(k.assertions); index > limit; index-- {
		assertion := &k.assertions[index-1]
		if !assertion.Proof.IsNull() {
			if err := k.manager.DecRef(assertion.Proof); err != nil {
				return err
			}
		}
		if err := k.manager.DecRef(assertion.Formula); err != nil {
			return err
		}
	}
	k.assertions = k.assertions[:limit]
	if k.assertedHead > limit {
		k.assertedHead = limit
	}
	if err := k.restoreNRAInputs(k.nraScopes[level]); err != nil {
		return err
	}
	k.nraScopes = k.nraScopes[:level]
	k.scopes = k.scopes[:level]
	k.assertedInconsistent = k.assertedScopeConflicts[level]
	k.assertedScopeConflicts = k.assertedScopeConflicts[:level]
	if k.preprocessor != nil {
		if err := k.flushAssertedCache(); err != nil {
			return err
		}
	}
	return k.traceAssertedBody("asserted_formulas_scopes", "pop_scope", 206, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "after pop %d\n", count)
		return err
	})
}

func (k *SMTKernel) ScopeLevel() uint32 {
	if k == nil {
		return 0
	}
	return uint32(len(k.scopes))
}

func (k *SMTKernel) prepareCheck(useStaticFeatures bool) error {
	if err := k.configureCheck(useStaticFeatures); err != nil {
		return err
	}
	return k.internalizeCheckAssertions()
}

func (k *SMTKernel) configureCheck(useStaticFeatures bool) error {
	if err := k.open(); err != nil {
		return err
	}
	if k.smtParams != nil {
		if err := k.configureArrayTheories(k.smtParams.array.state); err != nil {
			return err
		}
	}
	if k.copiedBVOwner != nil {
		if err := k.configureBitVectorTheories(k.bvState, k.smtParams.preprocess.bit); err != nil {
			return err
		}
	}
	if k.copiedArithmeticOwner != nil {
		if err := k.configureArithmeticTheories(k.arithState); err != nil {
			return err
		}
	}
	if err := k.setupAssertedBounds(useStaticFeatures); err != nil {
		return fmt.Errorf("smt kernel configure asserted bounds: %w", err)
	}
	if err := k.configureQueueHeuristics(k.smtState, !k.booleanSetupSeeded); err != nil {
		return fmt.Errorf("smt kernel configure Boolean heuristics: %w", err)
	}
	k.booleanSetupSeeded = true
	if err := k.internalizer.SetupTheories(); err != nil {
		return fmt.Errorf("smt kernel setup theories: %w", err)
	}
	return nil
}

func (k *SMTKernel) internalizeCheckAssertions() error {
	if err := k.internalizePending(); err != nil {
		return fmt.Errorf("smt kernel internalize assertions: %w", err)
	}
	if k.assertedFormulasInconsistent() || k.internalizer.Inconsistent() {
		return nil
	}
	// Cached assertions may have been internalized while relevancy filtering
	// was disabled. Reestablish their root relevance before each check so a
	// live option update or scope pop cannot omit them from the next model.
	if k.repairRelevancy && k.relevancy.enabled {
		for _, assertion := range k.assertions[:k.assertedHead] {
			if err := k.internalizer.markRelevant(assertion.Formula); err != nil {
				return fmt.Errorf("smt kernel mark asserted formulas relevant: %w", err)
			}
			if err := k.relevancy.restoreExpressionWatches(assertion.Formula); err != nil {
				return fmt.Errorf("smt kernel restore asserted relevance watches: %w", err)
			}
		}
	}
	return nil
}

func (k *SMTKernel) Check() (result lbool.LBool, resultErr error) {
	ready, err := k.beginCheck()
	if err != nil || !ready {
		return lbool.Undef, err
	}
	if k.internalizer.Inconsistent() {
		return lbool.False, nil
	}
	if err := k.configureCheck(false); err != nil {
		return lbool.Undef, err
	}
	assumptions, refs, err := k.addTheoryAssumptions(nil)
	defer func() {
		resultErr = errors.Join(resultErr, k.releaseTheoryAssumptions(refs))
		if resultErr != nil {
			result = lbool.Undef
		}
	}()
	if err != nil {
		return lbool.Undef, err
	}
	if len(assumptions) != 0 {
		return k.checkConfiguredAssumptions(assumptions)
	}
	if result, handled, err := k.tryNLSAT(); handled || err != nil {
		return k.finalizeCheck(result, err)
	}
	if err := k.internalizeCheckAssertions(); err != nil {
		return lbool.Undef, err
	}
	if k.assertedFormulasInconsistent() {
		return lbool.False, nil
	}
	if k.internalizer.Inconsistent() {
		return k.resolveCheckConflict()
	}
	return k.finalizeCheck(k.search())
}

func (k *SMTKernel) ConfigureModelGeneration(enabled bool, config model.ModelEvaluatorConfig) error {
	if err := k.open(); err != nil {
		return err
	}
	k.produceModel = enabled
	k.modelConfig = config
	return nil
}

func (k *SMTKernel) buildGeneratedModel() error {
	var generator *SMTModelGenerator
	if manager := k.internalizer.quantifierManager; manager != nil {
		generator = manager.acceptedModel
		manager.acceptedModel = nil
	}
	if !k.produceModel {
		if generator != nil {
			return generator.Close()
		}
		return nil
	}
	var err error
	if generator == nil {
		generator, err = k.buildCandidateModelGenerator()
		if err != nil {
			return err
		}
	}
	generated, err := k.finalizeGeneratedModel(generator)
	if err != nil {
		return err
	}
	k.generated = generated
	return nil
}

// finalizeGeneratedModel consumes the generator and returns one model
// reference for the kernel, without changing the currently published model.
func (k *SMTKernel) finalizeGeneratedModel(generator *SMTModelGenerator) (*model.Model, error) {
	fail := func(cause error) (*model.Model, error) { return nil, errors.Join(cause, generator.Close()) }
	generated, err := generator.FinalModel()
	if err != nil {
		return fail(err)
	}
	if err = k.addRecursiveFunctionsToModel(generated.Core()); err != nil {
		return nil, errors.Join(err, generated.Close(), generator.Close())
	}
	if err = generator.Close(); err != nil {
		return nil, errors.Join(err, generated.Close())
	}
	if err = generated.Core().IncRef(); err != nil {
		return nil, errors.Join(err, generated.Close())
	}
	return generated, nil
}

func (k *SMTKernel) clearGeneratedModel() error {
	if k == nil || k.generated == nil {
		return nil
	}
	consumed, err := k.generated.Core().DecRefConsumed()
	if consumed {
		k.generated = nil
	}
	return err
}

func (k *SMTKernel) GeneratedModel() *model.ModelCore {
	if k == nil || k.generated == nil {
		return nil
	}
	return k.generated.Core()
}

func (k *SMTKernel) LastFailure() (SMTFailure, error) {
	if err := k.open(); err != nil {
		return SMTFailureUnknown, err
	}
	return k.lastFailure, nil
}

func (k *SMTKernel) ReasonUnknown() (string, error) {
	if err := k.open(); err != nil {
		return "", err
	}
	switch k.lastFailure {
	case SMTFailureOK, SMTFailureUnknown:
		return k.unknown, nil
	case SMTFailureTimeout:
		return "timeout", nil
	case SMTFailureMemory:
		return "memout", nil
	case SMTFailureCanceled:
		return "canceled", nil
	case SMTFailureConflictLimit:
		return "max-conflicts-reached", nil
	case SMTFailureTheory:
		var reason strings.Builder
		reason.WriteString("(incomplete (theory")
		for _, theory := range k.internalizer.IncompleteTheories() {
			if named, ok := theory.(interface{ Name() string }); ok {
				if name := named.Name(); name != "" {
					reason.WriteByte(' ')
					reason.WriteString(name)
				}
			}
		}
		reason.WriteString("))")
		return reason.String(), nil
	case SMTFailureResourceLimit:
		return "(resource limits reached)", nil
	case SMTFailureQuantifiers:
		return "(incomplete quantifiers)", nil
	default:
		return "", ast.ErrASTDomain
	}
}

func (k *SMTKernel) SetReasonUnknown(message string) error {
	if err := k.open(); err != nil {
		return err
	}
	k.unknown = message
	return nil
}

func (k *SMTKernel) captureRelevantLabels() error {
	k.labels = nil
	for variable := int32(SMTFirstBoolVar); int(variable) < len(k.internalizer.boolExpr); variable++ {
		expression := k.internalizer.boolExpr[variable]
		if !k.relevancy.IsRelevant(expression) {
			continue
		}
		value, err := k.internalizer.Assignment(NewSMTLiteral(variable, false))
		if err != nil {
			return err
		}
		if value != lbool.True {
			continue
		}
		var names []usymbol.Symbol
		label, err := k.manager.IsLabelLit(expression, &names)
		if err != nil {
			return err
		}
		if label {
			k.labels = append(k.labels, names...)
		}
	}
	return nil
}

func (k *SMTKernel) Labels() ([]usymbol.Symbol, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	return append([]usymbol.Symbol(nil), k.labels...), nil
}

func (k *SMTKernel) SetProgressCallback(callback solver.ProgressCallback) error {
	if err := k.open(); err != nil {
		return err
	}
	k.callback = callback
	return nil
}

// Proof returns the retained proof established by assertion internalization or
// search. It is null when proof generation is disabled or no unsatisfiable
// result has been retained since the most recent result reset.
func (k *SMTKernel) Proof() (ast.ASTHandle, error) {
	if err := k.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return k.manager.RefGet(k.unsatProof)
}

func (k *SMTKernel) Inconsistent() bool {
	return k != nil && k.internalizer != nil && k.internalizer.Inconsistent()
}

func (k *SMTKernel) Statistics() SMTContextStatistics {
	if k == nil || k.internalizer == nil {
		return SMTContextStatistics{}
	}
	return k.internalizer.Statistics()
}

// ResetStatistics clears the cumulative counters retained by the kernel.
func (k *SMTKernel) ResetStatistics() error {
	// Source smt_kernel.cpp imp::reset_statistics intentionally has no
	// body. Tactics clear their separately stored statistics themselves.
	return k.open()
}

// Canceled performs the same resource-limit check used by the source context.
// The check accounts for one resource step before reporting the cancel state.
func (k *SMTKernel) Canceled() (bool, error) {
	if err := k.open(); err != nil {
		return false, err
	}
	return k.manager.Canceled()
}

// Display renders the retained asserted formulas in the source kernel format.
func (k *SMTKernel) Display(out io.Writer) error {
	if err := k.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	printer, err := ast.NewLegacySMTPrinter(k.manager)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, "(kernel"); err != nil {
		return err
	}
	for _, assertion := range k.assertions {
		if _, err = io.WriteString(out, "\n  "); err != nil {
			return err
		}
		if err = printer.DisplayAST(out, assertion.Formula, 2, nil); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, ")")
	return err
}

func (k *SMTKernel) Close() error {
	if k == nil || k.manager == nil {
		return ast.ErrASTHandle
	}
	if k.closed {
		return nil
	}
	if k.univariateNRA != nil {
		if err := k.univariateNRA.Close(); err != nil {
			return err
		}
		k.univariateNRA = nil
	}
	if err := k.restoreNRAInputs(0); err != nil {
		return err
	}
	var result error
	result = errors.Join(result, k.clearLiteralResults())
	result = errors.Join(result, k.clearGeneratedModel())
	result = errors.Join(result, k.clearUnsatCore())
	if k.bvSharing != nil {
		if err := k.bvSharing.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			k.bvSharing = nil
		}
	}
	result = errors.Join(result, k.manager.DelRef(&k.unsatProof))
	if k.patternInference != nil {
		if err := k.patternInference.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			k.patternInference = nil
		}
	}
	if k.preprocessor != nil {
		if err := k.preprocessor.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			k.preprocessor = nil
		}
	}
	if k.pendingSetupLRA != nil {
		if err := k.pendingSetupLRA.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			k.pendingSetupLRA = nil
		}
	}
	result = errors.Join(result, k.internalizer.Close())
	if k.preprocessor == nil {
		if k.assertedScopedSubst != (ast.ASTScopedExprSubstitution{}) {
			result = errors.Join(result, k.manager.DelScopedExprSubstitution(&k.assertedScopedSubst))
		}
		if k.assertedScopedSubst == (ast.ASTScopedExprSubstitution{}) && k.assertedSubstitution != (ast.ASTExprSubstitution{}) {
			result = errors.Join(result, k.manager.DelExprSubstitution(&k.assertedSubstitution))
		}
	}
	result = errors.Join(result, k.assertedMacros.Close())
	result = errors.Join(result, k.definedNames.Close())
	if k.quantifierCosts != nil {
		result = errors.Join(result, k.quantifierCosts.Close())
	}
	if k.modelCooperation != nil {
		result = errors.Join(result, k.modelCooperation.Close())
	}
	if k.smtParams != nil && !k.smtParamsBorrowed {
		result = errors.Join(result, k.smtParams.Close())
	}
	if k.assertedParamsReady {
		if err := k.paramContext.Del(&k.assertedParams); err != nil {
			result = errors.Join(result, err)
		} else {
			k.assertedParamsReady = false
		}
	}
	for index := len(k.assertions); index > 0; index-- {
		assertion := &k.assertions[index-1]
		if !assertion.Proof.IsNull() {
			result = errors.Join(result, k.manager.DecRef(assertion.Proof))
		}
		result = errors.Join(result, k.manager.DecRef(assertion.Formula))
	}
	if result == nil {
		k.assertions = nil
		k.scopes = nil
		k.assertedScopeConflicts = nil
		k.assertedInconsistent = false
		k.caseSplitQueue = nil
		k.queueWarnings = nil
		k.booleanSetupSeeded = false
		k.internalizer = nil
		k.checker = nil
		k.relevancy = nil
		k.paramContext = nil
		k.paramGlobals = nil
		k.smtParams = nil
		k.smtParamsBorrowed = false
		k.smtState = SMTParamsState{}
		k.preprocessState = SMTPreprocessorState{}
		k.quantifierCosts = nil
		k.modelCooperation = nil
		k.manager = nil
		k.closed = true
	}
	return result
}
