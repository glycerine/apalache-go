package rel

// Source: src/muz/rel/check_relation.cpp operation wrappers. These helpers
// keep the finite checker on compiler temporaries while proof aggregation
// remains an independent relation-level component.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func emptyFormulaCheckedLike(source *FormulaCheckedTable) (*FormulaCheckedTable, error) {
	if err := source.open(); err != nil {
		return nil, err
	}
	table, err := NewTableRelation(source.table.manager, source.table.sorts)
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedTable(table, source.config)
	if err != nil {
		_ = table.Close()
		return nil, err
	}
	return result, nil
}

// liftFormulaCheckedLike returns source itself when it already has the right
// checker kind. Otherwise the caller owns the returned temporary.
func liftFormulaCheckedLike(template *FormulaCheckedTable, source FiniteTable) (_ *FormulaCheckedTable, resultErr error) {
	if concrete, ok := source.(*FormulaCheckedTable); ok {
		return concrete, nil
	}
	signature, err := source.Signature()
	if err != nil {
		return nil, err
	}
	// Signature carries cardinalities, not AST sorts, so only a foreign
	// nullary identity can be lifted without guessing its source sorts.
	if signature.Len() != 0 {
		return nil, ErrCompilerUnsupported
	}
	plain, err := NewTableRelation(template.table.manager, nil)
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedTable(plain, template.config)
	if err != nil {
		_ = plain.Close()
		return nil, err
	}
	owned := true
	defer func() {
		if resultErr != nil && owned {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	resultSignature, err := result.Signature()
	if err != nil || !resultSignature.equal(signature) {
		return nil, ErrTable
	}
	facts, err := source.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if _, err = result.Add(fact); err != nil {
			return nil, err
		}
	}
	owned = false
	return result, nil
}

func liftFormulaCheckedWithSorts(template *FormulaCheckedTable, source FiniteTable, sorts []ast.ASTHandle) (_ *FormulaCheckedTable, resultErr error) {
	if concrete, ok := source.(*FormulaCheckedTable); ok {
		return concrete, nil
	}
	if len(sorts) == 0 {
		return liftFormulaCheckedLike(template, source)
	}
	if err := template.open(); err != nil {
		return nil, ErrCompilerUnsupported
	}
	relation, err := NewTableRelation(template.table.manager, sorts)
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedTable(relation, template.config)
	if err != nil {
		return nil, errors.Join(err, relation.Close())
	}
	owned := true
	defer func() {
		if resultErr != nil && owned {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	facts, err := source.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if _, err = result.Add(fact); err != nil {
			return nil, err
		}
	}
	owned = false
	return result, nil
}

func formulaCheckedFilterEqual(table *FormulaCheckedTable, value Element, column int) (resultErr error) {
	if err := table.open(); err != nil || column < 0 || column >= len(table.table.sorts) {
		return ErrTable
	}
	decoded, err := table.table.manager.DecodeValue(table.table.sorts[column], value)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, table.manager.DelRef(&decoded)) }()
	handle, err := table.manager.RefGet(decoded)
	if err != nil {
		return err
	}
	return table.FilterEqual(handle, column)
}

