package smtcore

// Source: src/smt/asserted_formulas.h apply_bit2int, max_bv_sharing_fn;
// src/smt/asserted_formulas.cpp simplify_fmls, reduce, push_scope, pop_scope.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (k *SMTKernel) applyPendingBit2Int() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess || !k.preprocessState.SimplifyBitToInt {
		return nil
	}
	proc, err := rewriter.NewBit2Int(k.manager)
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
	for _, row := range k.assertions[k.assertedHead:] {
		if err = proc.RewriteWithProof(row.Formula, out, proof); err != nil {
			return err
		}
		if err = k.appendSimplifiedAssertion(batch, row, out, proof); err != nil {
			return err
		}
		if canceled, err := k.manager.Canceled(); canceled || err != nil {
			return err
		}
	}
	return k.finishAssertedPass(batch, true)
}

func (k *SMTKernel) maximizePendingBVSharing() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess || !k.preprocessState.MaxBVSharing {
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
	for _, row := range k.assertions[k.assertedHead:] {
		if err = k.bvSharing.Rewrite(row.Formula, out, proof); err != nil {
			return err
		}
		if err = k.appendSimplifiedAssertion(batch, row, out, proof); err != nil {
			return err
		}
		if canceled, err := k.manager.Canceled(); canceled || err != nil {
			return err
		}
	}
	// Source max_bv_sharing_fn invokes ordinary reduction without a cache flush.
	if err := k.swapPendingFormulaBatch(batch); err != nil {
		return err
	}
	if err := k.traceAssertedState("asserted_formulas", "operator()", 416, false); err != nil {
		return err
	}
	return k.reducePendingAssertions()
}
