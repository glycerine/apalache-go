package base

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *DatalogContext) querySignature(sorts []ast.ASTHandle, names []usymbol.Symbol) ([]ast.ASTHandle, []usymbol.Symbol, error) {
	if len(names) > len(sorts) {
		return nil, nil, ast.ErrASTDomain
	}
	for len(names) < len(sorts) {
		name, err := c.manager.Symbols.Intern(strconv.Itoa(len(names)))
		if err != nil {
			return nil, nil, err
		}
		names = append(names, name)
	}
	for left, right := 0, len(sorts)-1; left < right; left, right = left+1, right-1 {
		sorts[left], sorts[right] = sorts[right], sorts[left]
		names[left], names[right] = names[right], names[left]
	}
	return sorts, names, nil
}
