package qe

// Sources: src/muz/spacer/spacer_legacy_mbp.cpp;
// src/muz/spacer/spacer_util.cpp qe_project legacy path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

type LegacyProjection struct {
	manager    *ast.ASTManager
	config     Config
	arithmetic *legacyArithmeticProjection
	array      *legacyArrayProjection
}

type LegacyProjectionResult struct {
	manager   *ast.ASTManager
	formula   ast.ASTRef
	variables []ast.ASTRef
}

func NewLegacyProjection(manager *ast.ASTManager, config Config) (_ *LegacyProjection, resultErr error) {
	arithmetic, err := newLegacyArithmeticProjection(manager, config)
	if err != nil {
		return nil, err
	}
	array, err := newLegacyArrayProjection(manager, config)
	if err != nil {
		return nil, errors.Join(err, arithmetic.Close())
	}
	return &LegacyProjection{manager: manager, config: config, arithmetic: arithmetic, array: array}, nil
}

func (p *LegacyProjection) Close() error {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	err := p.array.Close()
	if err == nil {
		p.array = nil
		err = p.arithmetic.Close()
	}
	if err == nil {
		p.arithmetic = nil
		p.manager = nil
	}
	return err
}

func (r *LegacyProjectionResult) Formula() (ast.ASTHandle, error) {
	if r == nil || r.manager == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return r.manager.RefGet(r.formula)
}

func (r *LegacyProjectionResult) Variables() ([]ast.ASTHandle, error) {
	if r == nil || r.manager == nil {
		return nil, ast.ErrASTHandle
	}
	result := make([]ast.ASTHandle, len(r.variables))
	for index := range r.variables {
		var err error
		result[index], err = r.manager.RefGet(r.variables[index])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *LegacyProjectionResult) Close() (result error) {
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

func (p *LegacyProjection) substituteModelValues(md *model.ModelCore, formula ast.ASTHandle, variables []ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	if len(variables) == 0 {
		return p.manager.NewRef(formula)
	}
	evaluator, err := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.config.Context, Globals: p.config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace,
	})
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return ast.ASTRef{}, err
	}
	replace, err := rewriter.NewExprSafeReplace(p.manager)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, replace.Close()) }()
	values := make([]ast.ASTRef, 0, len(variables))
	defer func() {
		for index := len(values) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&values[index]))
		}
	}()
	for _, variable := range variables {
		value, err := evaluator.EvaluateValue(variable)
		if err != nil {
			return ast.ASTRef{}, err
		}
		values = append(values, value)
		handle, err := p.manager.RefGet(value)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if err = replace.Insert(variable, handle); err != nil {
			return ast.ASTRef{}, err
		}
	}
	result, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = replace.Apply(formula, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, p.manager.DelRef(&result))
	}
	return result, nil
}

