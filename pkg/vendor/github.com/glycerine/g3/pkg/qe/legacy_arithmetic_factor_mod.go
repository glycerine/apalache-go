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
	// Refuse cleanup before discarding any roots when manager entry is pending.
	if _, err := r.manager.ProofMode(); err != nil {
		return err
	}
	for len(r.variables) != 0 {
		index := len(r.variables) - 1
		if err := r.manager.DelRef(&r.variables[index]); err != nil {
			return err
		}
		r.variables = r.variables[:index]
	}
	if r.formula != (ast.ASTRef{}) {
		if err := r.manager.DelRef(&r.formula); err != nil {
			return err
		}
	}
	r.manager = nil
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
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{
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
	definitions := make([]ast.ASTRef, 0)
	defer func() { resultErr = errors.Join(resultErr, truncateLiteVector(p.syntax.manager, &definitions, 0)) }()
	cachedRefs := make([]ast.ASTRef, 0)
	defer func() { resultErr = errors.Join(resultErr, truncateLiteVector(p.syntax.manager, &cachedRefs, 0)) }()
	rewriteApp := func(current ast.ASTHandle, children []ast.ASTHandle) (ast.ASTHandle, error) {
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
		definition, err := p.syntax.manager.MkEq(fresh, rebuilt)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		definitionRef, err := p.syntax.manager.NewRef(definition)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		definitions = append(definitions, definitionRef)

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
		memo[current] = fresh
		return fresh, nil
	}
	rewritten := formula
	rootKind, err := p.syntax.manager.GetKind(formula)
	if err != nil {
		return nil, err
	}
	var todo []ast.ASTHandle
	if rootKind == ast.ASTApp {
		todo = append(todo, formula)
	}
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		if _, done := memo[current]; done {
			todo = todo[:len(todo)-1]
			continue
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return nil, err
		}
		children := make([]ast.ASTHandle, 0, count)
		allDone := true
		for index := uint32(0); index < count; index++ {
			original, err := p.syntax.manager.GetArg(current, index)
			if err != nil {
				return nil, err
			}
			kind, err := p.syntax.manager.GetKind(original)
			if err != nil {
				return nil, err
			}
			if child, done := memo[original]; done {
				children = append(children, child)
			} else if kind != ast.ASTApp {
				children = append(children, original)
			} else {
				todo = append(todo, original)
				allDone = false
			}
		}
		if !allDone {
			continue
		}
		next, err := rewriteApp(current, children)
		if err != nil {
			return nil, err
		}
		if next != current {
			ref, err := p.syntax.manager.NewRef(next)
			if err != nil {
				return nil, err
			}
			cachedRefs = append(cachedRefs, ref)
		}
		todo = todo[:len(todo)-1]
	}
	if replacement, found := memo[formula]; found {
		rewritten = replacement
	}
	if len(definitions) != 0 {
		handles := make([]ast.ASTHandle, 0, len(definitions))
		for _, ref := range definitions {
			handle, err := p.syntax.manager.RefGet(ref)
			if err != nil {
				return nil, err
			}
			handles = append(handles, handle)
		}
		conjunction, err := p.syntax.manager.MkAndArray(handles)
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
