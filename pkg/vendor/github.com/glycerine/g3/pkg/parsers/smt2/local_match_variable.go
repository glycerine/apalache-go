package smt2

// Source: src/parsers/smt2/smt2parser.cpp:1499-1510.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

var errMatchVariableState = errors.New("smt2 parser: match variable state is unavailable")

// bindMatchVariable is the non-constructor name branch of parse_match_pattern.
// The expression stack owns the variable; the local table borrows it until
// the enclosing match-case scope ends. Source post-increments num_bindings,
// so the stored level is the value before the increment.
func (e *smt2LocalEnv) bindMatchVariable(m *ast.ASTManager, name symbol.Symbol, sort ast.ASTHandle, numBindings *uint32, exprs *localExprStack) error {
	if e == nil || m == nil || numBindings == nil || exprs == nil {
		return errMatchVariableState
	}
	v, err := m.MkVar(0, sort)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(v)
	if err != nil {
		return err
	}
	exprs.pushOwned(ref)
	e.insert(name, smt2Local{term: v, level: *numBindings})
	*numBindings++
	return nil
}
