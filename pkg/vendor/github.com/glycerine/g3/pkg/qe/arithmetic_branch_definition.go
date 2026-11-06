package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) branchDefinition(x ArithmeticBranchSelection, s *arithmeticBranchSubstitution) (ast.ASTRef, error) {
	if x.Kind == ArithmeticInfinityBranch {
		return ast.ASTRef{}, nil
	}
	return p.finiteBranchDefinition(x, s)
}
