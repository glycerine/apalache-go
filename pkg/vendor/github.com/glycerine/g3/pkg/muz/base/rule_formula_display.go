package base

// Source: src/muz/base/dl_rule.cpp rule_manager::display_smt2.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// WriteRuleFormula writes the rule's quantified formula with the concrete AST
// expression printer. The writer is borrowed.
func (m *RuleManager) WriteRuleFormula(out io.Writer, rule *Rule) (resultErr error) {
	if out == nil {
		return io.ErrClosedPipe
	}
	formula, err := m.RuleFormula(rule)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&formula)) }()
	handle, err := m.manager.RefGet(formula)
	if err != nil {
		return err
	}
	return writeRuleFormula(m.manager, out, handle)
}

func writeRuleFormula(manager *ast.ASTManager, out io.Writer, formula ast.ASTHandle) error {
	printer, err := ast.NewSimpleASTPrinterContext(manager, out, out)
	if err != nil {
		return err
	}
	return printer.DisplayExpr(out, formula, 0)
}

// WriteFormula writes this rule using its owning manager.
func (r *Rule) WriteFormula(out io.Writer) error {
	if out == nil {
		return io.ErrClosedPipe
	}
	state, err := r.open()
	if err != nil {
		return err
	}
	formula, err := r.ToFormula()
	if err != nil {
		return err
	}
	defer state.manager.DelRef(&formula)
	handle, err := state.manager.RefGet(formula)
	if err != nil {
		return err
	}
	return writeRuleFormula(state.manager, out, handle)
}
