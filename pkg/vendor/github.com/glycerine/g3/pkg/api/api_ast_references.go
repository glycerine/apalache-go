package api

// Source: src/api/api_context.cpp:370-388.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrAPIDecRef = errors.New("api: invalid dec_ref command")

// IncASTRef adds one explicit caller reference to an AST. It is the Go
// counterpart of Z3_inc_ref for clients that need the reference-counted
// context discipline.
func (c *SMTContext) IncASTRef(value ast.ASTHandle) error {
	manager, err := c.Manager()
	if err != nil {
		return err
	}
	return manager.IncRef(value)
}

// DecASTRef releases one explicit caller reference. As in Z3_dec_ref, an AST
// whose reference count is already zero is rejected instead of underflowing.
func (c *SMTContext) DecASTRef(value ast.ASTHandle) error {
	manager, err := c.Manager()
	if err != nil {
		return err
	}
	references, err := manager.GetRefCount(value)
	if err != nil {
		return err
	}
	if references == 0 {
		return ErrAPIDecRef
	}
	return manager.DecRef(value)
}
