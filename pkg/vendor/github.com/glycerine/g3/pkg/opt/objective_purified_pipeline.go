package opt

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type PurifiedObjectivePipeline struct {
	purifier *ObjectivePurifier
	pipeline *ObjectivePipeline
}

func NewPurifiedObjectivePipeline(manager *ast.ASTManager, params *uparams.ParamsRef, logic string, config smtcore.SMTSolverConfig, hard []ast.ASTHandle, objectives []DecomposedObjective) (result *PurifiedObjectivePipeline, resultErr error) {
	purifier, err := NewObjectivePurifier(manager)
	if err != nil {
		return nil, err
	}
	result = &PurifiedObjectivePipeline{purifier: purifier}
	owner := result
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, owner.Close())
			result = nil
		}
	}()
	set, err := purifier.Purify(hard, objectives)
	if err != nil {
		return nil, err
	}
	result.pipeline, err = NewObjectivePipeline(manager, params, logic, config, set.Hard, set.Objectives)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (p *PurifiedObjectivePipeline) Statistics() (ObjectivePurificationStats, error) {
	if p == nil || p.purifier == nil {
		return ObjectivePurificationStats{}, ast.ErrASTHandle
	}
	return p.purifier.Statistics()
}

func (p *PurifiedObjectivePipeline) Check() (*ObjectivePipelineResult, error) {
	if p == nil || p.pipeline == nil || p.purifier == nil {
		return nil, ast.ErrASTHandle
	}
	result, err := p.pipeline.Check()
	if err != nil || result.Model == nil {
		return result, err
	}
	filtered, err := p.purifier.FilterModel(result.Model)
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if err = result.Model.DecRef(); err != nil {
		return nil, errors.Join(err, filtered.DecRef(), result.Close())
	}
	result.Model = filtered
	return result, nil
}

func (p *PurifiedObjectivePipeline) Close() error {
	if p == nil {
		return ast.ErrASTHandle
	}
	var result error
	if p.pipeline != nil {
		result = p.pipeline.Close()
		if result == nil {
			p.pipeline = nil
		}
	}
	if result == nil && p.purifier != nil {
		err := p.purifier.Close()
		result = errors.Join(result, err)
		if err == nil {
			p.purifier = nil
		}
	}
	return result
}
