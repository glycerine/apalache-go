package qe

import "github.com/glycerine/g3/pkg/ast"

func (r *ArithmeticCooperRuntime) equate(original, replacement ast.ASTHandle) error {
	forward, err := r.plugin.manager.MkImplies(original, replacement)
	if err != nil {
		return err
	}
	backward, err := r.plugin.manager.MkImplies(replacement, original)
	if err != nil {
		return err
	}
	if err = r.plugin.appendConstraint(false, forward); err != nil {
		return err
	}
	return r.plugin.appendConstraint(false, backward)
}
