package opt

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type ObjectivePipeline struct {
	manager    *ast.ASTManager
	params     *uparams.ParamsRef
	logic      string
	config     smtcore.SMTSolverConfig
	hard       []ast.ASTRef
	objectives []pipelineObjective
	closed     bool
}

type pipelineObjective struct {
	sourceKind FormulaObjectiveKind
	term       ast.ASTRef
	soft       []ast.ASTRef
	weights    []big.Rat
	offset     big.Rat
	negate     bool
	id         string
	converted  bool
}

func (p *ObjectivePipeline) open() error {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	return nil
}

func (p *ObjectivePipeline) Close() (result error) {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	for i := len(p.objectives) - 1; i >= 0; i-- {
		o := &p.objectives[i]
		for j := len(o.soft) - 1; j >= 0; j-- {
			result = errors.Join(result, p.manager.DelRef(&o.soft[j]))
		}
		if o.term != (ast.ASTRef{}) {
			result = errors.Join(result, p.manager.DelRef(&o.term))
		}
	}
	for i := len(p.hard) - 1; i >= 0; i-- {
		result = errors.Join(result, p.manager.DelRef(&p.hard[i]))
	}
	if result == nil {
		p.hard, p.objectives, p.closed = nil, nil, true
	}
	return result
}
