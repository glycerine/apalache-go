package fp

// Source: src/muz/pdr/pdr_dl_interface.cpp query setup and PDR dispatch.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) queryPDR(declaration ast.ASTHandle) (lbool.LBool, error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, fmt.Errorf("muz/fp: PDR requires parameter and global services")
	}
	if e.spacer != nil {
		if err := e.spacer.Close(); err != nil {
			return lbool.Undef, err
		}
		e.spacer = nil
		e.spacerRevision = 0
		e.spacerPredicates = nil
		e.spacerSlices = nil
		e.spacerQuantified = nil
		e.spacerInstantiated = nil
		e.spacerBitBlasted = nil
		e.spacerScaled = nil
		e.spacerMagic = nil
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		valid, x := e.set.Finalize()
		if x != nil {
			return lbool.Undef, x
		}
		if !valid {
			return lbool.Undef, fmt.Errorf("muz/fp: negation is not stratified")
		}
	}
	if err = e.normalizeSimpleJoins(); err != nil {
		return lbool.Undef, err
	}
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	if e.pdr != nil && e.pdrRevision == e.revision {
		return e.pdr.Solve(declaration)
	}
	if e.pdr != nil {
		if err = e.pdr.Close(); err != nil {
			return lbool.Undef, err
		}
		e.pdr = nil
		e.pdrRevision = 0
	}
	fixedpoint, err := e.globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return lbool.Undef, err
	}
	maximumName := "pdr.max_num_contexts"
	maximumContexts, err := e.globals.Context.GetUIntName(&fixedpoint, &maximumName, 500)
	if err != nil {
		return lbool.Undef, err
	}
	cacheName := "pdr.cache_mode"
	cacheValue, err := e.globals.Context.GetUIntName(&fixedpoint, &cacheName, uint32(pdr.ReachableNoCache))
	if err != nil {
		return lbool.Undef, err
	}
	cacheMode := pdr.ReachableCacheMode(cacheValue)
	if cacheMode > pdr.ReachableConstraintCache {
		return lbool.Undef, fmt.Errorf("muz/fp: invalid pdr.cache_mode %d", cacheValue)
	}
	context, err := pdr.NewContext(e.manager, nil, maximumContexts, cacheMode, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
	if err != nil {
		return lbool.Undef, err
	}
	farkasName := "pdr.farkas"
	useFarkas, err := e.globals.Context.GetBoolName(&fixedpoint, &farkasName, true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseFarkas(useFarkas); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	inductiveName := "pdr.use_inductive_generalizer"
	useInductive, err := e.globals.Context.GetBoolName(&fixedpoint, &inductiveName, true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseInductiveGeneralizer(useInductive); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	convexClosureName := "pdr.use_convex_closure_generalizer"
	useConvexClosure, err := e.globals.Context.GetBoolName(&fixedpoint, &convexClosureName, false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseConvexClosureGeneralizer(useConvexClosure); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	convexInteriorName := "pdr.use_convex_interior_generalizer"
	useConvexInterior, err := e.globals.Context.GetBoolName(&fixedpoint, &convexInteriorName, false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseConvexInteriorGeneralizer(useConvexInterior); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	modelGeneralizerName := "pdr.use_model_generalizer"
	useModelGeneralizer, err := e.globals.Context.GetBoolName(&fixedpoint, &modelGeneralizerName, false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetUseModelGeneralizer(useModelGeneralizer); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	breadthFirstName := "pdr.bfs_model_search"
	breadthFirst, err := e.globals.Context.GetBoolName(&fixedpoint, &breadthFirstName, true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetBreadthFirstModelSearch(breadthFirst); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	flexibleName := "pdr.flexible_trace"
	flexible, err := e.globals.Context.GetBoolName(&fixedpoint, &flexibleName, false)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetFlexibleTrace(flexible); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	inlineEagerName := "xform.inline_eager"
	inlineEager, err := e.globals.Context.GetBoolName(&fixedpoint, &inlineEagerName, true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	inlineLinearName := "xform.inline_linear"
	inlineLinear, err := e.globals.Context.GetBoolName(&fixedpoint, &inlineLinearName, true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	sliceName := "xform.slice"
	sliceRules, err := e.globals.Context.GetBoolName(&fixedpoint, &sliceName, true)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetFullTraceAnswer(!inlineEager && !inlineLinear && !sliceRules); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	auxiliary := make([]ast.ASTHandle, 0, len(e.auxiliary))
	for declaration := range e.auxiliary {
		auxiliary = append(auxiliary, declaration)
	}
	if err = context.SetAuxiliaryPredicates(auxiliary); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	background, err := e.manager.RefGet(e.background)
	if err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.SetBackground(background); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	if err = context.Initialize(e.set); err != nil {
		return lbool.Undef, errors.Join(err, context.Close())
	}
	e.pdr = context
	e.pdrRevision = e.revision
	return context.Solve(declaration)
}

func (e *PropositionalEngine) pdrContext() (*pdr.Context, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.backend != "pdr" || e.pdr == nil || e.pdrRevision != e.revision {
		return nil, ErrPropositionalEngine
	}
	return e.pdr, nil
}

// NumLevels returns the finite PDR frame count for a relation after a query.
func (e *PropositionalEngine) NumLevels(declaration ast.ASTHandle) (uint32, error) {
	if err := e.open(); err != nil {
		return 0, err
	}
	if e.backend == "spacer" && e.spacer != nil && e.spacerRevision == e.revision {
		return e.spacer.NumLevels(e.spacerPredicate(declaration))
	}
	context, err := e.pdrContext()
	if err != nil {
		return 0, err
	}
	return context.NumLevels(declaration)
}

// CoverDelta returns an owning reference to a PDR or Spacer cover frame after
// a query.
func (e *PropositionalEngine) CoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if e.backend == "spacer" && e.spacer != nil && e.spacerRevision == e.revision {
		cover, err := e.spacer.CoverDelta(level, e.spacerPredicate(declaration))
		if err != nil {
			return ast.ASTRef{}, err
		}
		return e.expandSpacerFormula(declaration, cover)
	}
	context, err := e.pdrContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return context.CoverDelta(level, declaration)
}

// AddCover installs a bound-variable property in a retained PDR or Spacer
// frame.
func (e *PropositionalEngine) AddCover(level int, declaration, property ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	if e.backend == "spacer" && e.spacer != nil && e.spacerRevision == e.revision {
		if err := e.rejectSlicedProperty(declaration); err != nil {
			return err
		}
		return e.spacer.AddCover(level, e.spacerPredicate(declaration), property)
	}
	context, err := e.pdrContext()
	if err != nil {
		return err
	}
	return context.AddCover(level, declaration, property)
}

// AddInvariant installs a bound-variable property in the invariant frame.
func (e *PropositionalEngine) AddInvariant(declaration, property ast.ASTHandle) error {
	return e.AddCover(-1, declaration, property)
}

// Reachable returns the accumulated Spacer reach summary for a predicate.
func (e *PropositionalEngine) Reachable(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if e.backend != "spacer" || e.spacer == nil || e.spacerRevision != e.revision {
		return ast.ASTRef{}, ErrPropositionalEngine
	}
	result, err := e.spacer.Reachable(e.spacerPredicate(declaration))
	if err != nil {
		return ast.ASTRef{}, err
	}
	return e.expandSpacerFormula(declaration, result)
}

// GroundSatAnswer returns the latest successful Spacer reach-fact answer.
func (e *PropositionalEngine) GroundSatAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if e.backend != "spacer" || e.spacer == nil || e.spacerRevision != e.revision {
		return ast.ASTRef{}, ErrPropositionalEngine
	}
	status, err := e.spacer.LastResult()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if status != lbool.True {
		return ast.ASTRef{}, ErrPropositionalEngine
	}
	answer, err := e.spacer.Answer(e.spacerPredicate(declaration))
	if err != nil {
		return ast.ASTRef{}, err
	}
	return e.expandSpacerFormula(declaration, answer)
}

// RulesAlongTrace returns owning rule formulas and their names in preorder for
// the latest successful Spacer query. The caller releases every ASTRef.
func (e *PropositionalEngine) RulesAlongTrace() (formulas []ast.ASTRef, names []string, resultErr error) {
	if err := e.open(); err != nil {
		return nil, nil, err
	}
	if e.backend != "spacer" || e.spacer == nil || e.spacerRevision != e.revision {
		return nil, nil, ErrPropositionalEngine
	}
	rules, err := e.spacer.RulesAlongTrace()
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		for index := len(rules) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, rules[index].Close())
		}
		if resultErr != nil {
			for index := len(formulas) - 1; index >= 0; index-- {
				resultErr = errors.Join(resultErr, e.manager.DelRef(&formulas[index]))
			}
			formulas = nil
			names = nil
		}
	}()
	for _, rule := range rules {
		formula, err := rule.ToFormula()
		if err != nil {
			return formulas, names, err
		}
		name, err := rule.Name()
		if err != nil {
			return formulas, names, err
		}
		formulas = append(formulas, formula)
		names = append(names, name)
	}
	return formulas, names, resultErr
}
