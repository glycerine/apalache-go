package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *ObjectiveFormulaCodec) EncodeObjective(objective FormulaObjective) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	var args []ast.ASTHandle
	metadata := objectiveFormulaMetadata{kind: objective.Kind, originalTerm: objective.Term, id: objective.ID}
	switch objective.Kind {
	case FormulaMaximize, FormulaMinimize:
		if objective.Term.IsNull() {
			return ast.ASTHandle{}, ast.ErrASTHandle
		}
		args = []ast.ASTHandle{objective.Term}
	case FormulaMaxSAT:
		if len(objective.SoftTerms) != len(objective.Weights) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		args = append([]ast.ASTHandle(nil), objective.SoftTerms...)
		metadata.weights = make([]big.Rat, len(objective.Weights))
		for i, weight := range objective.Weights {
			if weight == nil {
				return ast.ASTHandle{}, ast.ErrASTHandle
			}
			metadata.weights[i].Set(weight)
		}
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	declaration, err := c.freshDeclaration(objective.Kind, args, metadata)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.manager.MkApp(declaration, args)
}
