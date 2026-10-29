package qe

// Source: src/muz/spacer/spacer_qe_project.cpp array_project.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type legacyArrayProjectionResult struct {
	manager   *ast.ASTManager
	formula   ast.ASTRef
	variables []ast.ASTRef
}

func (r *legacyArrayProjectionResult) Close() (result error) {
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

func (p *legacyArrayProjection) projectFormula(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle, reduceAllSelects bool) (result *legacyArrayProjectionResult, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, ast.ErrASTHandle
	}
	result = &legacyArrayProjectionResult{manager: p.syntax.manager}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	current, err := p.syntax.manager.NewRef(formula)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.syntax.manager.DelRef(&current)) }()
	remaining := make([]ast.ASTHandle, 0, len(variables))
	for _, variable := range variables {
		currentHandle, err := p.syntax.manager.RefGet(current)
		if err != nil {
			return nil, err
		}
		direct, eliminated, err := p.projectDirectEquality(md, variable, currentHandle)
		if err != nil {
			return nil, err
		}
		if eliminated {
			next, err := p.syntax.manager.RefGet(direct.formula)
			if err == nil {
				err = p.syntax.manager.SetRef(current, next)
			}
			err = errors.Join(err, direct.Close())
			if err != nil {
				return nil, err
			}
			continue
		}
		currentHandle, err = p.syntax.manager.RefGet(current)
		if err != nil {
			return nil, err
		}
		store, eliminated, err := p.projectStoreEquality(md, variable, currentHandle)
		if err != nil {
			return nil, err
		}
		if eliminated {
			next, err := p.syntax.manager.RefGet(store.formula)
			if err == nil {
				err = p.syntax.manager.SetRef(current, next)
			}
			for _, auxiliary := range store.variables {
				if err != nil {
					break
				}
				handle, e := p.syntax.manager.RefGet(auxiliary)
				if e == nil {
					var copy ast.ASTRef
					copy, e = p.syntax.manager.NewRef(handle)
					if e == nil {
						result.variables = append(result.variables, copy)
					}
				}
				err = e
			}
			err = errors.Join(err, store.Close())
			if err != nil {
				return nil, err
			}
			continue
		}
		currentHandle, err = p.syntax.manager.RefGet(current)
		if err != nil {
			return nil, err
		}
		polarized, err := p.replaceEqualityPolarities(md, variable, currentHandle)
		if err != nil {
			return nil, err
		}
		next, err := p.syntax.manager.RefGet(polarized.formula)
		if err == nil {
			err = p.syntax.manager.SetRef(current, next)
		}
		err = errors.Join(err, polarized.Close())
		if err != nil {
			return nil, err
		}
		remaining = append(remaining, variable)
	}
	currentHandle, err := p.syntax.manager.RefGet(current)
	if err != nil {
		return nil, err
	}
	reduced, err := p.reduceSelects(md, remaining, currentHandle, reduceAllSelects)
	if err != nil {
		return nil, err
	}
	next, err := p.syntax.manager.RefGet(reduced.formula)
	if err == nil {
		err = p.syntax.manager.SetRef(current, next)
	}
	err = errors.Join(err, reduced.Close())
	if err != nil {
		return nil, err
	}
	currentHandle, err = p.syntax.manager.RefGet(current)
	if err != nil {
		return nil, err
	}
	selects, err := p.projectSelects(md, remaining, currentHandle)
	if err != nil {
		return nil, err
	}
	next, err = p.syntax.manager.RefGet(selects.formula)
	if err == nil {
		result.formula, err = p.syntax.manager.NewRef(next)
	}
	for _, auxiliary := range selects.variables {
		if err != nil {
			break
		}
		handle, e := p.syntax.manager.RefGet(auxiliary)
		if e == nil {
			var copy ast.ASTRef
			copy, e = p.syntax.manager.NewRef(handle)
			if e == nil {
				result.variables = append(result.variables, copy)
			}
		}
		err = e
	}
	err = errors.Join(err, selects.Close())
	if err != nil {
		return nil, err
	}
	return result, nil
}
