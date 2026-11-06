package smtcore

// Source: src/smt/asserted_formulas.h pull_nested_quantifiers and
// src/smt/asserted_formulas.cpp simplify_fmls::operator().

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/ast/normal_forms"
)

func (k *SMTKernel) pullPendingNestedQuantifiers() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess || !k.preprocessState.PullNestedQuantifiers || !k.assertedHasQuantifiers {
		return nil
	}
	proc, err := normal_forms.NewPullNestedQuantifiers(k.manager)
	if proc != nil {
		defer func() { result = errors.Join(result, proc.Close()) }()
	}
	if err != nil {
		return err
	}
	out, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&out)) }()
	proof, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&proof)) }()
	batch, err := macros.NewFormulaBatch(k.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, batch.Close()) }()
	proofs, err := k.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	for _, assertion := range k.assertions[k.assertedHead:] {
		if err := proc.RewriteWithProof(assertion.Formula, out, proof); err != nil {
			return err
		}
		value, err := k.manager.RefGet(out)
		if err != nil {
			return err
		}
		row := macros.Formula{Expression: value, Proof: assertion.Proof}
		if value != assertion.Formula {
			if proofs {
				rewrite, err := k.manager.RefGet(proof)
				if err != nil {
					return err
				}
				if rewrite.IsNull() {
					rewrite, err = k.manager.MkProofRewrite(assertion.Formula, value)
					if err != nil {
						return err
					}
				}
				row.Proof, err = k.manager.MkProofModusPonens(assertion.Proof, rewrite)
				if err != nil {
					return err
				}
			}
		}
		if err = k.appendTransformedAssertion(batch, assertion, row.Expression, row.Proof); err != nil {
			return err
		}
		if canceled, err := k.manager.Canceled(); canceled || err != nil {
			return err
		}
	}
	return k.finishAssertedPass(batch, false)
}
