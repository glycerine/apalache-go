package fpa

// Source: src/tactic/fpa/fpa2bv_tactic.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astfpa "github.com/glycerine/g3/pkg/ast/fpa"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Result is the source tactic's single output goal and optional model
// converter. The caller owns the goal reference and converter.
type Result struct {
	Goal           *tactic.Goal
	ModelConverter tactic.ModelConverter
}

type Pass struct {
	self      *Pass
	manager   *ast.ASTManager
	context   *uparams.ParamsContext
	globals   *uparams.GlobalParamsService
	params    uparams.ParamsRef
	converter *astfpa.Converter
	rewriter  *rewriter.FPA2BVRewriter
	closed    bool
}

func NewPass(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, value *uparams.ParamsRef) (*Pass, error) {
	if m == nil || context == nil || globals == nil || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	p := &Pass{manager: m, context: context, globals: globals}
	p.self = p
	var err error
	p.params, err = context.NewValue()
	if err != nil {
		return nil, err
	}
	if value != nil {
		if err = context.Assign(&p.params, value); err != nil {
			return nil, errors.Join(err, p.Close())
		}
	}
	if err = p.build(); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	return p, nil
}

func (p *Pass) open() error {
	if p == nil || p.self != p || p.closed || p.manager == nil || p.converter == nil || p.rewriter == nil {
		return ast.ErrASTHandle
	}
	return p.context.Check(&p.params)
}

func (p *Pass) build() error {
	converter, err := astfpa.NewConverter(p.manager)
	if err != nil {
		return err
	}
	p.converter = converter
	rewrite, err := rewriter.NewFPA2BVRewriter(p.manager, converter)
	if err != nil {
		return err
	}
	p.rewriter = rewrite
	settings, err := astfpa.NewFPA2BVRewriterParams(p.context, p.globals, &p.params)
	if err != nil {
		return err
	}
	high, err := settings.HighFPUnspecified()
	err = errors.Join(err, settings.Close())
	if err != nil {
		return err
	}
	return rewrite.SetHighUnspecified(high)
}

func (p *Pass) UpdateParams(value *uparams.ParamsRef) error {
	if err := p.open(); err != nil {
		return err
	}
	if value == nil {
		return uparams.ErrParamsOwner
	}
	if err := p.context.Assign(&p.params, value); err != nil {
		return err
	}
	settings, err := astfpa.NewFPA2BVRewriterParams(p.context, p.globals, &p.params)
	if err != nil {
		return err
	}
	high, err := settings.HighFPUnspecified()
	err = errors.Join(err, settings.Close())
	if err != nil {
		return err
	}
	return p.rewriter.SetHighUnspecified(high)
}

func (p *Pass) Translate(manager *ast.ASTManager) (*Pass, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	return NewPass(manager, p.context, p.globals, &p.params)
}

func (p *Pass) Apply(goal *tactic.Goal) (_ Result, result error) {
	if err := p.open(); err != nil {
		return Result{}, err
	}
	if goal == nil || goal.Manager() != p.manager {
		return Result{}, ast.ErrASTHandle
	}
	wellSorted, err := goal.IsWellSorted()
	if err != nil || !wellSorted {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return Result{}, err
	}
	report, err := tactic.NewTacticReport("fpa2bv", goal)
	if err != nil {
		return Result{}, err
	}
	defer func() { result = errors.Join(result, report.Close()) }()
	if err = p.rewriter.Reset(); err != nil {
		return Result{}, err
	}
	inconsistent, err := goal.Inconsistent()
	if err != nil {
		return Result{}, err
	}
	if inconsistent {
		if err = goal.IncRef(); err != nil {
			return Result{}, err
		}
		return Result{Goal: goal}, nil
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return Result{}, err
	}
	size, err := goal.Size()
	if err != nil {
		return Result{}, err
	}
	for i := uint32(0); i < size; i++ {
		inconsistent, err = goal.Inconsistent()
		if err != nil || inconsistent {
			if err != nil {
				return Result{}, err
			}
			break
		}
		current, err := goal.Form(i)
		if err != nil {
			return Result{}, err
		}
		out, err := p.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return Result{}, err
		}
		proof, err := p.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return Result{}, errors.Join(err, p.manager.DelRef(&out))
		}
		if proofs {
			err = p.rewriter.RewriteWithProof(current, out, proof)
		} else {
			err = p.rewriter.Rewrite(current, out)
		}
		if err != nil {
			return Result{}, errors.Join(err, p.manager.DelRef(&proof), p.manager.DelRef(&out))
		}
		rewritten, err := p.manager.RefGet(out)
		if err != nil {
			return Result{}, errors.Join(err, p.manager.DelRef(&proof), p.manager.DelRef(&out))
		}
		newProof, err := p.manager.RefGet(proof)
		if err != nil {
			return Result{}, errors.Join(err, p.manager.DelRef(&proof), p.manager.DelRef(&out))
		}
		if proofs {
			oldProof, e := goal.Proof(i)
			if e == nil {
				newProof, e = p.manager.MkProofModusPonens(oldProof, newProof)
			}
			if e != nil {
				return Result{}, errors.Join(e, p.manager.DelRef(&proof), p.manager.DelRef(&out))
			}
		}
		dependency, err := goal.Dependency(i)
		if err == nil {
			err = goal.Update(i, rewritten, newProof, dependency)
		}
		err = errors.Join(err, p.manager.DelRef(&proof), p.manager.DelRef(&out))
		if err != nil {
			return Result{}, err
		}
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return Result{}, err
	}
	var modelConverter tactic.ModelConverter
	if models {
		modelConverter, err = tactic.NewFPA2BVModelConverter(p.manager, p.converter)
		if err != nil {
			return Result{}, err
		}
	}
	failed := true
	defer func() {
		if failed && modelConverter != nil {
			result = errors.Join(result, modelConverter.Close())
		}
	}()
	if err = goal.IncDepth(); err != nil {
		return Result{}, err
	}
	for _, assertion := range p.converter.ExtraAssertions() {
		if err = goal.Assert(assertion); err != nil {
			return Result{}, err
		}
	}
	wellSorted, err = goal.IsWellSorted()
	if err != nil || !wellSorted {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return Result{}, err
	}
	if err = goal.IncRef(); err != nil {
		return Result{}, err
	}
	failed = false
	return Result{Goal: goal, ModelConverter: modelConverter}, nil
}

func (p *Pass) Cleanup() error {
	if err := p.open(); err != nil {
		return err
	}
	if err := p.rewriter.Close(); err != nil {
		return err
	}
	p.rewriter = nil
	if err := p.converter.Close(); err != nil {
		return err
	}
	p.converter = nil
	return p.build()
}

func (p *Pass) Close() (result error) {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.rewriter != nil {
		result = errors.Join(result, p.rewriter.Close())
		if result != nil {
			return result
		}
		p.rewriter = nil
	}
	if p.converter != nil {
		result = errors.Join(result, p.converter.Close())
		if result != nil {
			return result
		}
		p.converter = nil
	}
	result = p.context.Del(&p.params)
	if result == nil {
		p.closed = true
	}
	return result
}
