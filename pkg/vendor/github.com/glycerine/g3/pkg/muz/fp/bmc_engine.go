package fp

// Source: src/muz/bmc/dl_bmc_engine.cpp and fixedpoint engine registration.

import (
	"errors"
	"fmt"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var errRetryUnslicedProof = errors.New("muz/fp: retry proof without slicing")

// configureBMCSolverParams installs the three source setup profiles. Nonlinear
// BMC leaves MBQI at its inherited default; linear BMC disables it and QBMC
// enables it explicitly.
func (e *PropositionalEngine) configureBMCSolverParams(local *uparams.ParamsRef, linear, quantified bool) error {
	if e == nil || e.params == nil || local == nil || quantified && !linear {
		return ErrPropositionalEngine
	}
	compact := "compact"
	if err := e.params.SetBoolName(local, &compact, true); err != nil {
		return err
	}
	if linear {
		mbqi := "mbqi"
		if err := e.params.SetBoolName(local, &mbqi, quantified); err != nil {
			return err
		}
	}
	relevancy := "relevancy"
	value := uint32(2)
	if linear && !quantified {
		value = 0
	}
	return e.params.SetUIntName(local, &relevancy, value)
}

// onlyErrorKind ensures cleanup errors are not hidden by a retry signal.
func onlyErrorKind(err, target error) bool {
	if err == nil {
		return false
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		children := many.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !onlyErrorKind(child, target) {
				return false
			}
		}
		return true
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return onlyErrorKind(one.Unwrap(), target)
	}
	return err == target
}

// queryRelationExpression implements engine_base's single-relation wrapper,
// which turns a relation into an expression query with fresh variables.
func (e *PropositionalEngine) queryRelationExpression(declaration ast.ASTHandle) (status lbool.LBool, resultErr error) {
	ref, err := e.makeRelationQueryExpression(declaration)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(e.manager, []ast.ASTRef{ref})) }()
	query, err := e.manager.RefGet(ref)
	if err != nil {
		return lbool.Undef, err
	}
	return e.QueryExpression(query)
}

// makeRelationQueryExpression owns the source expr_ref_vector arguments and
// query before any later construction can fail. QueryExpression performs the
// source existential-variable hoisting when it builds the query rule.
func (e *PropositionalEngine) makeRelationQueryExpression(declaration ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	var roots []ast.ASTRef
	defer func() {
		if resultErr != nil {
			result = ast.ASTRef{}
			resultErr = errors.Join(resultErr, closeASTRefs(e.manager, roots))
			return
		}
		// Keep the query in the same owner until its argument references have
		// been released. A refusal must not publish an unfinished result.
		if cleanup := releaseQueryReferences(e.manager, roots[:len(roots)-1]); cleanup != nil {
			result = ast.ASTRef{}
			resultErr = &queryReferenceCleanupFailure{manager: e.manager, runtime: e.manager.NumeralService().RuntimeOwner(), references: roots, cause: cleanup, cleanup: cleanup}
			return
		}
		result = roots[len(roots)-1]
	}()
	arity, err := e.manager.GetArity(declaration)
	if err != nil {
		return result, err
	}
	arguments := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		sort, err := e.manager.GetDomain(declaration, i)
		if err != nil {
			return result, err
		}
		root, err := e.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return result, err
		}
		roots = append(roots, root)
		arguments[i], err = e.manager.MkVar(i, sort)
		if !arguments[i].IsNull() {
			err = errors.Join(err, e.manager.SetRef(root, arguments[i]))
		}
		if err != nil {
			return result, err
		}
	}
	root, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	roots = append(roots, root)
	query, err := e.manager.MkApp(declaration, arguments)
	if !query.IsNull() {
		err = errors.Join(err, e.manager.SetRef(root, query))
	}
	return result, err
}

func (e *PropositionalEngine) clearBMCTrace() (result error) {
	if e.bmcProof != (ast.ASTRef{}) {
		if err := e.manager.DelRef(&e.bmcProof); err != nil {
			return err
		}
	}
	for len(e.bmcTrace) != 0 {
		i := len(e.bmcTrace) - 1
		if err := e.bmcTrace[i].Close(); err != nil {
			return errors.Join(result, err)
		}
		e.bmcTrace = e.bmcTrace[:i]
	}
	return result
}

