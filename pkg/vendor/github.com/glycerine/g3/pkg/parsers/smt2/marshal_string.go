package smt2

import (
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func MarshalString(expression ast.ASTHandle, manager *ast.ASTManager) (string, error) {
	var output strings.Builder
	if err := Marshal(&output, expression, manager); err != nil {
		return "", err
	}
	return output.String(), nil
}
