package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

type arithmeticCompleteProject struct {
	plugin       *ArithmeticPlugin
	atoms        *ArithmeticAtomSet
	call         *arithmeticModelProjectCall
	variable     *ast.ContainsApp
	root, target ast.ASTHandle
	entry        *arithmeticPluginBounds
}

func (c *arithmeticCompleteProject) close() error {
	var a, b error
	if c.call != nil {
		a = c.call.close()
	}
	if c.atoms != nil {
		b = c.atoms.Close()
	}
	return errors.Join(a, b)
}