func relationExplanationFiniteUnion(target, source, delta FiniteTable, sorts []ast.ASTHandle) (resultErr error) {
	if nested, ok := target.(*FormulaCheckedFinitePair); ok {
		nestedSource, err := liftFormulaCheckedFinitePairWithSorts(nested, source, sorts)
		if err != nil {
			return err
		}
		sourceOwned := nestedSource != source
		if sourceOwned {
			defer func() { resultErr = errors.Join(resultErr, nestedSource.Close()) }()
		}
		var nestedDelta *FormulaCheckedFinitePair
		deltaOwned := false
		if delta != nil {
			if concrete, same := delta.(*FormulaCheckedFinitePair); same {
				nestedDelta = concrete
			} else {
				nestedDelta, err = liftFormulaCheckedFinitePairWithSorts(nested, delta, sorts)
				if err != nil {
					return err
				}
				deltaOwned = true
				defer func() { resultErr = errors.Join(resultErr, nestedDelta.Close()) }()
			}
		}
		if err = nested.UnionFrom(nestedSource, nestedDelta); err != nil {
			return err
		}
		if deltaOwned {
			facts, err := nestedDelta.Facts()
			if err != nil {
				return err
			}
			if err = delta.Reset(); err != nil {
				return err
			}
			for _, fact := range facts {
				if _, err = delta.Add(fact); err != nil {
					return err
				}
			}
		}
		return nil
	}
	checked, ok := target.(*FormulaCheckedTable)
	if !ok {
		return finiteTableUnion(target, source, delta)
	}
	checkedSource, err := liftFormulaCheckedWithSorts(checked, source, sorts)
	if err != nil {
		return err
	}
	sourceOwned := checkedSource != source
	if sourceOwned {
		defer func() { resultErr = errors.Join(resultErr, checkedSource.Close()) }()
	}
	var checkedDelta *FormulaCheckedTable
	deltaOwned := false
	if delta != nil {
		if concrete, same := delta.(*FormulaCheckedTable); same {
			checkedDelta = concrete
		} else {
			checkedDelta, err = liftFormulaCheckedWithSorts(checked, delta, sorts)
			if err != nil {
				return err
			}
			deltaOwned = true
			defer func() { resultErr = errors.Join(resultErr, checkedDelta.Close()) }()
		}
	}
	if err = checked.UnionFrom(checkedSource, checkedDelta); err != nil {
		return err
	}
	if deltaOwned {
		facts, err := checkedDelta.Facts()
		if err != nil {
			return err
		}
		if err = delta.Reset(); err != nil {
			return err
		}
		for _, fact := range facts {
			if _, err = delta.Add(fact); err != nil {
				return err
			}
		}
	}
	return nil
}

func formulaCheckedFilterInterpretedBound(table *FormulaCheckedTable, theory *rewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) (resultErr error) {
	if err := table.open(); err != nil || theory == nil || theory.Manager() != table.manager {
		return ErrTableRelation
	}
	for _, condition := range conditions {
		used, err := ast.NewASTUsedVars(table.manager)
		if err != nil {
			return err
		}
		if err = used.Collect(condition); err != nil {
			return err
		}
		bindings := make([]ast.ASTHandle, used.MaxFoundVarIndexPlusOne())
		refs := make([]ast.ASTRef, 0, len(bindings)+1)
		cleanup := func() error {
			var closeErr error
			for index := len(refs) - 1; index >= 0; index-- {
				closeErr = errors.Join(closeErr, table.manager.DelRef(&refs[index]))
			}
			return closeErr
		}
		for variable := range bindings {
			sort := used.Contains(uint32(variable))
			if sort.IsNull() {
				continue
			}
			column, found := columns[uint32(variable)]
			declared, sorted := sorts[uint32(variable)]
			if !found || !sorted || declared != sort || column < 0 || column >= len(table.table.sorts) || table.table.sorts[column] != sort {
				return errors.Join(ErrTableRelation, cleanup())
			}
			variableHandle, err := table.manager.MkVar(uint32(column), sort)
			if err != nil {
				return errors.Join(err, cleanup())
			}
			ref, err := table.manager.NewRef(variableHandle)
			if err != nil {
				return errors.Join(err, cleanup())
			}
			refs = append(refs, ref)
			bindings[variable] = variableHandle
		}
		substitution, err := rewriter.NewVarSubstitution(table.manager, false)
		if err != nil {
			return errors.Join(err, cleanup())
		}
		out, err := table.manager.NewRef(ast.ASTHandle{})
		if err == nil {
			refs = append(refs, out)
			err = substitution.Substitute(condition, bindings, out)
		}
		if err == nil {
			var mapped ast.ASTHandle
			mapped, err = table.manager.RefGet(out)
			if err == nil {
				err = table.FilterInterpreted(theory, mapped)
			}
		}
		err = errors.Join(err, substitution.Close(), cleanup())
		if err != nil {
			return err
		}
	}
	return nil
}
