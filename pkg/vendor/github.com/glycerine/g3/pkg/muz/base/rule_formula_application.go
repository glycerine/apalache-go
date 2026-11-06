package base

// Source: src/muz/base/dl_rule.cpp rule_manager::ensure_app and check_app.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrRuleFormula = errors.New("muz/base: invalid rule formula")

// EnsureBooleanApplication returns an owning application for a Boolean
// expression. Quantifiers are represented by (= expression true), matching
// the source invariant that rule heads and tails are applications.
func (m *RuleManager) EnsureBooleanApplication(expression ast.ASTHandle) (ast.ASTRef, error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	kind, err := m.manager.GetKind(expression)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if kind == ast.ASTApp {
		return m.manager.NewRef(expression)
	}
	sort, err := m.manager.GetSort(expression)
	if err != nil {
		return ast.ASTRef{}, err
	}
	boolean, err := m.manager.MkBoolSort()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if sort != boolean {
		return ast.ASTRef{}, fmt.Errorf("%w: expected Boolean expression", ErrRuleFormula)
	}
	truth, err := m.manager.MkTrue()
	if err != nil {
		return ast.ASTRef{}, err
	}
	application, err := m.manager.MkEq(expression, truth)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.manager.NewRef(application)
}

// CheckApplication enforces the application-only rule representation.
func (m *RuleManager) CheckApplication(expression ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	kind, err := m.manager.GetKind(expression)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return fmt.Errorf("%w: expected application", ErrRuleFormula)
	}
	return nil
}