func (p *LegacyProjection) Project(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle, reduceAllSelects, retainFailures bool) (result *LegacyProjectionResult, resultErr error) {
	if p == nil || p.manager == nil || md == nil || md.Manager() != p.manager {
		return nil, ast.ErrASTHandle
	}
	result = &LegacyProjectionResult{manager: p.manager}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	current, err := p.manager.NewRef(formula)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&current)) }()
	variableRefs := make([]ast.ASTRef, 0, len(variables))
	defer func() { resultErr = errors.Join(resultErr, truncateLiteVector(p.manager, &variableRefs, 0)) }()
	for _, variable := range variables {
		variableRef, err := p.manager.NewRef(variable)
		if err != nil {
			return nil, err
		}
		variableRefs = append(variableRefs, variableRef)
	}
	lite, err := NewLite(p.manager, nil, false, p.config)
	if err != nil {
		return nil, err
	}
	if err = lite.EliminateConstants(&variableRefs, current); err != nil {
		return nil, errors.Join(err, lite.Close())
	}
	pending := make([]ast.ASTHandle, len(variableRefs))
	for index := range variableRefs {
		pending[index], err = p.manager.RefGet(variableRefs[index])
		if err != nil {
			return nil, errors.Join(err, lite.Close())
		}
	}
	if err = lite.Close(); err != nil {
		return nil, err
	}
	ownedVariables := make([]ast.ASTRef, 0)
	defer func() {
		for index := len(ownedVariables) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&ownedVariables[index]))
		}
	}()
	arithmetic := make([]ast.ASTHandle, 0)
	booleanSort, err := p.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	for len(pending) != 0 {
		booleans := make([]ast.ASTHandle, 0)
		arrays := make([]ast.ASTHandle, 0)
		for _, variable := range pending {
			sort, err := p.manager.GetSort(variable)
			if err != nil {
				return nil, err
			}
			if sort == booleanSort {
				booleans = append(booleans, variable)
				continue
			}
			family, err := p.manager.GetFamilyID(sort)
			if err != nil {
				return nil, err
			}
			if family == p.array.syntax.family {
				arrays = append(arrays, variable)
			} else {
				arithmetic = append(arithmetic, variable)
			}
		}
		if len(booleans) != 0 {
			handle, err := p.manager.RefGet(current)
			if err != nil {
				return nil, err
			}
			next, err := p.substituteModelValues(md, handle, booleans)
			if err != nil {
				return nil, err
			}
			if err = p.manager.DelRef(&current); err != nil {
				_ = p.manager.DelRef(&next)
				return nil, err
			}
			current = next
		}
		if len(arrays) == 0 {
			break
		}
		handle, err := p.manager.RefGet(current)
		if err != nil {
			return nil, err
		}
		projected, err := p.array.projectFormula(md, arrays, handle, reduceAllSelects)
		if err != nil {
			return nil, err
		}
		nextHandle, err := p.manager.RefGet(projected.formula)
		if err == nil {
			err = p.manager.SetRef(current, nextHandle)
		}
		pending = pending[:0]
		for _, variableRef := range projected.variables {
			if err != nil {
				break
			}
			var handle ast.ASTHandle
			handle, err = p.manager.RefGet(variableRef)
			if err == nil {
				var owned ast.ASTRef
				owned, err = p.manager.NewRef(handle)
				if err == nil {
					ownedVariables = append(ownedVariables, owned)
				}
			}
			if err == nil {
				pending = append(pending, handle)
			}
		}
		err = errors.Join(err, projected.Close())
		if err != nil {
			return nil, err
		}
	}
	if len(arithmetic) != 0 {
		handle, err := p.manager.RefGet(current)
		if err != nil {
			return nil, err
		}
		projected, err := p.arithmetic.projectFormula(md, arithmetic, handle)
		if err != nil {
			return nil, err
		}
		nextHandle, err := p.manager.RefGet(projected.formula)
		if err == nil {
			err = p.manager.SetRef(current, nextHandle)
		}
		remaining := make([]ast.ASTHandle, 0, len(projected.variables))
		for _, variableRef := range projected.variables {
			if err != nil {
				break
			}
			var handle ast.ASTHandle
			handle, err = p.manager.RefGet(variableRef)
			if err == nil {
				remaining = append(remaining, handle)
			}
		}
		err = errors.Join(err, projected.Close())
		if err != nil {
			return nil, err
		}
		if len(remaining) != 0 && !retainFailures {
			handle, err := p.manager.RefGet(current)
			if err != nil {
				return nil, err
			}
			next, err := p.substituteModelValues(md, handle, remaining)
			if err != nil {
				return nil, err
			}
			if err = p.manager.DelRef(&current); err != nil {
				_ = p.manager.DelRef(&next)
				return nil, err
			}
			current = next
		} else {
			for _, variable := range remaining {
				variableRef, err := p.manager.NewRef(variable)
				if err != nil {
					return nil, err
				}
				result.variables = append(result.variables, variableRef)
			}
		}
	}
	handle, err := p.manager.RefGet(current)
	if err != nil {
		return nil, err
	}
	result.formula, err = p.arithmetic.rewriteLegacy(handle)
	if err != nil {
		return nil, err
	}
	return result, nil
}
