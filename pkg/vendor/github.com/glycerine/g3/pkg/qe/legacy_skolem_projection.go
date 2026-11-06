package qe

// Source: src/muz/spacer/spacer_legacy_mbp.cpp expr_map overload.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// ProjectSkolem performs the source QE-lite, Boolean substitution and mapped
// arithmetic projection pipeline. Mapping is borrowed and intentionally does
// not receive Boolean substitutions.
func (p *LegacyProjection) ProjectSkolem(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle, mapping *ast.ASTExprMap) (result *LegacyProjectionResult, resultErr error) {
	if p == nil || p.closing || p.manager == nil || md == nil || md.Manager() != p.manager || mapping == nil {
		return nil, ast.ErrASTHandle
	}
	result = &LegacyProjectionResult{manager: p.manager}
	ownedResult := result
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, ownedResult.Close())
			result = nil
		}
	}()
	current, err := p.manager.NewRef(formula)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&current)) }()
	vars, err := apiProjectionRefs(p.manager, variables)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, truncateLiteVector(p.manager, &vars, 0)) }()
	lite, err := NewLite(p.manager, nil, true, p.config)
	if err != nil {
		return nil, err
	}
	if err = lite.EliminateConstants(&vars, current); err != nil {
		return nil, errors.Join(err, lite.Close())
	}
	if err = lite.Close(); err != nil {
		return nil, err
	}
	booleanSort, err := p.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	booleans, arithmetic := make([]ast.ASTHandle, 0), make([]ast.ASTHandle, 0)
	for _, ref := range vars {
		variable, x := p.manager.RefGet(ref)
		if x != nil {
			return nil, x
		}
		sort, x := p.manager.GetSort(variable)
		if x != nil {
			return nil, x
		}
		if sort == booleanSort {
			booleans = append(booleans, variable)
		} else {
			arithmetic = append(arithmetic, variable)
		}
	}
	if len(booleans) != 0 {
		next, x := p.substituteSkolemBooleans(md, current, booleans)
		if x != nil {
			return nil, x
		}
		if x = p.manager.DelRef(&current); x != nil {
			_ = p.manager.DelRef(&next)
			return nil, x
		}
		current = next
	}
	if len(arithmetic) != 0 {
		body, x := p.manager.RefGet(current)
		if x != nil {
			return nil, x
		}
		projected, x := p.arithmetic.projectFormulaMapped(md, arithmetic, body, mapping)
		if x != nil {
			return nil, x
		}
		nextBody, x := p.manager.RefGet(projected.formula)
		if x == nil {
			x = p.manager.SetRef(current, nextBody)
		}
		remaining := make([]ast.ASTHandle, 0, len(projected.variables))
		for _, variableRef := range projected.variables {
			variable, y := p.manager.RefGet(variableRef)
			if x == nil {
				x = y
			}
			if x == nil {
				remaining = append(remaining, variable)
			}
		}
		for _, variable := range remaining {
			if x != nil {
				break
			}
			var ref ast.ASTRef
			ref, x = p.manager.NewRef(variable)
			if x == nil {
				result.variables = append(result.variables, ref)
			}
		}
		x = errors.Join(x, projected.Close())
		if x != nil {
			return nil, x
		}
	}
	body, err := p.manager.RefGet(current)
	if err != nil {
		return nil, err
	}
	result.formula, err = p.manager.NewRef(body)
	return result, err
}

func apiProjectionRefs(manager *ast.ASTManager, values []ast.ASTHandle) (result []ast.ASTRef, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, truncateLiteVector(manager, &result, 0))
		}
	}()
	for _, value := range values {
		ref, err := manager.NewRef(value)
		if err != nil {
			return nil, err
		}
		result = append(result, ref)
	}
	return result, nil
}
