package spacer

// Source: src/muz/spacer/spacer_context.cpp context::get_num_levels/add_cover.

import "github.com/glycerine/g3/pkg/ast"

func (c *Context) writeMissingPredicate(declaration ast.ASTHandle) error {
	name, err := c.manager.GetASTName(declaration)
	if err != nil {
		return err
	}
	text, _, err := c.manager.Symbols.BareString(name)
	if err != nil {
		return err
	}
	return writeSpacerVerbose(10, "did not find predicate "+text+"\n")
}
