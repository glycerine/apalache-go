package rel

// Source: src/muz/rel/dl_product_relation.cpp to_formula, specialized to the
// finite table plus whole-relation explanation product from dl_mk_explanations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ToFormula returns an owning formula for all finite rows paired with the
// product's single global explanation row. Finite variables precede proof
// variables, matching the source product's outer signature.
func (p *RelationExplanationProduct) ToFormula() (result ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return result, err
	}
	if p.doc != nil {
		return p.toFormulaWithUDoc()
	}
	if p.numeric != nil {
		return p.toFormulaWithNumeric()
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

	var explanation []ast.ASTHandle
	proofEmpty := false
	if len(rows) != 0 {
		empty, explanationErr := p.explanation.Empty()
		if explanationErr != nil {
			return result, explanationErr
		}
		if empty {
			// The product is empty when either component is empty. Native
			// explanation to_formula dereferences stale/missing data here;
			// normalize that unsafe source boundary to the logical false result.
			proofEmpty = true
		} else {
			explanation, explanationErr = p.explanation.Data()
			if explanationErr != nil {
				return result, explanationErr
			}
			for _, value := range explanation {
				if value.IsNull() {
					return result, ErrTableRelation
				}
			}
		}
	}

	disjunction := make([]ast.ASTHandle, 0, len(rows))
	for _, row := range rows {
		if len(row) != len(p.finiteSorts) {
			return result, ErrTableRelation
		}
		values, decodeErr := p.manager.DecodeFact(p.finiteSorts, row)
		if decodeErr != nil {
			return result, decodeErr
		}
		roots = append(roots, values...)
		conjuncts := make([]ast.ASTHandle, 0, len(p.finiteSorts))
		for column := range p.finiteSorts {
			variable, makeErr := hold(m.MkVar(uint32(column), p.finiteSorts[column]))
			if makeErr != nil {
				return result, makeErr
			}
			value, getErr := m.RefGet(values[column])
			if getErr != nil {
				return result, getErr
			}
			equality, makeErr := hold(m.MkEq(variable, value))
			if makeErr != nil {
				return result, makeErr
			}
			conjuncts = append(conjuncts, equality)
		}
		conjunctionRef, makeErr := m.NewRef(ast.ASTHandle{})
		if makeErr != nil {
			return result, makeErr
		}
		roots = append(roots, conjunctionRef)
		if makeErr = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, conjuncts, conjunctionRef); makeErr != nil {
			return result, makeErr
		}
		conjunction, getErr := m.RefGet(conjunctionRef)
		if getErr != nil {
			return result, getErr
		}
		disjunction = append(disjunction, conjunction)
	}
	finiteRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	roots = append(roots, finiteRef)
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpOr, disjunction, finiteRef); err != nil {
		return result, err
	}
	finiteFormula, err := m.RefGet(finiteRef)
	if err != nil {
		return result, err
	}

	proofConjuncts := make([]ast.ASTHandle, 0, len(explanation))
	for column, value := range explanation {
		sort, getErr := m.GetSort(value)
		if getErr != nil {
			return result, getErr
		}
		variable, makeErr := hold(m.MkVar(uint32(len(p.finiteSorts)+column), sort))
		if makeErr != nil {
			return result, makeErr
		}
		equality, makeErr := hold(m.MkEq(variable, value))
		if makeErr != nil {
			return result, makeErr
		}
		proofConjuncts = append(proofConjuncts, equality)
	}
	proofRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	roots = append(roots, proofRef)
	proofGate := ast.ASTOpAnd
	if proofEmpty {
		proofGate = ast.ASTOpOr
	}
	if err = rewriter.SimplifyBooleanGateDefault(m, proofGate, proofConjuncts, proofRef); err != nil {
		return result, err
	}
	proofFormula, err := m.RefGet(proofRef)
	if err != nil {
		return result, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	ownedResult = true
	resultErr = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, []ast.ASTHandle{finiteFormula, proofFormula}, result)
	return result, resultErr
}
