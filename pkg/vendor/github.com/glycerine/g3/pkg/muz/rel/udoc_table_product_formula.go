package rel

// Source: product_relation::to_formula conjoins its component formulas. Finite
// rows use table_base::to_formula in the original outer-column coordinates.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (p *UDocTableProduct) ToFormula() (result ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return result, err
	}
	m := p.manager.manager
	owner := &typedUDocFormulaOwner{manager: m}
	defer func() {
		resultErr = finishTypedUDocFormula(owner, resultErr)
		if resultErr == nil {
			result, owner.result = owner.result, ast.ASTRef{}
		} else {
			result = ast.ASTRef{}
		}
	}()
	reserve := func() (ast.ASTRef, error) {
		ref, err := m.NewRef(ast.ASTHandle{})
		if ref != (ast.ASTRef{}) {
			owner.roots = append(owner.roots, ref)
		}
		return ref, err
	}
	if p.table == nil {
		doc, err := p.doc.ToFormula()
		owner.result = doc
		return owner.result, err
	}
	mask := productTableMask(p.doc.sorts, p.tableColumns)
	innerSorts := productTableInnerSorts(p.doc.sorts, mask)
	outerColumns := make([]int, 0, len(innerSorts))
	for column, include := range mask {
		if include {
			outerColumns = append(outerColumns, column)
		}
	}
	facts, err := p.table.Facts()
	if err != nil {
		return result, err
	}
	rows := make([]ast.ASTHandle, len(facts))
	for rowIndex, fact := range facts {
		values, err := p.manager.DecodeFact(innerSorts, fact)
		owner.roots = append(owner.roots, values...)
		if err != nil {
			return result, err
		}
		equalities := make([]ast.ASTHandle, len(values))
		for column, valueRef := range values {
			value, err := m.RefGet(valueRef)
			if err != nil {
				return result, err
			}
			root, err := reserve()
			if err != nil {
				return result, err
			}
			outerColumn := outerColumns[column]
			variable, cause := m.MkVar(uint32(outerColumn), p.doc.sorts[outerColumn])
			if err = errors.Join(cause, m.SetRef(root, variable)); err != nil {
				return result, err
			}
			root, err = reserve()
			if err != nil {
				return result, err
			}
			equalities[column], cause = m.MkEq(variable, value)
			if err = errors.Join(cause, m.SetRef(root, equalities[column])); err != nil {
				return result, err
			}
		}
		root, err := reserve()
		if err != nil {
			return result, err
		}
		if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, equalities, root); err != nil {
			return result, err
		}
		rows[rowIndex], err = m.RefGet(root)
		if err != nil {
			return result, err
		}
	}
	finite, err := reserve()
	if err != nil {
		return result, err
	}
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpOr, rows, finite); err != nil {
		return result, err
	}
	doc, err := p.doc.ToFormula()
	if doc != (ast.ASTRef{}) {
		owner.roots = append(owner.roots, doc)
	}
	if err != nil {
		return result, err
	}
	docHandle, err := m.RefGet(doc)
	if err != nil {
		return result, err
	}
	finiteHandle, err := m.RefGet(finite)
	if err != nil {
		return result, err
	}
	owner.result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	return result, rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, []ast.ASTHandle{docHandle, finiteHandle}, owner.result)
}
