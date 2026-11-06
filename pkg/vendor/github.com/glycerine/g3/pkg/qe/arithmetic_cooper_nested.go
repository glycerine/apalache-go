package qe

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *ArithmeticCooperRuntime) rewriteNested(processor *ArithmeticBoundsProcessor, result ast.ASTRef) error {
	for _, constraint := range processor.Divisibilities(true) {
		if err := r.rewriteOneNested(constraint, result); err != nil {
			return err
		}
	}
	return nil
}

func (r *ArithmeticCooperRuntime) rewriteOneNested(constraint *ArithmeticDivisibility, result ast.ASTRef) error {
	fresh, err := r.boundedVariable(new(big.Int).Abs(constraint.Divisor.Num()))
	if err != nil {
		return err
	}

	upper := new(big.Int).Sub(new(big.Int).Abs(constraint.Divisor.Num()), big.NewInt(1))
	bounded, err := r.boundedResidue(fresh, upper)
	if err != nil {
		return err
	}
	if err = r.plugin.appendConstraint(false, bounded); err != nil {
		return err
	}
	modAtom, err := r.plugin.manager.RefGet(constraint.Atom)
	if err != nil {
		return err
	}
	numerator, err := r.plugin.manager.GetArg(modAtom, 0)
	if err != nil {
		return err
	}
	difference, err := r.plugin.manager.MkArithmeticSub(numerator, fresh)
	if err != nil {
		return err
	}
	originalCongruence, err := r.modZero(difference, constraint.Divisor)
	if err != nil {
		return err
	}
	if err = r.plugin.appendConstraint(false, originalCongruence); err != nil {
		return err
	}
	if err = r.plugin.replacer.ApplySubstitution(modAtom, fresh, result); err != nil {
		return err
	}

	term, err := r.plugin.manager.RefGet(constraint.Term)
	if err != nil {
		return err
	}
	// Retain the source routing exactly: the nested residue is scaled by the
	// extracted coefficient, while the common Cooper residue is subtracted
	// from the non-variable term.
	scaled, err := r.plugin.util.scale(constraint.Coefficient, fresh)
	if err != nil {
		return err
	}
	scaledHandle, err := r.plugin.manager.RefGet(scaled)
	if err != nil {
		_ = r.plugin.manager.DelRef(&scaled)
		return err
	}
	rest, err := r.plugin.manager.MkArithmeticSub(term, r.residue)
	if err != nil {
		_ = r.plugin.manager.DelRef(&scaled)
		return err
	}
	sum, err := r.plugin.manager.MkArithmeticAdd(scaledHandle, rest)
	closeErr := r.plugin.manager.DelRef(&scaled)
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	congruence, err := r.modZero(sum, constraint.Divisor)
	if err != nil {
		return err
	}
	return r.conjoin(result, congruence)
}
