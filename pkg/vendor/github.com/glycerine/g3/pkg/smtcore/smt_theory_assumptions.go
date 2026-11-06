package smtcore

// Source: src/smt/smt_context.cpp add_theory_assumptions and the owning
// expr_ref_vector used by check/setup_and_check.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Callbacks append borrowed expressions; the context owns the check-local
// vector, including its external assumptions, until validation finishes.
func (k *SMTKernel) addTheoryAssumptions(external []ast.ASTHandle) (all []ast.ASTHandle, refs []ast.ASTRef, result error) {
	all = append([]ast.ASTHandle(nil), external...)
	pin := func(values []ast.ASTHandle) error {
		for _, value := range values {
			ref, err := k.manager.NewRef(value)
			if err != nil {
				return err
			}
			refs = append(refs, ref)
		}
		return nil
	}
	if err := pin(all); err != nil {
		return all, refs, err
	}
	for _, theory := range k.internalizer.theoryOrder {
		start := len(all)
		var err error
		all, err = theory.AddTheoryAssumptions(all)
		if err != nil {
			return all, refs, err
		}
		if err := pin(all[start:]); err != nil {
			return all, refs, err
		}
	}
	return all, refs, nil
}

func (k *SMTKernel) releaseTheoryAssumptions(refs []ast.ASTRef) (result error) {
	for i := range refs {
		result = errors.Join(result, k.manager.DelRef(&refs[i]))
	}
	return result
}
