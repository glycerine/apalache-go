package opt

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func NewObjectivePipeline(manager *ast.ASTManager, params *uparams.ParamsRef, logic string, config smtcore.SMTSolverConfig, hard []ast.ASTHandle, objectives []DecomposedObjective) (pipeline *ObjectivePipeline, resultErr error) {
	if manager == nil || params == nil || len(objectives) == 0 {
		return nil, ast.ErrASTHandle
	}
	pipeline = &ObjectivePipeline{manager: manager, params: params, logic: logic, config: config}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, pipeline.Close())
			pipeline = nil
		}
	}()
	for _, formula := range hard {
		ref, err := manager.NewRef(formula)
		if err != nil {
			return nil, err
		}
		pipeline.hard = append(pipeline.hard, ref)
	}
	for i := range objectives {
		source := &objectives[i]
		target := pipelineObjective{sourceKind: source.SourceKind, weights: cloneRationals(source.Weights), offset: *new(big.Rat).Set(&source.Offset), negate: source.Negate, id: source.ID, converted: source.Converted}
		if len(source.SoftTerms) != len(source.Weights) {
			return nil, ast.ErrASTDomain
		}
		if len(source.SoftTerms) != 0 || source.Converted && source.Term.IsNull() {
			for _, term := range source.SoftTerms {
				ref, err := manager.NewRef(term)
				if err != nil {
					return nil, err
				}
				target.soft = append(target.soft, ref)
			}
		} else {
			if source.Term.IsNull() {
				return nil, ast.ErrASTDomain
			}
			ref, err := manager.NewRef(source.Term)
			if err != nil {
				return nil, err
			}
			target.term = ref
		}
		pipeline.objectives = append(pipeline.objectives, target)
	}
	return pipeline, nil
}
