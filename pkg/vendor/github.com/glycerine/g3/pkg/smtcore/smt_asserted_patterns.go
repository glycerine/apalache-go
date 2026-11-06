package smtcore

// Source: src/smt/asserted_formulas.h pattern_inference_fn and
// src/smt/asserted_formulas.cpp simplify_fmls::operator(), reduce.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
)

func (k *SMTKernel) inferPendingPatterns() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess || !k.smtState.Ematching || k.patternInference == nil {
		return nil
	}
	if !k.assertedHasQuantifiers {
		return nil
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
	for _, row := range k.assertions[k.assertedHead:] {
		if proofs {
			err = k.patternInference.RewriteWithProof(row.Formula, out, proof)
		} else {
			err = k.patternInference.Rewrite(row.Formula, out)
		}
		if err != nil {
			return err
		}
		if err = k.appendSimplifiedAssertion(batch, row, out, proof); err != nil {
			return err
		}
		if canceled, err := k.manager.Canceled(); canceled || err != nil {
			return err
		}
	}
	// Source inference has no reduce_and_solve post-operation.
	return k.finishAssertedPass(batch, false)
}
