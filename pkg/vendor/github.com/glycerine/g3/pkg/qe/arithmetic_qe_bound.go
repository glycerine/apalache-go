package qe

// Source: src/qe/qe_arith_plugin.cpp bound.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticBoundDirection uint8

const (
	ArithmeticLowerBound ArithmeticBoundDirection = iota
	ArithmeticUpperBound
)

// ArithmeticBound owns Term and Atom. Coefficient is detached exact data.
type ArithmeticBound struct {
	manager     *ast.ASTManager
	Term        ast.ASTRef
	Atom        ast.ASTRef
	Coefficient *big.Rat
	Strict      bool
	Direction   ArithmeticBoundDirection
}

func (b *ArithmeticBound) Close() error {
	if b == nil || b.manager == nil {
		return ast.ErrASTHandle
	}
	err := errors.Join(b.manager.DelRef(&b.Atom), b.manager.DelRef(&b.Term))
	if err == nil {
		b.manager = nil
	}
	return err
}
