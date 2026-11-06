package opt

import (
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func decompositionID(source DecodedFormulaObjective, index int) string {
	if source.ID != "" {
		return source.ID
	}
	return fmt.Sprintf("objective:%d", index)
}

func cloneRationals(source []big.Rat) []big.Rat {
	result := make([]big.Rat, len(source))
	for i := range source {
		result[i].Set(&source[i])
	}
	return result
}

func (d *ObjectiveDecomposer) retainedMaxSAT(source DecodedFormulaObjective) (DecomposedObjective, error) {
	result := DecomposedObjective{
		SourceKind: source.Kind, OriginalTerm: source.OriginalTerm,
		SoftTerms: make([]ast.ASTHandle, len(source.SoftTerms)),
		Weights:   cloneRationals(source.Weights), Offset: *new(big.Rat).Set(&source.Offset),
		Negate: source.Negate, ID: source.ID, Converted: true,
	}
	for i, term := range source.SoftTerms {
		retained, err := d.hold(term)
		if err != nil {
			return DecomposedObjective{}, err
		}
		result.SoftTerms[i] = retained
	}
	return result, nil
}

func (d *ObjectiveDecomposer) convertPBSum(source DecodedFormulaObjective, index int, sum pbSum) (DecomposedObjective, error) {
	result := DecomposedObjective{SourceKind: source.Kind, OriginalTerm: source.OriginalTerm, ID: decompositionID(source, index), Converted: true}
	result.Offset.Set(&sum.offset)
	result.SoftTerms = make([]ast.ASTHandle, len(sum.terms))
	result.Weights = make([]big.Rat, len(sum.weights))
	for i := range sum.terms {
		weight := new(big.Rat).Set(&sum.weights[i])
		term := sum.terms[i]
		if source.Kind == FormulaMinimize {
			if weight.Sign() < 0 {
				result.Offset.Add(&result.Offset, weight)
				weight.Neg(weight)
			} else {
				var err error
				term, err = d.negated(term)
				if err != nil {
					return DecomposedObjective{}, err
				}
			}
		} else {
			if weight.Sign() < 0 {
				weight.Neg(weight)
				var err error
				term, err = d.negated(term)
				if err != nil {
					return DecomposedObjective{}, err
				}
			}
			result.Offset.Add(&result.Offset, weight)
			result.Negate = true
		}
		result.SoftTerms[i] = term
		result.Weights[i].Set(weight)
	}
	return result, nil
}
