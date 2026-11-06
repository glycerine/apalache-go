package smt2

// Source: src/parsers/smt2/smt2parser.cpp:1869-1900; src/util/symbol_table.h:142-182.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

var errLetDeclarations = errors.New("malformed let expression")

// bindLetLocals is the declaration half of pop_let_frame. All initializer
// expressions are already on the owning expression stack before the new scope
// starts, so an initializer cannot see another binding from the same let.
// The table borrows these handles until the body frame closes its scope.
func (e *smt2LocalEnv) bindLetLocals(m *ast.ASTManager, names []symbol.Symbol, exprs *localExprStack, exprStart int, numBindings uint32) error {
	if e == nil || exprs == nil || exprStart < 0 || exprStart > len(exprs.refs) || len(names) != len(exprs.refs)-exprStart {
		return errLetDeclarations
	}
	// Read every initializer before mutating the table. This also rejects a
	// stale stack reference without exposing a partially bound let scope.
	terms := make([]ast.ASTHandle, len(names))
	for i := range names {
		term, err := m.RefGet(exprs.refs[exprStart+i])
		if err != nil {
			return err
		}
		terms[i] = term
	}
	e.beginScope()
	for i, name := range names {
		e.insert(name, smt2Local{term: terms[i], level: numBindings})
	}
	return nil
}

// finishLetLocalExprs is the result-preserving half of pop_let_frame. The last
// stack reference owns the body result; declaration references are released
// before their borrowed local names leave scope, then the result is returned
// to the stack. A stopped release keeps the result and scope for retry.
func (e *smt2LocalEnv) finishLetLocalExprs(m *ast.ASTManager, exprs *localExprStack, exprStart int) error {
	if e == nil || len(e.limits) == 0 || exprs == nil || exprStart < 0 || exprStart >= len(exprs.refs) {
		return errLetDeclarations
	}
	last := len(exprs.refs) - 1
	result := exprs.refs[last]
	if _, err := m.RefGet(result); err != nil {
		return err
	}
	exprs.refs[last] = ast.ASTRef{}
	exprs.refs = exprs.refs[:last]
	if err := exprs.shrink(m, exprStart); err != nil {
		exprs.pushOwned(result)
		return err
	}
	if err := e.endScopeWithOwningCleanup(); err != nil {
		exprs.pushOwned(result)
		return err
	}
	exprs.pushOwned(result)
	return nil
}
