package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ObjectivePurifier) Purify(hard []ast.ASTHandle, objectives []DecomposedObjective) (PurifiedObjectiveSet, error) {
	if err := p.open(); err != nil {
		return PurifiedObjectiveSet{}, err
	}
	result := PurifiedObjectiveSet{Hard: make([]ast.ASTHandle, 0, len(hard)), Objectives: make([]DecomposedObjective, len(objectives))}
	for _, formula := range hard {
		if _, err := p.hold(formula); err != nil {
			return PurifiedObjectiveSet{}, err
		}
		result.Hard = append(result.Hard, formula)
	}
	for i := range objectives {
		source := &objectives[i]
		target := DecomposedObjective{SourceKind: source.SourceKind, Offset: *new(big.Rat).Set(&source.Offset), Negate: source.Negate, ID: source.ID, Converted: source.Converted, Weights: cloneRationals(source.Weights)}
		if !source.OriginalTerm.IsNull() {
			if _, err := p.hold(source.OriginalTerm); err != nil {
				return PurifiedObjectiveSet{}, err
			}
			target.OriginalTerm = source.OriginalTerm
		}
		if !source.Term.IsNull() {
			term, err := p.arithmeticTerm(source.Term, &result.Hard)
			if err != nil {
				return PurifiedObjectiveSet{}, err
			}
			if _, err = p.hold(term); err != nil {
				return PurifiedObjectiveSet{}, err
			}
			target.Term = term
		}
		target.SoftTerms = make([]ast.ASTHandle, len(source.SoftTerms))
		for j, term := range source.SoftTerms {
			atomic, err := p.atomic(term, &result.Hard)
			if err != nil {
				return PurifiedObjectiveSet{}, err
			}
			if _, err = p.hold(atomic); err != nil {
				return PurifiedObjectiveSet{}, err
			}
			target.SoftTerms[j] = atomic
		}
		result.Objectives[i] = target
	}
	return result, nil
}
