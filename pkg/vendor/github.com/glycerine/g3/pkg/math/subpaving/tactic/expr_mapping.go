package tactic

// Source: src/math/subpaving/tactic/expr2subpaving.h:34-47 and
// subpaving_tactic.cpp:32-49 (owning inverse-map expressions).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func pinExprMapping(m *ast.ASTManager, entries []ast.ExprVarBinding) ([]ast.ASTHandle, error) {
	pins := make([]ast.ASTHandle, 0, len(entries))
	for _, entry := range entries {
		if err := m.IncRef(entry.Expr); err != nil {
			for _, h := range pins {
				_, cleanup := m.DecRefConsumed(h)
				err = errors.Join(err, cleanup)
			}
			return nil, err
		}
		pins = append(pins, entry.Expr)
	}
	return pins, nil
}

func releaseExprPins(m *ast.ASTManager, pins *[]ast.ASTHandle) error {
	for i, h := range *pins {
		if h.IsNull() {
			continue
		}
		consumed, err := m.DecRefConsumed(h)
		if consumed {
			(*pins)[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
	}
	*pins = nil
	return nil
}

// Manager returns the borrowed AST manager.
func (c *HardwareExprConverter) Manager() (*ast.ASTManager, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	return c.ast, nil
}

// Engine returns the borrowed concrete subpaving engine.
func (c *HardwareExprConverter) Engine() (*HardwareEngine, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	return c.engine, nil
}

// Manager returns the borrowed AST manager.
func (c *ArbitraryExprConverter) Manager() (*ast.ASTManager, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	return c.ast, nil
}

// Engine returns the borrowed concrete subpaving engine.
func (c *ArbitraryExprConverter) Engine() (*ArbitraryEngine, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	return c.engine, nil
}

// Manager returns the borrowed AST manager.
func (c *FastExprConverter) Manager() (*ast.ASTManager, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	return c.ast, nil
}

// Engine returns the borrowed concrete subpaving engine.
func (c *FastExprConverter) Engine() (*FastEngine, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	return c.engine, nil
}

// Manager returns the borrowed AST manager.
func (c *FixedExprConverter) Manager() (*ast.ASTManager, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	return c.ast, nil
}

// Engine returns the borrowed concrete subpaving engine.
func (c *FixedExprConverter) Engine() (*FixedEngine, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	return c.engine, nil
}

// Manager returns the borrowed AST manager.
func (c *RationalExprConverter) Manager() (*ast.ASTManager, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	return c.ast, nil
}

// Engine returns the borrowed concrete subpaving engine.
func (c *RationalExprConverter) Engine() (*RationalEngine, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	return c.engine, nil
}
