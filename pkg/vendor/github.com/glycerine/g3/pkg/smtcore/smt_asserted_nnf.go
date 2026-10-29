package smtcore

// Source: src/smt/asserted_formulas.cpp asserted_formulas::nnf_cnf.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/ast/normal_forms"
)

func (k *SMTKernel) normalizePendingAssertions() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess {
		return nil
	}
	apply := k.preprocessState.NNFCNF || (k.assertedHasQuantifiers &&
		k.internalizer.quantifierManager != nil && k.internalizer.quantifierManager.mbqi)
	if !apply {
		return nil
	}
	// The C++ caller supplies an empty local parameter set, while the NNF
	// constructor still reads the global nnf module's defaults/overrides.
	empty := k.paramContext.EmptyValue()
	proc, err := normal_forms.NewNNFFromParams(k.manager, k.definedNames, k.paramContext, k.paramGlobals, &empty)
	if proc != nil {
		defer func() { result = errors.Join(result, proc.Close()) }()
	}
	if err != nil {
		return err
	}
	batch, err := macros.NewFormulaBatch(k.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, batch.Close()) }()
	if err = k.traceAssertedBody("nnf_bug", "nnf_cnf", 359, func(out io.Writer) error {
		_, traceErr := fmt.Fprintf(out, "i: %d sz: %d\n", k.assertedHead, len(k.assertions))
		return traceErr
	}); err != nil {
		return err
	}
	for _, assertion := range k.assertions[k.assertedHead:] {
		if err = k.traceAssertedExpression("nnf_bug", "nnf_cnf", 362, "processing:\n", assertion.Formula); err != nil {
			return err
		}
		if canceled, err := k.appendNormalizedAssertion(proc, batch, assertion); canceled || err != nil {
			return err
		}
	}
	return k.swapPendingFormulaBatch(batch)
}

func (k *SMTKernel) appendNormalizedAssertion(proc *normal_forms.NNF, batch *macros.FormulaBatch, assertion SMTKernelAssertion) (canceled bool, result error) {
	out, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&out)) }()
	proof, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&proof)) }()
	var definitions, definitionProofs []ast.ASTRef
	defer func() {
		for i := range definitionProofs {
			result = errors.Join(result, k.manager.DelRef(&definitionProofs[i]))
		}
		for i := range definitions {
			result = errors.Join(result, k.manager.DelRef(&definitions[i]))
		}
	}()
	if err = proc.Transform(assertion.Formula, &definitions, &definitionProofs, out, proof); err != nil {
		return false, err
	}
	if canceled, err := k.manager.Canceled(); canceled || err != nil {
		return canceled, err
	}
	proofs, err := k.manager.ProofsEnabled()
	if err != nil {
		return false, err
	}
	for i, definition := range definitions {
		value, err := k.manager.RefGet(definition)
		if err != nil {
			return false, err
		}
		var p ast.ASTHandle
		if proofs {
			p, err = k.manager.RefGet(definitionProofs[i])
			if err != nil {
				return false, err
			}
		}
		if canceled, err := k.appendNNFRewritten(batch, value, p); canceled || err != nil {
			return canceled, err
		}
	}
	value, err := k.manager.RefGet(out)
	if err != nil {
		return false, err
	}
	var p ast.ASTHandle
	if proofs {
		rewrite, err := k.manager.RefGet(proof)
		if err != nil {
			return false, err
		}
		p, err = k.manager.MkProofModusPonens(assertion.Proof, rewrite)
		if err != nil {
			return false, err
		}
		// Source push_todo_prs owns this composed proof while the theory
		// rewriter processes the normalized main formula.
		held, err := k.manager.NewRef(p)
		if err != nil {
			return false, err
		}
		defer func() { result = errors.Join(result, k.manager.DelRef(&held)) }()
	}
	return k.appendNNFRewritten(batch, value, p)
}

// NNF checks cancellation both after normalization and after rewriting each
// introduced definition/main row, before pushing it into the local vector.
func (k *SMTKernel) appendNNFRewritten(batch *macros.FormulaBatch, formula, proof ast.ASTHandle) (canceled bool, result error) {
	value, p, err := k.preprocessAssertion(formula, proof)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, k.manager.DecRef(value), k.manager.DecRef(p)) }()
	if canceled, err := k.manager.Canceled(); canceled || err != nil {
		return canceled, err
	}
	return false, k.pushAssertionInto(value, p, batch)
}
