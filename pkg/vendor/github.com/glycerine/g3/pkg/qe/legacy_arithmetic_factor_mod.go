package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::factor_mod_terms.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type legacyFactoredMods struct {
	manager   *ast.ASTManager
	formula   ast.ASTRef
	variables []ast.ASTRef
}

func (r *legacyFactoredMods) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	for index := len(r.variables) - 1; index >= 0; index-- {
		result = errors.Join(result, r.manager.DelRef(&r.variables[index]))
	}
	r.variables = nil
	if r.formula != (ast.ASTRef{}) {
		result = errors.Join(result, r.manager.DelRef(&r.formula))
	}
	if result == nil {
		r.manager = nil
	}
	return result
}

func (p *legacyArithmeticProjection) factorModTerms(md *model.ModelCore, formula ast.ASTHandle) (_ *legacyFactoredMods, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, ast.ErrASTHandle
	}
	result := &legacyFactoredMods{manager: p.syntax.manager}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	evaluator, err := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.syntax.config.Context, Globals: p.syntax.config.Globals,
		Memory: p.syntax.manager.NumeralService().RuntimeOwner(), Cooperation: p.syntax.config.Cooperation, Trace: p.syntax.config.Trace,
	})
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return nil, err
	}
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	definitions := make([]ast.ASTHandle, 0)
	var rewrite func(ast.ASTHandle) (ast.ASTHandle, error)
	rewrite = func(current ast.ASTHandle) (ast.ASTHandle, error) {
		if cached, ok := memo[current]; ok {
			return cached, nil
		}
		kind, err := p.syntax.manager.GetKind(current)
		if err != nil || kind != ast.ASTApp {
			return current, err
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		children := make([]ast.ASTHandle, count)
		for index := range children {
			child, err := p.syntax.manager.GetArg(current, uint32(index))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			children[index], err = rewrite(child)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		decl, err := p.syntax.manager.GetDecl(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		rebuilt, err := p.syntax.manager.MkApp(decl, children)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		_, modular, err := p.syntax.operation(current, p.syntax.family, 13)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !modular {
			memo[current] = rebuilt
			return rebuilt, nil
		}
		sort, err := p.syntax.manager.GetSort(rebuilt)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		fresh, err := p.syntax.manager.MkFreshConst("mod_var", sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		freshRef, err := p.syntax.manager.NewRef(fresh)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		result.variables = append(result.variables, freshRef)
		value, err := evaluator.EvaluateValue(rebuilt)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		valueHandle, err := p.syntax.manager.RefGet(value)
		if err == nil {
			var freshDecl ast.ASTHandle
			freshDecl, err = p.syntax.manager.GetDecl(fresh)
			if err == nil {
				err = md.RegisterConstant(freshDecl, valueHandle)
			}
		}
		err = errors.Join(err, p.syntax.manager.DelRef(&value))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		definition, err := p.syntax.manager.MkEq(fresh, rebuilt)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		definitions = append(definitions, definition)
		memo[current] = fresh
		return fresh, nil
	}
	rewritten, err := rewrite(formula)
	if err != nil {
		return nil, err
	}
	if len(definitions) != 0 {
		conjunction, err := p.syntax.manager.MkAndArray(definitions)
		if err != nil {
			return nil, err
		}
		rewritten, err = p.syntax.manager.MkAnd(rewritten, conjunction)
		if err != nil {
			return nil, err
		}
	}
	result.formula, err = p.syntax.manager.NewRef(rewritten)
	if err != nil {
		return nil, err
	}
	return result, nil
}
