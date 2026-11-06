package cmd_context

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *Context) makeOPBVariable(variable int32, declarations map[int32]ast.ASTHandle) (ast.ASTHandle, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, found := declarations[variable]
	if !found {
		boolean, e := manager.MkBoolSort()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		declaration, e = c.DeclareFunction(strconv.FormatInt(int64(variable), 10), nil, boolean)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		declarations[variable] = declaration
	}
	return manager.MkApp(declaration, nil)
}
