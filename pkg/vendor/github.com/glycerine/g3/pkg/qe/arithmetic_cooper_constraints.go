package qe

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *ArithmeticCooperRuntime) baseConstraints(result ast.ASTRef) error {
	upper := new(big.Int).Sub(new(big.Int).Set(r.common), big.NewInt(1))
	bounded, err := r.boundedResidue(r.residue, upper)
	if err != nil {
		return err
	}
	if err = r.plugin.appendConstraint(false, bounded); err != nil {
		return err
	}
	if err = r.conjoin(result, bounded); err != nil {
		return err
	}
	difference, err := r.plugin.manager.MkArithmeticSub(r.variable, r.residue)
	if err != nil {
		return err
	}
	divisor := new(big.Rat).SetInt(r.common)
	divides, err := r.modZero(difference, divisor)
	if err != nil {
		return err
	}
	return r.plugin.appendConstraint(false, divides)
}

var _ ast.ASTHandle
