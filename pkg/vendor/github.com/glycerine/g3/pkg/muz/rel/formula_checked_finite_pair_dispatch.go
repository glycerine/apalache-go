package rel

// Source: check_relation(table_relation(check_table)) mixed-representation
// conversion. Generic dispatch can lift only an unambiguous nullary identity;
// relation products use the typed helper with the actual AST sorts.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func liftFormulaCheckedFinitePairLike(template *FormulaCheckedFinitePair, source FiniteTable) (_ *FormulaCheckedFinitePair, resultErr error) {
	if concrete, ok := source.(*FormulaCheckedFinitePair); ok {
		return concrete, nil
	}
	signature, err := source.Signature()
	if err != nil {
		return nil, err
	}
	if signature.Len() != 0 {
		return nil, ErrCompilerUnsupported
	}
	return liftFormulaCheckedFinitePairWithSorts(template, source, nil)
}

func liftFormulaCheckedFinitePairWithSorts(template *FormulaCheckedFinitePair, source FiniteTable, sorts []ast.ASTHandle) (_ *FormulaCheckedFinitePair, resultErr error) {
	if concrete, ok := source.(*FormulaCheckedFinitePair); ok {
		return concrete, nil
	}
	if err := template.open(); err != nil {
		return nil, err
	}
	if concrete, ok := source.(*FormulaCheckedTable); ok {
		if len(sorts) != len(concrete.table.sorts) {
			return nil, ast.ErrASTDomain
		}
		for index := range sorts {
			if sorts[index] != concrete.table.sorts[index] {
				return nil, ast.ErrASTDomain
			}
		}
		// Clone verifies the existing expected formula against the concrete
		// rows before this operand crosses into the nested checker.
		formula, err := concrete.Clone()
		if err != nil {
			return nil, err
		}
		pair, err := template.pair.Lift(source)
		if err != nil {
			return nil, errors.Join(err, formula.Close())
		}
		return adoptFormulaCheckedFinitePair(pair, formula)
	}
	pair, err := template.pair.Lift(source)
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedFinitePair(template.formula.table.manager, sorts, pair, template.formula.config)
	if err != nil {
		// NewFormulaCheckedFinitePair transfers pair only on success.
		return nil, errors.Join(err, pair.Close())
	}
	return result, nil
}
