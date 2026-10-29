package ast

// Source: src/ast/ast.h; src/util/id_gen.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (m *ASTManager) Budget() *arena.Budget { return m.BudgetField }
func (m *ASTManager) IsOutOfMemory() bool   { return m.ExprIDs.IsOutOfMemory() }

// NumeralService returns the borrowed rational service used by AST numerals.
func (m *ASTManager) NumeralService() *numeral.RationalService {
	if m == nil || m.Self != m || m.Closed {
		return nil
	}
	return m.numerals
}

func (m *ASTManager) ResourceLimit() *control.Reslimit { return &m.Limit }
func (m *ASTManager) IsTrue(h ASTHandle) bool          { return h == m.external(m.trueApp) }
func (m *ASTManager) IsFalse(h ASTHandle) bool         { return h == m.external(m.falseApp) }
