package spacer

// Source: src/muz/spacer/spacer_generalizers.cpp lemma_eq_generalizer.

import "github.com/glycerine/g3/pkg/ast"

func (p *PredicateTransformer) expandEqualityClasses(cube []ast.ASTHandle) (result []ast.ASTRef, dirty bool, resultErr error) {
	equivalence, err := ast.NewExprEquivClass(p.manager)
	if err != nil {
		return nil, false, err
	}
	owner := &equalityExpansionOwner{manager: p.manager, equivalence: equivalence}
	defer func() {
		resultErr = finishEqualityExpansion(owner, resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	remainder, err := p.manager.FactorEqualities(cube, equivalence)
	if err != nil {
		return nil, false, err
	}
	equalities, dirty, err := p.manager.EquivToExprFull(equivalence)
	owner.refs = equalities
	if err != nil {
		return nil, false, err
	}
	remainderRefs := make([]ast.ASTRef, 0, len(remainder))
	for _, formula := range remainder {
		ref, refErr := p.manager.NewRef(formula)
		if refErr != nil {
			return nil, false, refErr
		}
		owner.refs = append(owner.refs, ref)
		remainderRefs = append(remainderRefs, ref)
	}
	result = append(remainderRefs, equalities...)
	equalities = nil
	owner.refs = result
	return result, dirty, nil
}

func (p *PredicateTransformer) generalizeEqualityClasses(obligation *ProofObligation, lemma *Lemma) (result *Lemma, resultErr error) {
	result = lemma
	cube, err := lemma.Cube()
	if err != nil {
		return lemma, err
	}
	expanded, dirty, err := p.expandEqualityClasses(cube)
	owner := &equalityGeneralizationOwner{manager: p.manager, refs: expanded}
	defer func() {
		var published *Lemma
		if resultErr == nil && result != nil && result != lemma {
			published = result
		}
		resultErr = finishEqualityGeneralization(owner, resultErr, published)
		if resultErr != nil && result != lemma {
			result = nil
		}
	}()
	if err != nil || !dirty {
		return lemma, err
	}
	level, err := lemma.Level()
	if err != nil {
		return lemma, err
	}
	handles := make([]ast.ASTHandle, len(expanded))
	for i, ref := range expanded {
		handles[i], err = p.manager.RefGet(ref)
		if err != nil {
			return lemma, err
		}
	}
	result, resultErr = NewLemmaFromProofObligation(obligation, handles, level)
	if result != nil && result != lemma {
		owner.lemma = result
	}
	return result, resultErr
}
