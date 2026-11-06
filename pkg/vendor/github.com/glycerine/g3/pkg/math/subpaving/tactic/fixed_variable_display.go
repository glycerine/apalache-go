package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:32-49;
// concrete inverse expr2var display for MPFX arithmetic variables.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/subpaving"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (c *FixedExprConverter) displayMappedVariable(w io.Writer, h ast.ASTHandle, _ *symbol.Manager) error {
	return writeSubpavingSMT2Expression(w, c.ast, h)
}

// VariableDisplay captures the source inverse mapping and replaces the prior
// display procedure's expression references before publishing the new one.
func (c *FixedExprConverter) VariableDisplay(symbols *symbol.Manager) (subpaving.FixedVariableDisplay, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	if symbols == nil {
		return nil, arena.ErrInvalidSlot
	}
	entries, err := c.mapping.Entries()
	if err != nil {
		return nil, err
	}
	inverse := make(map[uint32]ast.ASTHandle, len(entries))
	pins, err := pinExprMapping(c.ast, entries)
	if err != nil {
		return nil, err
	}
	if err := releaseExprPins(c.ast, &c.displayOrder); err != nil {
		c.displayOrder = append(c.displayOrder, pins...)
		return nil, err
	}
	c.displayOrder = pins
	for _, entry := range entries {
		inverse[entry.Variable] = entry.Expr
	}
	return func(w io.Writer, variable uint32) error {
		if err := c.check(); err != nil {
			return err
		}
		if w == nil {
			return arena.ErrInvalidSlot
		}
		if h, found := inverse[variable]; found {
			return c.displayMappedVariable(w, h, symbols)
		}
		return subpavingWriteDisplay(w, "k!"+strconv.FormatUint(uint64(variable), 10))
	}, nil
}
