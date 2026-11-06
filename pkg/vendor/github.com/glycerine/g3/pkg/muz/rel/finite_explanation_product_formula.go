package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp, to_formula, specialized
// to the finite/explanation product installed by dl_mk_explanations.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ToFormula returns an owning formula enumerating the finite keys and their
// explanation tuples. Finite columns precede explanation columns, matching
// the source product relation's variable shift.
func (p *FiniteExplanationProduct) ToFormula() (result ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return result, err
	}
	rows, err := p.table.Facts()
	if err != nil {
		return result, err
	}
	m := p.manager.manager
	var roots []ast.ASTRef
	ownedResult := false
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&roots[i]))
		}
		if resultErr != nil && ownedResult {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	hold := func(value ast.ASTHandle, cause error) (ast.ASTHandle, error) {
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		ref, cause := m.NewRef(value)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		roots = append(roots, ref)
		return value, nil
	}
	disjunction := make([]ast.ASTHandle, 0, len(rows))
	for _, row := range rows {
		if len(row) != len(p.finiteSorts)+1 {
			return result, ErrTableRelation
		}
		index := uint64(row[len(row)-1])
		if index >= uint64(len(p.inner)) || p.inner[index] == nil {
			return result, ErrTableRelation
		}
		explanation, err := p.inner[index].Data()
		if err != nil {
			return result, err
		}
		if len(explanation) != p.explanationColumns {
			return result, ErrTableRelation
		}
		values, err := p.manager.DecodeFact(p.finiteSorts, row[:len(row)-1])
		if err != nil {
			return result, err
		}
		roots = append(roots, values...)
		conjuncts := make([]ast.ASTHandle, 0, len(p.finiteSorts)+len(explanation))
		for column := range p.finiteSorts {
			variable, err := hold(m.MkVar(uint32(column), p.finiteSorts[column]))
			if err != nil {
				return result, err
			}
			value, err := m.RefGet(values[column])
			if err != nil {
				return result, err
			}
			equality, err := hold(m.MkEq(variable, value))
			if err != nil {
				return result, err
			}
			conjuncts = append(conjuncts, equality)
		}
		for column, value := range explanation {
			if value.IsNull() {
				return result, ErrTableRelation
			}
			sort, err := m.GetSort(value)
			if err != nil {
				return result, err
			}
			variable, err := hold(m.MkVar(uint32(len(p.finiteSorts)+column), sort))
			if err != nil {
				return result, err
			}
			equality, err := hold(m.MkEq(variable, value))
			if err != nil {
				return result, err
			}
			conjuncts = append(conjuncts, equality)
		}
		conjunctionRef, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return result, err
		}
		roots = append(roots, conjunctionRef)
		if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, conjuncts, conjunctionRef); err != nil {
			return result, err
		}
		conjunction, err := m.RefGet(conjunctionRef)
		if err != nil {
			return result, err
		}
		disjunction = append(disjunction, conjunction)
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	ownedResult = true
	resultErr = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpOr, disjunction, result)
	return result, resultErr
}
