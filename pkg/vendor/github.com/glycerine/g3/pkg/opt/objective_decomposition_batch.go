package opt

import (
	"github.com/glycerine/g3/pkg/ast"
)

// Decompose applies opt_context::is_maxsat conversion in source objective
// order. Non-convertible arithmetic objectives remain explicit.
func (d *ObjectiveDecomposer) Decompose(source DecodedFormulaSet) ([]DecomposedObjective, ObjectiveDecompositionStats, error) {
	if err := d.open(); err != nil {
		return nil, ObjectiveDecompositionStats{}, err
	}
	result := make([]DecomposedObjective, 0, len(source.Objectives))
	stats := ObjectiveDecompositionStats{Objectives: len(source.Objectives)}
	for index, objective := range source.Objectives {
		if objective.Kind == FormulaMaxSAT {
			converted, err := d.retainedMaxSAT(objective)
			if err != nil {
				return nil, ObjectiveDecompositionStats{}, err
			}
			result = append(result, converted)
			stats.RetainedMaxSAT++
			continue
		}
		if objective.Kind != FormulaMaximize && objective.Kind != FormulaMinimize {
			return nil, ObjectiveDecompositionStats{}, ast.ErrASTDomain
		}
		sum, yes, err := d.pseudoBooleanSum(objective.Term)
		if err != nil {
			return nil, ObjectiveDecompositionStats{}, err
		}
		if yes {
			converted, e := d.convertPBSum(objective, index, sum)
			if e != nil {
				return nil, ObjectiveDecompositionStats{}, e
			}
			result = append(result, converted)
			stats.ConvertedPBSums++
			continue
		}
		converted, yes, err := d.convertBitVector(objective, index)
		if err != nil {
			return nil, ObjectiveDecompositionStats{}, err
		}
		if yes {
			result = append(result, converted)
			stats.ConvertedBVTerms++
			continue
		}
		result = append(result, DecomposedObjective{SourceKind: objective.Kind, Term: objective.Term, OriginalTerm: objective.OriginalTerm, ID: objective.ID})
		stats.Unconverted++
	}
	stats.GeneratedTerms = d.generated
	return result, stats, nil
}
