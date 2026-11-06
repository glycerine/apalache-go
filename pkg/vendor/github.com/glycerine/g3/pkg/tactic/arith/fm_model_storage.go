package arith

// Source: src/tactic/arith/fm_tactic.cpp:196-203.
import "github.com/glycerine/g3/pkg/ast"

func (c *FMModelConverter) Insert(variable ast.ASTHandle, clauses []ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	if k, e := c.manager.GetKind(variable); e != nil || k != ast.ASTFuncDecl {
		if e != nil {
			return e
		}
		return ast.ErrASTDomain
	}
	if e := c.manager.IncRef(variable); e != nil {
		return e
	}
	c.variables = append(c.variables, variable)
	c.clauses = append(c.clauses, make([]ast.ASTHandle, 0, len(clauses)))
	row := &c.clauses[len(c.clauses)-1]
	for _, h := range clauses {
		if e := c.manager.IncRef(h); e != nil {
			return e
		}
		*row = append(*row, h)
	}
	return nil
}
