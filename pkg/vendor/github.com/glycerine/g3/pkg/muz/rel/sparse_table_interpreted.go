package rel

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpreted applies the relation manager's default finite-table
// evaluator to sparse rows, removing rows for which condition rewrites to
// false. Evaluation completes before any row is removed.
func (t *SparseTable) FilterInterpreted(manager *RelationManager, sorts []ast.ASTHandle, rewriter *astrewriter.TheoryRewriter, condition ast.ASTHandle) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if manager == nil || manager.open() != nil || rewriter == nil || len(sorts) != t.signature.Len() {
		return ErrTable
	}
	managerAST := manager.manager
	boolean, err := managerAST.MkBoolSort()
	if err != nil {
		return err
	}
	conditionSort, err := managerAST.GetSort(condition)
	if err != nil {
		return err
	}
	if conditionSort != boolean {
		return fmt.Errorf("%w: interpreted condition is not Boolean", ErrTable)
	}
	for column, sort := range sorts {
		domain, domainErr := manager.sortDomain(sort)
		if domainErr != nil || domain != t.signature.domains[column] {
			return fmt.Errorf("%w: interpreted sort does not match sparse column %d", ErrTable, column)
		}
	}
	freeVars, err := astrewriter.NewExprFreeVars(managerAST)
	if err != nil {
		return err
	}
	if err = freeVars.Collect(condition); err != nil {
		return err
	}
	if int(freeVars.Size()) > len(sorts) {
		return fmt.Errorf("%w: condition variable exceeds sparse signature", ErrTable)
	}
	for variable := uint32(0); variable < freeVars.Size(); variable++ {
		if !freeVars.Contains(variable) {
			continue
		}
		variableSort, getErr := freeVars.Get(variable)
		if getErr != nil {
			return getErr
		}
		if variableSort != sorts[variable] {
			return fmt.Errorf("%w: condition variable sort does not match sparse column %d", ErrTable, variable)
		}
	}
	facts, err := t.Facts()
	if err != nil {
		return err
	}
	removed := make([]Fact, 0)
	for _, fact := range facts {
		bindings := make([]ast.ASTHandle, len(sorts))
		refs := make([]ast.ASTRef, 0, freeVars.Size()+1)
		accepted, evalErr := func() (bool, error) {
			defer func() {
				for i := len(refs) - 1; i >= 0; i-- {
					resultErr = errors.Join(resultErr, managerAST.DelRef(&refs[i]))
				}
			}()
			for variable := uint32(0); variable < freeVars.Size(); variable++ {
				if !freeVars.Contains(variable) {
					continue
				}
				value, decodeErr := manager.DecodeValue(sorts[variable], fact[variable])
				if decodeErr != nil {
					return false, decodeErr
				}
				refs = append(refs, value)
				handle, getErr := managerAST.RefGet(value)
				if getErr != nil {
					return false, getErr
				}
				bindings[tableRelationBindingIndex(len(bindings), variable)] = handle
			}
			output, newErr := managerAST.NewRef(ast.ASTHandle{})
			if newErr != nil {
				return false, newErr
			}
			refs = append(refs, output)
			if rewriteErr := rewriter.RewriteWithBindings(condition, bindings, output); rewriteErr != nil {
				return false, rewriteErr
			}
			result, getErr := managerAST.RefGet(output)
			if getErr != nil {
				return false, getErr
			}
			if managerAST.IsFalse(result) {
				return false, nil
			}
			// The source removes only explicit false results; an expression
			// that remains symbolic is kept.
			return true, nil
		}()
		if evalErr != nil {
			return errors.Join(evalErr, resultErr)
		}
		if resultErr != nil {
			return resultErr
		}
		if !accepted {
			removed = append(removed, fact)
		}
	}
	_, err = t.RemoveFacts(removed)
	return err
}
