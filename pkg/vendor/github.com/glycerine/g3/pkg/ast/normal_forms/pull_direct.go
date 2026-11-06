package normal_forms

// Source: src/ast/normal_forms/pull_quant.cpp:163-197;
// src/ast/pattern/pattern_inference.cpp:679-698.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// In the direct forall(forall(body)) case, the inner body's de Bruijn indices
// already address inner binders first and outer binders afterward. Source
// pull_quant1_core therefore concatenates declarations without shifting body.
func (p *pullQuantCore) pullDirectForall(outer, inner ast.ASTHandle) (ast.ASTHandle, error) {
	m := p.manager
	outerCount, err := m.GetNumDeclarations(outer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	innerCount, err := m.GetNumDeclarations(inner)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sorts := make([]ast.ASTHandle, 0, outerCount+innerCount)
	names := make([]symbol.Symbol, 0, outerCount+innerCount)
	for _, item := range []struct {
		q ast.ASTHandle
		n uint32
	}{{outer, outerCount}, {inner, innerCount}} {
		for i := uint32(0); i < item.n; i++ {
			sort, e := m.GetDeclarationSort(item.q, i)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			name, e := m.GetDeclarationName(item.q, i)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			sorts = append(sorts, sort)
			names = append(names, name)
		}
	}
	outerWeight, err := m.GetQuantifierWeight(outer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	innerWeight, err := m.GetQuantifierWeight(inner)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if innerWeight < outerWeight {
		outerWeight = innerWeight
	}
	qid, _, err := m.GetQuantifierIDs(outer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	body, err := m.GetQuantifierBody(inner)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names,
		Body: body, Weight: outerWeight, QID: qid})
}
