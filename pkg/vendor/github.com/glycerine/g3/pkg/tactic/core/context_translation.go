package core

// Source: concrete tactic translate methods in src/tactic/core. Target-owned
// services replace source process-global parameter and evaluator services.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateContextLeaf translates the supported concrete core tactics. The
// Boolean reports whether the tactic belongs to this set.
func TranslateContextLeaf(value tactic.Tactic, m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, services model.ModelEvaluatorConfig) (result tactic.Tactic, matched bool, resultErr error) {
	switch t := value.(type) {
	case *DERTactic:
		r, err := t.Translate(m)
		return r, true, err
	case *DistributeForallTactic:
		r, err := t.Translate(m)
		return r, true, err
	case *OCCFTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		r, err := NewOCCFTactic(m, services)
		if err != nil {
			return nil, true, err
		}
		return r, true, nil
	case *PBPreprocessTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		r, err := NewPBPreprocessTactic(m, services)
		if err != nil {
			return nil, true, err
		}
		return r, true, nil
	case *CollectStatisticsTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		if !t.paramsReady {
			r, err := NewCollectStatisticsTacticWithServices(m, context, nil, t.output)
			return r, true, err
		}
	case *SplitClauseTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		r, err := NewSplitClauseTactic(m, context, nil)
		if err != nil {
			return nil, true, err
		}
		r.largestClause = t.largestClause
		return r, true, nil
	case *ReduceArgsTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		r, err := NewReduceArgsTactic(m, context, globals, services)
		if err != nil {
			return nil, true, err
		}
		return r, true, nil
	case *SymmetryReduceTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		r, err := NewSymmetryReduceTactic(m, context, globals)
		if err != nil {
			return nil, true, err
		}
		r.trace = t.trace
		return r, true, nil
	}
	var source *uparams.ParamsContext
	var sourceParams *uparams.ParamsRef
	switch t := value.(type) {
	case *PropagateValuesTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, sourceParams = t.context, &t.params
	case *SolveEquationsTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, sourceParams = t.context, &t.params
	case *EliminateUnconstrainedTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, sourceParams = t.context, &t.params
	case *EliminateTermITETactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, sourceParams = t.context, &t.params
	case *TseitinCNFTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, sourceParams = t.context, &t.params
	case *ContextSimplifyTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		if _, ok := t.current.simp.(*contextAssertions); !ok {
			return nil, false, nil
		}
		source, sourceParams = t.context, &t.params
	case *NNFTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, sourceParams = t.context, &t.params
	case *InjectivityTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, sourceParams = t.context, &t.params
	case *CollectStatisticsTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, sourceParams = t.context, &t.params
	case *BlastTermITETactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, sourceParams = t.context, &t.params
	case *CofactorTermITETactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, sourceParams = t.context, &t.params
	default:
		return nil, false, nil
	}
	p, err := source.TranslateTo(sourceParams, context)
	if err != nil {
		return nil, true, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, context.Del(&p))
		if resultErr != nil && result != nil {
			if e := result.IncRef(); e == nil {
				resultErr = errors.Join(resultErr, result.DecRef())
			} else {
				resultErr = errors.Join(resultErr, e)
			}
			result = nil
		}
	}()
	switch t := value.(type) {
	case *PropagateValuesTactic:
		var next *PropagateValuesTactic
		next, err = NewPropagateValuesTactic(m, context, globals, &p)
		if err == nil {
			next.trace = t.trace
			result = next
		}
	case *SolveEquationsTactic:
		var next *SolveEquationsTactic
		next, err = NewSolveEquationsTactic(m, context, globals, &p, services)
		if err == nil {
			next.trace = t.trace
			result = next
		}
	case *EliminateUnconstrainedTactic:
		var next *EliminateUnconstrainedTactic
		next, err = NewEliminateUnconstrainedTactic(m, context, globals, &p, services)
		if err == nil {
			next.trace = t.trace
			result = next
		}
	case *EliminateTermITETactic:
		var next *EliminateTermITETactic
		next, err = NewEliminateTermITETactic(m, context, globals, &p, services)
		if err == nil {
			result = next
		}
	case *TseitinCNFTactic:
		var next *TseitinCNFTactic
		next, err = NewTseitinCNFCoreTactic(m, context, globals, &p, services)
		if err == nil {
			next.numAux = t.numAux
			result = next
		}
	case *ContextSimplifyTactic:
		var next *ContextSimplifyTactic
		next, err = NewContextSimplifyTactic(m, context, globals, &p, services)
		if err == nil {
			next.trace = t.trace
			result = next
		}
	case *NNFTactic:
		var next *NNFTactic
		next, err = newNNFTactic(m, context, globals, &p, t.full)
		if err == nil {
			result = next
		}
	case *InjectivityTactic:
		var next *InjectivityTactic
		next, err = NewInjectivityTactic(m, context, globals, &p)
		if err == nil {
			result = next
		}
	case *CollectStatisticsTactic:
		var next *CollectStatisticsTactic
		next, err = NewCollectStatisticsTacticWithServices(m, context, &p, t.output)
		if err == nil {
			result = next
		}
	case *BlastTermITETactic:
		var next *BlastTermITETactic
		next, err = NewBlastTermITETactic(m, context, globals, &p, services)
		if err == nil {
			result = next
		}
	case *CofactorTermITETactic:
		var next *CofactorTermITETactic
		next, err = NewCofactorTermITETactic(m, context, globals, &p, services)
		if err == nil {
			result = next
		}
	}
	return result, true, err
}
