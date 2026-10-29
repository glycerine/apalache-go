package spacer

// Source: src/muz/spacer/spacer_generalizers.cpp lemma_eq_generalizer.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *PredicateTransformer) expandEqualityClasses(cube []ast.ASTHandle) (_ []ast.ASTHandle, dirty bool, resultErr error) {
	equivalence, err := ast.NewExprEquivClass(p.manager)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, equivalence.Close()) }()
	remainder, err := p.manager.FactorEqualities(cube, equivalence)
	if err != nil {
		return nil, false, err
	}
	equalities, dirty, err := p.manager.EquivToExprFull(equivalence)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, equalities)) }()
	result := append([]ast.ASTHandle(nil), remainder...)
	for i := range equalities {
		handle, err := p.manager.RefGet(equalities[i])
		if err != nil {
			return nil, false, err
		}
		result = append(result, handle)
	}
	return result, dirty, nil
}

func (p *PredicateTransformer) generalizeEqualityClasses(obligation *ProofObligation, lemma *Lemma) (*Lemma, error) {
	cube, err := lemma.Cube()
	if err != nil {
		return lemma, err
	}
	expanded, dirty, err := p.expandEqualityClasses(cube)
	if err != nil || !dirty {
		return lemma, err
	}
	level, err := lemma.Level()
	if err != nil {
		return lemma, err
	}
	return NewLemmaFromProofObligation(obligation, expanded, level)
}
