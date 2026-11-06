package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (p *RelationExplanationProduct) toFormulaWithUDoc() (result ast.ASTRef, resultErr error) {
	docFormula, err := p.doc.ToFormula()
	if err != nil {
		return ast.ASTRef{}, err
	}
	m := p.manager.manager
	roots := []ast.ASTRef{docFormula}
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
	data, err := p.explanation.Data()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if len(data) == 0 {
		falsehood, err := m.MkFalse()
		if err != nil {
			return ast.ASTRef{}, err
		}
		result, err = m.NewRef(falsehood)
		return result, err
	}
	proofEqualities := make([]ast.ASTHandle, 0, len(data))
	for i, value := range data {
		if value.IsNull() {
			falsehood, err := m.MkFalse()
			if err != nil {
				return ast.ASTRef{}, err
			}
			result, err = m.NewRef(falsehood)
			return result, err
		}
		sort, err := m.GetSort(value)
		if err != nil {
			return ast.ASTRef{}, err
		}
		variable, err := m.MkVar(uint32(len(p.finiteSorts)+i), sort)
		if err != nil {
			return ast.ASTRef{}, err
		}
		variableRef, err := m.NewRef(variable)
		if variableRef != (ast.ASTRef{}) {
			roots = append(roots, variableRef)
		}
		if err != nil {
			return ast.ASTRef{}, err
		}
		equality, err := m.MkEq(variable, value)
		if err != nil {
			return ast.ASTRef{}, err
		}
		equalityRef, err := m.NewRef(equality)
		if equalityRef != (ast.ASTRef{}) {
			roots = append(roots, equalityRef)
		}
		if err != nil {
			return ast.ASTRef{}, err
		}
		proofEqualities = append(proofEqualities, equality)
	}
	proofRef, err := m.NewRef(ast.ASTHandle{})
	if proofRef != (ast.ASTRef{}) {
		roots = append(roots, proofRef)
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, proofEqualities, proofRef); err != nil {
		return ast.ASTRef{}, err
	}
	proof, err := m.RefGet(proofRef)
	if err != nil {
		return ast.ASTRef{}, err
	}
	doc, err := m.RefGet(docFormula)
	if err != nil {
		return ast.ASTRef{}, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if result != (ast.ASTRef{}) {
		ownedResult = true
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, []ast.ASTHandle{doc, proof}, result)
	return result, err
}
