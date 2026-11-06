package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util equation solving.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type arithmeticEquationTerm struct {
	coefficient *big.Rat
	expression  ast.ASTHandle
}

type arithmeticEquation struct {
	constant     *big.Rat
	coefficients []*big.Rat
	opaque       []arithmeticEquationTerm
}

// ArithmeticEquationElimination owns the rewritten formula, definition, and
// any auxiliary integer variables introduced by the Omega step.
type ArithmeticEquationElimination struct {
	self          *ArithmeticEquationElimination
	manager       *ast.ASTManager
	VariableIndex uint32
	Formula       ast.ASTRef
	Definition    ast.ASTRef
	Fresh         []ast.ASTRef
	closed        bool
}

func (e *ArithmeticEquationElimination) Close() (result error) {
	if e == nil || e.self != e || e.manager == nil {
		return ast.ErrASTHandle
	}
	if e.closed {
		return nil
	}
	for len(e.Fresh) > 0 {
		last := len(e.Fresh) - 1
		if err := e.manager.DelRef(&e.Fresh[last]); err != nil {
			return errors.Join(result, err)
		}
		e.Fresh = e.Fresh[:last]
	}
	if e.Definition != (ast.ASTRef{}) {
		result = errors.Join(result, e.manager.DelRef(&e.Definition))
	}
	if e.Formula != (ast.ASTRef{}) {
		result = errors.Join(result, e.manager.DelRef(&e.Formula))
	}
	if result == nil {
		e.closed = true
	}
	return result
}
