package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_eqs_util::factor_selects.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type legacyFactoredPartialEquality struct {
	manager    *ast.ASTManager
	partial    *legacyPartialEquality
	variables  []ast.ASTRef
	equalities []ast.ASTRef
}

func (r *legacyFactoredPartialEquality) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.partial != nil {
		result = errors.Join(result, r.partial.Close())
		r.partial = nil
	}
	for index := len(r.equalities) - 1; index >= 0; index-- {
		result = errors.Join(result, r.manager.DelRef(&r.equalities[index]))
	}
	r.equalities = nil
	for index := len(r.variables) - 1; index >= 0; index-- {
		result = errors.Join(result, r.manager.DelRef(&r.variables[index]))
	}
	r.variables = nil
	if result == nil {
		r.manager = nil
	}
	return result
}

func (p *legacyArrayProjection) factorPartialSelects(md *model.ModelCore, evaluator *model.ModelEvaluator, variable ast.ASTHandle, partial *legacyPartialEquality) (result *legacyFactoredPartialEquality, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || evaluator == nil || partial == nil {
		return nil, ast.ErrASTHandle
	}
	result = &legacyFactoredPartialEquality{manager: p.syntax.manager}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	application, err := partial.Application()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.syntax.manager.DelRef(&application)) }()
	root, err := p.syntax.manager.RefGet(application)
	if err != nil {
		return nil, err
	}
	type rewrittenNode struct {
		handle    ast.ASTHandle
		hasStores bool
	}
	cache := make(map[ast.ASTHandle]rewrittenNode)
	var rewrite func(ast.ASTHandle) (rewrittenNode, error)
	rewrite = func(current ast.ASTHandle) (rewrittenNode, error) {
		if cached, ok := cache[current]; ok {
			return cached, nil
		}
		kind, err := p.syntax.manager.GetKind(current)
		if err != nil || kind != ast.ASTApp {
			return rewrittenNode{handle: current}, err
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return rewrittenNode{}, err
		}
		children := make([]ast.ASTHandle, count)
		argumentsHaveStores := false
		changed := false
		for index := range children {
			original, err := p.syntax.manager.GetArg(current, uint32(index))
			if err != nil {
				return rewrittenNode{}, err
			}
			child, err := rewrite(original)
			if err != nil {
				return rewrittenNode{}, err
			}
			children[index] = child.handle
			changed = changed || child.handle != original
			argumentsHaveStores = argumentsHaveStores || child.hasStores
		}
		rebuilt := current
		if changed {
			decl, err := p.syntax.manager.GetDecl(current)
			if err != nil {
				return rewrittenNode{}, err
			}
			rebuilt, err = p.syntax.manager.MkApp(decl, children)
			if err != nil {
				return rewrittenNode{}, err
			}
		}
		storeArgs, storeTerm, err := p.syntax.operation(rebuilt, p.syntax.family, ast.ASTArrayOpStore)
		if err != nil {
			return rewrittenNode{}, err
		}
		hasStores := argumentsHaveStores || (storeTerm && len(storeArgs) != 0 && storeArgs[0] == variable)
		selectArgs, selectTerm, err := p.syntax.operation(rebuilt, p.syntax.family, ast.ASTArrayOpSelect)
		if err != nil {
			return rewrittenNode{}, err
		}
		if selectTerm && len(selectArgs) >= 2 && (selectArgs[0] == variable || argumentsHaveStores) {
			sort, err := p.syntax.manager.GetSort(rebuilt)
			if err != nil {
				return rewrittenNode{}, err
			}
			fresh, err := p.syntax.manager.MkFreshConst("sel", sort)
			if err != nil {
				return rewrittenNode{}, err
			}
			freshRef, err := p.syntax.manager.NewRef(fresh)
			if err != nil {
				return rewrittenNode{}, err
			}
			result.variables = append(result.variables, freshRef)
			value, err := evaluator.EvaluateValue(rebuilt)
			if err != nil {
				return rewrittenNode{}, err
			}
			valueHandle, err := p.syntax.manager.RefGet(value)
			if err == nil {
				var decl ast.ASTHandle
				decl, err = p.syntax.manager.GetDecl(fresh)
				if err == nil {
					err = md.RegisterConstant(decl, valueHandle)
				}
			}
			err = errors.Join(err, p.syntax.manager.DelRef(&value))
			if err != nil {
				return rewrittenNode{}, err
			}
			equality, err := p.syntax.manager.MkEq(fresh, rebuilt)
			if err != nil {
				return rewrittenNode{}, err
			}
			equalityRef, err := p.syntax.manager.NewRef(equality)
			if err != nil {
				return rewrittenNode{}, err
			}
			result.equalities = append(result.equalities, equalityRef)
			node := rewrittenNode{handle: fresh}
			cache[current] = node
			return node, nil
		}
		node := rewrittenNode{handle: rebuilt, hasStores: hasStores && !selectTerm}
		cache[current] = node
		return node, nil
	}
	rewritten, err := rewrite(root)
	if err != nil {
		return nil, err
	}
	count, err := p.syntax.manager.GetNumArgs(rewritten.handle)
	if err != nil || count < 2 {
		return nil, errors.Join(err, ast.ErrASTDomain)
	}
	lhs, err := p.syntax.manager.GetArg(rewritten.handle, 0)
	if err != nil {
		return nil, err
	}
	rhs, err := p.syntax.manager.GetArg(rewritten.handle, 1)
	if err != nil {
		return nil, err
	}
	indices := make([]ast.ASTHandle, 0, count-2)
	for index := uint32(2); index < count; index++ {
		handle, err := p.syntax.manager.GetArg(rewritten.handle, index)
		if err != nil {
			return nil, err
		}
		indices = append(indices, handle)
	}
	result.partial, err = newLegacyPartialEquality(p.syntax.manager, p.syntax.family, lhs, rhs, indices)
	if err != nil {
		return nil, err
	}
	return result, nil
}