func (e *PropositionalEngine) queryBMCPredicate(declaration ast.ASTHandle) (status lbool.LBool, resultErr error) {
	status, resultErr = e.queryBMCPredicateWithSliceProof(declaration, true)
	if onlyErrorKind(resultErr, errRetryUnslicedProof) {
		return e.queryBMCPredicateWithSliceProof(declaration, false)
	}
	return status, resultErr
}

func (e *PropositionalEngine) queryBMCPredicateWithSliceProof(declaration ast.ASTHandle, allowSliceProof bool) (status lbool.LBool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, ErrPropositionalEngine
	}
	sourceDeclaration := declaration
	if err := e.clearBMCTrace(); err != nil {
		return lbool.Undef, err
	}
	e.bmcStatistics.Reset()
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		valid, err := e.set.Finalize()
		if err != nil {
			return lbool.Undef, err
		}
		if !valid {
			return lbool.Undef, ErrUnstratifiedNegation
		}
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	proofTrace, err := view.Bool("generate_proof_trace", false)
	if err != nil {
		return lbool.Undef, err
	}
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	// BMC uses the same default Horn transformations and optional slicing as
	// Spacer before it selects the linear or nonlinear compiler. Both BMC
	// backends translate sliced proofs back to the original rules below.
	preprocessed, err := e.preprocessHornRulesWithStages(declaration, nil, true, true, allowSliceProof, true, false)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, preprocessed.Close()) }()
	rules := preprocessed.rules
	declaration, err = rules.DerivedPredicate(declaration)
	if err != nil {
		return lbool.Undef, fmt.Errorf("muz/fp: resolve preprocessed BMC query: %w", err)
	}
	group, err := rules.PredicateRules(declaration)
	if err != nil {
		return lbool.Undef, err
	}
	for i := len(group) - 1; i >= 0; i-- {
		err = errors.Join(err, group[i].Close())
	}
	if err != nil {
		return lbool.Undef, err
	}
	canceled, err := e.manager.Canceled()
	if err != nil {
		return lbool.Undef, err
	}
	if canceled {
		return lbool.Undef, z3exception.NewDefaultException(e.manager.ResourceLimit().CancelMessage())
	}
	if len(group) == 0 {
		return lbool.False, nil
	}
	linear, err := bmc.IsLinear(rules)
	if err != nil {
		return lbool.Undef, err
	}
	// Nonlinear answers require rule proofs even when the caller asks for
	// them only after the query. Rebuild nonlinear preprocessing with proofs;
	// ordinary linear queries keep their existing certificate path.
	if !linear && !proofTrace && !e.forceProofTrace {
		if err = preprocessed.Close(); err != nil {
			return lbool.Undef, err
		}
		e.forceProofTrace = true
		defer func() { e.forceProofTrace = false }()
		return e.queryBMCPredicateWithSliceProof(sourceDeclaration, allowSliceProof)
	}
	if !linear {
		// Keep this after the proof-preparation retry so one public query emits
		// the source warning exactly once.
		warnNonlinearBMC()
	}
	depth, err := view.UInt("bmc.linear_unrolling_depth", math.MaxUint32)
	if err != nil {
		return lbool.Undef, err
	}
	local, err := e.params.NewValue()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.params.Del(&local)) }()
	quantified := linear && e.backend == "qbmc"
	if err = e.configureBMCSolverParams(&local, linear, quantified); err != nil {
		return lbool.Undef, err
	}
	solver, err := smtcore.NewSMTSolver(e.manager, &local, "", smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, solver.CollectStatistics(&e.bmcStatistics), solver.Close())
	}()
	compiler, err := bmc.NewCompiler(e.manager)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, compiler.Close()) }()
	var level uint32
	var md *model.ModelCore
	var normalization transforms.RuleNormalizationParams
	if linear {
		normalization, err = e.ruleNormalizationParams()
		if err != nil {
			return lbool.Undef, err
		}
	}
	if quantified {
		cooperation, e2 := cooperate.NewCooperation(e.manager.Budget())
		if e2 != nil {
			return lbool.Undef, e2
		}
		defer func() { resultErr = errors.Join(resultErr, cooperation.Close()) }()
		services := model.ModelEvaluatorConfig{Context: e.params, Globals: e.globals, Memory: e.manager.NumeralService().RuntimeOwner(), Cooperation: cooperation}
		status, level, md, err = compiler.SearchQuantifiedLinearInScope(rules, declaration, solver, func(width uint32, modelCore *model.ModelCore) error {
			var cause error
			e.bmcTrace, e.bmcProof, cause = compiler.QuantifiedLinearTraceAndProof(rules, declaration, width, modelCore, services, normalization)
			return cause
		})
	} else if linear {
		status, level, err = compiler.SearchLinear(rules, declaration, solver, depth)
	} else {
		status, level, err = compiler.SearchNonlinear(rules, declaration, solver)
	}
	if err != nil || status != lbool.True {
		// A successful model callback can be followed by a refused solver Pop.
		// Retire its unpublished outputs, keeping any refused owner on the
		// engine for the ordinary cleanup continuation.
		if quantified {
			err = errors.Join(err, e.clearBMCTrace())
		}
		return status, err
	}
	if !quantified {
		md, err = solver.AcquireModel()
		if err != nil {
			return lbool.Undef, err
		}
	}
	if md == nil {
		return lbool.Undef, ast.ErrASTHandle
	}
	defer func() { resultErr = errors.Join(resultErr, md.DecRef()) }()
	if quantified {
		// Trace and proof were constructed while the winning scope was live.
	} else if linear {
		e.bmcTrace, err = compiler.LinearTrace(rules, declaration, level, md)
		if err == nil {
			services := model.ModelEvaluatorConfig{Context: e.params, Globals: e.globals}
			e.bmcProof, err = compiler.LinearProof(rules, declaration, level, md, services, normalization)
		}
	} else {
		cooperation, e2 := cooperate.NewCooperation(e.manager.Budget())
		if e2 != nil {
			return lbool.Undef, e2
		}
		defer func() { resultErr = errors.Join(resultErr, cooperation.Close()) }()
		services := model.ModelEvaluatorConfig{Context: e.params, Globals: e.globals, Memory: e.manager.NumeralService().RuntimeOwner(), Cooperation: cooperation}
		e.bmcTrace, err = compiler.NonlinearTrace(rules, declaration, level, md, services)
		if err == nil && (proofTrace || e.forceProofTrace) {
			e.bmcProof, err = compiler.NonlinearProof(rules, declaration, level, md, services)
		}
	}
	if err != nil {
		return lbool.Undef, err
	}
	if preprocessed.sliceProof != nil && e.bmcProof != (ast.ASTRef{}) {
		original, cause := e.manager.RefGet(e.bmcProof)
		if cause != nil {
			return lbool.Undef, cause
		}
		converted, cause := preprocessed.sliceProof.Convert(original)
		if cause != nil {
			if onlyErrorKind(cause, transforms.ErrSliceProofUnsupported) {
				return lbool.Undef, errRetryUnslicedProof
			}
			return lbool.Undef, cause
		}
		newProof, cause := e.manager.RefGet(converted)
		if cause == nil {
			var before, after ast.ASTHandle
			before, cause = e.manager.GetProofFact(original)
			if cause == nil {
				after, cause = e.manager.GetProofFact(newProof)
			}
			if cause == nil && before != after {
				cause = errRetryUnslicedProof
			}
		}
		if cause != nil {
			return lbool.Undef, errors.Join(cause, e.manager.DelRef(&converted))
		}
		if cause = e.manager.DelRef(&e.bmcProof); cause != nil {
			return lbool.Undef, errors.Join(cause, e.manager.DelRef(&converted))
		}
		e.bmcProof = converted
	}
	return status, nil
}

// bmcRulesAlongTrace returns caller-owned formulas from the latest query.
func (e *PropositionalEngine) bmcRulesAlongTrace() (formulas []ast.ASTRef, names []symbol.Symbol, resultErr error) {
	if err := e.requireBMCSnapshot(); err != nil {
		return nil, nil, err
	}
	a := &contextFormulaAttempt{manager: e.manager}
	defer func() {
		if resultErr != nil {
			a.references = append(a.references, formulas...)
			formulas, names = nil, nil
		}
		resultErr = a.finish(resultErr)
	}()
	for _, rule := range e.bmcTrace {
		formula, err := rule.ToFormula()
		if formula != (ast.ASTRef{}) {
			formulas = append(formulas, formula)
		}
		if err != nil {
			return formulas, names, err
		}
		name, err := rule.NameSymbol()
		if err != nil {
			return formulas, names, err
		}
		names = append(names, name)
	}
	return formulas, names, nil
}
