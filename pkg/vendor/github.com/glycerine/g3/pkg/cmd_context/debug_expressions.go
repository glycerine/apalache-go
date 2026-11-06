package cmd_context

// Source: src/cmd_context/cmd_util.cpp, named expression references used by
// the SMT-LIB debug commands.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// StoreDebugExpression owns an expression reference until replacement or reset.
func (c *Context) StoreDebugExpression(name string, expression ast.ASTHandle) error {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	kind, err := m.GetKind(expression)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp && kind != ast.ASTVar && kind != ast.ASTQuantifier {
		return fmt.Errorf("global variable does not reference a term")
	}
	value, err := m.NewRef(expression)
	if err != nil {
		return err
	}
	if c.debugExpressions == nil {
		c.debugExpressions = make(map[string]ast.ASTRef)
	}
	if previous, found := c.debugExpressions[name]; found {
		if err = m.DelRef(&previous); err != nil {
			if previous == (ast.ASTRef{}) {
				delete(c.debugExpressions, name)
			} else {
				c.debugExpressions[name] = previous
			}
			return errors.Join(err, m.DelRef(&value))
		}
	}
	c.debugExpressions[name] = value
	return nil
}

// DebugExpression returns a borrowed expression, valid until replacement or reset.
func (c *Context) DebugExpression(name string) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, found := c.debugExpressions[name]
	if !found {
		return ast.ASTHandle{}, fmt.Errorf("unknown global variable %s", name)
	}
	return m.RefGet(value)
}

func (c *Context) clearDebugExpressions() error {
	for name, value := range c.debugExpressions {
		if err := c.manager.DelRef(&value); err != nil {
			if value == (ast.ASTRef{}) {
				delete(c.debugExpressions, name)
			} else {
				c.debugExpressions[name] = value
			}
			return err
		}
		delete(c.debugExpressions, name)
	}
	return nil
}
