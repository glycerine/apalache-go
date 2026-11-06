package opt

import "github.com/glycerine/g3/pkg/ast"

func (c *ObjectiveFormulaCodec) Decode(formulas []ast.ASTHandle) (DecodedFormulaSet, error) {
	if err := c.open(); err != nil {
		return DecodedFormulaSet{}, err
	}
	var result DecodedFormulaSet
	for _, formula := range formulas {
		kind, err := c.manager.GetKind(formula)
		if err != nil {
			return DecodedFormulaSet{}, err
		}
		if kind != ast.ASTApp {
			result.Hard = append(result.Hard, formula)
			continue
		}
		declaration, err := c.manager.GetDecl(formula)
		if err != nil {
			return DecodedFormulaSet{}, err
		}
		metadata, marked := c.metadata[declaration]
		if !marked {
			result.Hard = append(result.Hard, formula)
			continue
		}
		count, err := c.manager.GetNumArgs(formula)
		if err != nil {
			return DecodedFormulaSet{}, err
		}
		args := make([]ast.ASTHandle, count)
		for i := uint32(0); i < count; i++ {
			args[i], err = c.manager.GetArg(formula, i)
			if err != nil {
				return DecodedFormulaSet{}, err
			}
		}
		objective := DecodedFormulaObjective{Kind: metadata.kind, OriginalTerm: metadata.originalTerm, ID: metadata.id}
		switch metadata.kind {
		case FormulaMaximize, FormulaMinimize:
			if len(args) != 1 {
				return DecodedFormulaSet{}, ast.ErrASTDomain
			}
			objective.Term = args[0]
			objective.Negate = metadata.kind == FormulaMinimize
		case FormulaMaxSAT:
			objective.SoftTerms, objective.Weights, objective.Offset, err = c.normalizeSoft(args, metadata)
			if err != nil {
				return DecodedFormulaSet{}, err
			}
		default:
			return DecodedFormulaSet{}, ast.ErrASTDomain
		}
		result.Objectives = append(result.Objectives, objective)
	}
	return result, nil
}
