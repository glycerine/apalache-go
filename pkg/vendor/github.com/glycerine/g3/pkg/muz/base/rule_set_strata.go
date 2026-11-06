package base

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
)

func (s *RuleSet) Strata() ([][]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if !s.finalized {
		return nil, fmt.Errorf("muz/base: finalize rule set before reading strata")
	}
	max := uint32(0)
	for _, value := range s.strata {
		if value > max {
			max = value
		}
	}
	result := make([][]ast.ASTHandle, max+1)
	seen := make(map[ast.ASTHandle]bool)
	appendDeclaration := func(declaration ast.ASTHandle) {
		if !seen[declaration] {
			seen[declaration] = true
			result[s.strata[declaration]] = append(result[s.strata[declaration]], declaration)
		}
	}
	for _, declaration := range s.headOrder {
		appendDeclaration(declaration)
		for _, dependency := range s.dependencies[declaration] {
			appendDeclaration(dependency)
		}
	}
	return result, nil
}
