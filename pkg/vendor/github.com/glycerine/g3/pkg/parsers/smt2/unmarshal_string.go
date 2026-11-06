package smt2

import (
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func UnmarshalString(input string, manager *ast.ASTManager) (ast.ASTRef, error) {
	return Unmarshal(strings.NewReader(input), manager)
}
