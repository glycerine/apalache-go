package aig

// Source: src/tactic/aig/aig.cpp:1488-1527; src/tactic/aig/aig.h:63.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// MkAIGGoal converts and conjoins a goal's current formulas. One expression
// converter spans every formula, so shared source AST nodes reuse their AIG
// variables and cached subexpressions exactly as in the source overload.
func (m *Manager) MkAIGGoal(g *tactic.Goal) (out *Ref, err error) {
	if err = m.open(); err != nil {
		return nil, err
	}
	if g == nil || g.Manager() != m.ast {
		return nil, ErrOwner
	}
	size, err := g.Size()
	if err != nil {
		return nil, err
	}
	c := exprConverter{manager: m, cache: make(map[ast.ASTHandle]*Ref)}
	acc, err := m.True()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, c.cleanup())
		if acc != nil {
			err = errors.Join(err, acc.Close())
		}
		if err != nil && out != nil {
			err = errors.Join(err, out.Close())
			out = nil
		}
	}()
	for i := uint32(0); i < size; i++ {
		formula, e := g.Form(i)
		if e != nil {
			return nil, e
		}
		piece, e := c.convertOne(formula)
		if e != nil {
			return nil, e
		}
		next, e := m.And(acc, piece)
		released := errors.Join(acc.Close(), piece.Close())
		acc = next
		if e != nil || released != nil {
			return nil, errors.Join(e, released)
		}
	}
	out, acc = acc, nil
	return out, nil
}
