package qe

import "github.com/glycerine/g3/pkg/ast"

func (r *ArithmeticCooperRuntime) conjoin(result ast.ASTRef, formula ast.ASTHandle) error {
	root, err := r.plugin.manager.RefGet(result)
	if err != nil {
		return err
	}
	combined, err := r.plugin.manager.MkAnd(root, formula)
	if err != nil {
		return err
	}
	return r.plugin.manager.SetRef(result, combined)
}
