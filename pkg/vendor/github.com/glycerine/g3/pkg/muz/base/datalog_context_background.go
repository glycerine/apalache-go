package base

// Source: src/muz/base/dl_context.cpp context::assert_expr and the assertion
// accessors in src/muz/base/dl_context.h.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *DatalogContext) AssertExpression(expression ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	sort, err := c.manager.GetSort(expression)
	if err != nil {
		return err
	}
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if sort != boolean {
		return fmt.Errorf("muz/base: background assertion is not Boolean")
	}
	reference, err := c.manager.NewRef(expression)
	if err != nil {
		return err
	}
	c.background = append(c.background, reference)
	return nil
}

func (c *DatalogContext) BackgroundAssertionCount() (int, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return len(c.background), nil
}

// BackgroundAssertion returns a borrowed assertion handle.
func (c *DatalogContext) BackgroundAssertion(index int) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if index < 0 || index >= len(c.background) {
		return ast.ASTHandle{}, fmt.Errorf("muz/base: background assertion index out of range")
	}
	return c.manager.RefGet(c.background[index])
}
