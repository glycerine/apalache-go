package rel

// Source: src/muz/rel/dl_sieve_relation.cpp
// sieve_relation::to_formula.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ToFormula returns an owning Boolean formula for the sieve relation. Inner
// facts constrain the selected outer columns; ignored columns remain free.
func (r *SieveRelation) ToFormula(relations *RelationManager, sorts []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if err := r.open(); err != nil {
		return result, err
	}
	if relations == nil || relations.open() != nil || len(sorts) != r.signature.Len() {
		return result, ErrSieve
	}
	outerSignature, err := tableRelationSignature(relations, sorts)
	if err != nil {
		return result, err
	}
	if !outerSignature.equal(r.signature) {
		return result, ErrSieve
	}
	facts, err := r.inner.Facts()
	if err != nil {
		return result, err
	}
	manager := relations.manager
	rows := make([]ast.ASTRef, 0, len(facts))
	resultOwned := false
	defer func() {
		resultErr = errors.Join(resultErr, releaseTableFormulaRefs(manager, rows))
		if resultErr != nil && resultOwned {
			resultErr = errors.Join(resultErr, manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	rowHandles := make([]ast.ASTHandle, 0, len(facts))
	for _, fact := range facts {
		row, rowErr := r.formulaRow(relations, sorts, fact)
		if rowErr != nil {
			return result, rowErr
		}
		rows = append(rows, row)
		handle, getErr := manager.RefGet(row)
		if getErr != nil {
			return result, getErr
		}
		rowHandles = append(rowHandles, handle)
	}
	result, err = manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	resultOwned = true
	if err = rewriter.SimplifyBooleanGateDefault(manager, ast.ASTOpOr, rowHandles, result); err != nil {
		return result, err
	}
	return result, nil
}

func (r *SieveRelation) formulaRow(relations *RelationManager, sorts []ast.ASTHandle, fact Fact) (result ast.ASTRef, resultErr error) {
	if len(fact) != len(r.innerToSig) {
		return result, ErrSieve
	}
	manager := relations.manager
	equations := make([]ast.ASTRef, 0, len(fact))
	resultOwned := false
	defer func() {
		resultErr = errors.Join(resultErr, releaseTableFormulaRefs(manager, equations))
		if resultErr != nil && resultOwned {
			resultErr = errors.Join(resultErr, manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	equationHandles := make([]ast.ASTHandle, 0, len(fact))
	for innerColumn, outerColumn := range r.innerToSig {
		variable, err := manager.MkVar(uint32(outerColumn), sorts[outerColumn])
		if err != nil {
			return result, err
		}
		variableRef, err := manager.NewRef(variable)
		if err != nil {
			return result, err
		}
		valueRef, err := relations.DecodeValue(sorts[outerColumn], fact[innerColumn])
		if err != nil {
			return result, errors.Join(err, manager.DelRef(&variableRef))
		}
		variableHandle, err := manager.RefGet(variableRef)
		if err == nil {
			var value ast.ASTHandle
			value, err = manager.RefGet(valueRef)
			if err == nil {
				var equality ast.ASTHandle
				equality, err = manager.MkEq(variableHandle, value)
				if err == nil {
					var equalityRef ast.ASTRef
					equalityRef, err = manager.NewRef(equality)
					if err == nil {
						equations = append(equations, equalityRef)
						equationHandles = append(equationHandles, equality)
					}
				}
			}
		}
		err = errors.Join(err, manager.DelRef(&valueRef), manager.DelRef(&variableRef))
		if err != nil {
			return result, err
		}
	}
	result, err := manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	resultOwned = true
	if err = rewriter.SimplifyBooleanGateDefault(manager, ast.ASTOpAnd, equationHandles, result); err != nil {
		return result, err
	}
	return result, nil
}
