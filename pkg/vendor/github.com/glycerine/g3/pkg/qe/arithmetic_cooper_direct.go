package qe

import "github.com/glycerine/g3/pkg/ast"

func (r *ArithmeticCooperRuntime) rewriteDirect(processor *ArithmeticBoundsProcessor, result ast.ASTRef) error {
	for _, constraint := range processor.Divisibilities(false) {
		term, err := r.plugin.manager.RefGet(constraint.Term)
		if err != nil {
			return err
		}
		scaled, err := r.plugin.util.scale(constraint.Coefficient, r.residue)
		if err != nil {
			return err
		}
		scaledHandle, err := r.plugin.manager.RefGet(scaled)
		if err != nil {
			_ = r.plugin.manager.DelRef(&scaled)
			return err
		}
		sum, err := r.plugin.manager.MkArithmeticAdd(scaledHandle, term)
		closeErr := r.plugin.manager.DelRef(&scaled)
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		replacement, err := r.modZero(sum, constraint.Divisor)
		if err != nil {
			return err
		}
		atom, err := r.plugin.manager.RefGet(constraint.Atom)
		if err != nil {
			return err
		}
		if err = r.plugin.replacer.ApplySubstitution(atom, replacement, result); err != nil {
			return err
		}
		if err = r.equate(atom, replacement); err != nil {
			return err
		}
	}
	return nil
}

var _ ast.ASTHandle
