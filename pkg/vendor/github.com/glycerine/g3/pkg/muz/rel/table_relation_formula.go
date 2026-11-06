package rel

// Source: src/muz/rel/dl_base.cpp table_base::to_formula and
// src/muz/rel/dl_table_relation.h table_relation::to_formula.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ToFormula returns an owning Boolean AST reference for the table's row set.
// Each row becomes a conjunction equating de-Bruijn variables to its values;
// the table formula is the disjunction of those row conjunctions.
func (r *TableRelation) ToFormula() (result ast.ASTRef, resultErr error) {
	if err := r.open(); err != nil {
		return ast.ASTRef{}, err
	}
	facts, err := r.table.Facts()
	if err != nil {
		return ast.ASTRef{}, err
	}
	rows := make([]ast.ASTRef, 0, len(facts))
	resultOwned := false
	defer func() {
		resultErr = errors.Join(resultErr, releaseTableFormulaRefs(r.manager.manager, rows))
		if resultErr != nil && resultOwned {
			resultErr = errors.Join(resultErr, r.manager.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	rowHandles := make([]ast.ASTHandle, 0, len(facts))
	for _, fact := range facts {
		row, rowErr := tableRelationRowFormula(r.manager, r.sorts, fact)
		if rowErr != nil {
			return ast.ASTRef{}, rowErr
		}
		rows = append(rows, row)
		handle, getErr := r.manager.manager.RefGet(row)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		rowHandles = append(rowHandles, handle)
	}
	result, err = r.manager.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	resultOwned = true
	if err = rewriter.SimplifyBooleanGateDefault(r.manager.manager, ast.ASTOpOr, rowHandles, result); err != nil {
		return result, err
	}
	return result, nil
}

func tableRelationRowFormula(relations *RelationManager, sorts []ast.ASTHandle, fact Fact) (result ast.ASTRef, resultErr error) {
	if len(sorts) != len(fact) {
		return ast.ASTRef{}, ErrTable
	}
	manager := relations.manager
	equations := make([]ast.ASTRef, 0, len(fact))
	var conjunction ast.ASTRef
	conjunctionOwned := false
	defer func() {
		resultErr = errors.Join(resultErr, releaseTableFormulaRefs(manager, equations))
		if resultErr != nil && conjunctionOwned {
			resultErr = errors.Join(resultErr, manager.DelRef(&conjunction))
			result = ast.ASTRef{}
		}
	}()
	equationHandles := make([]ast.ASTHandle, 0, len(fact))
	for index, value := range fact {
		variable, err := manager.MkVar(uint32(index), sorts[index])
		if err != nil {
			return ast.ASTRef{}, err
		}
		variableRef, err := manager.NewRef(variable)
		if err != nil {
			return ast.ASTRef{}, err
		}
		numeralRef, err := relations.DecodeValue(sorts[index], value)
		if err != nil {
			return ast.ASTRef{}, errors.Join(err, manager.DelRef(&variableRef))
		}
		variableHandle, err := manager.RefGet(variableRef)
		if err == nil {
			var numeral ast.ASTHandle
			numeral, err = manager.RefGet(numeralRef)
			if err == nil {
				var equation ast.ASTHandle
				equation, err = manager.MkEq(variableHandle, numeral)
				if err == nil {
					var equationRef ast.ASTRef
					equationRef, err = manager.NewRef(equation)
					if err == nil {
						equations = append(equations, equationRef)
						equationHandle, getErr := manager.RefGet(equationRef)
						if getErr != nil {
							err = getErr
						} else {
							equationHandles = append(equationHandles, equationHandle)
						}
					}
				}
			}
		}
		err = errors.Join(err, manager.DelRef(&numeralRef), manager.DelRef(&variableRef))
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	conjunction, err := manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	conjunctionOwned = true
	if err = rewriter.SimplifyBooleanGateDefault(manager, ast.ASTOpAnd, equationHandles, conjunction); err != nil {
		return conjunction, err
	}
	return conjunction, nil
}

func releaseTableFormulaRefs(manager *ast.ASTManager, refs []ast.ASTRef) (result error) {
	for i := len(refs) - 1; i >= 0; i-- {
		result = errors.Join(result, manager.DelRef(&refs[i]))
	}
	return result
}
