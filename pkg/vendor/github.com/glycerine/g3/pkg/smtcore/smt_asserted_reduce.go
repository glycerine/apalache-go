package smtcore

// Source: src/smt/asserted_formulas.cpp set_eliminate_and, reduce_and_solve
// and simplify_fmls; src/smt/asserted_formulas.h distribute_forall_fn.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// Source flush_cache resets the rewriter then restores its borrowed pointer.
// SetSubstitution performs that same reset/reattachment in the native owner.
func (k *SMTKernel) flushAssertedCache() error {
	if k.preprocessor == nil {
		return nil
	}
	return k.preprocessor.SetSubstitution(&k.assertedSubstitution)
}

// Native allocation errors can return before the local source vector is
// swapped. Preserve only conflict state that was already present or belongs to
// published rows; ordinary cancellation retains the source's eager flag.
func (k *SMTKernel) restoreAssertedConflictOnError(previous bool, result *error) {
	if *result == nil {
		return
	}
	k.assertedInconsistent = previous
	for _, row := range k.assertions {
		if k.manager.IsFalse(row.Formula) {
			k.assertedInconsistent = true
			break
		}
	}
}

func (k *SMTKernel) setAssertedEliminateAnd(flag bool) error {
	if !k.preprocess || k.preprocessor == nil || (k.assertedPhaseReady && k.assertedElimAnd == flag) {
		return nil
	}
	settings := []struct {
		name  string
		value bool
	}{
		{"elim_and", flag}, {"arith_ineq_lhs", true}, {"sort_sums", true},
		{"rewrite_patterns", true}, {"eq2ineq", k.arithState.Eq2Ineq},
		{"gcd_rounding", true}, {"expand_select_store", true},
		{"bv_sort_ac", true}, {"som", true},
	}
	if k.preprocessState.PullCheapITE {
		settings = append(settings, struct {
			name  string
			value bool
		}{"pull_cheap_ite", true})
	}
	for _, setting := range settings {
		name := setting.name
		if err := k.paramContext.SetBoolName(&k.assertedParams, &name, setting.value); err != nil {
			return err
		}
	}
	if err := k.preprocessor.UpdateParams(&k.assertedParams); err != nil {
		return err
	}
	if err := k.flushAssertedCache(); err != nil {
		return err
	}
	k.assertedElimAnd, k.assertedPhaseReady = flag, true
	return nil
}

func (k *SMTKernel) reducePendingAssertions() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess || k.preprocessor == nil {
		return nil
	}
	batch, err := macros.NewFormulaBatch(k.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, batch.Close()) }()
	for _, row := range k.assertions[k.assertedHead:] {
		formula, proof, err := k.preprocessAssertion(row.Formula, row.Proof)
		if err != nil {
			return err
		}
		err = k.appendTransformedAssertion(batch, row, formula, proof)
		err = errors.Join(err, k.manager.DecRef(formula), k.manager.DecRef(proof))
		if err != nil {
			return err
		}
		if canceled, err := k.manager.Canceled(); canceled || err != nil {
			return err
		}
	}
	if err := k.swapPendingFormulaBatch(batch); err != nil {
		return err
	}
	return k.traceAssertedState("asserted_formulas", "operator()", 416, false)
}

func (k *SMTKernel) reduceAndSolveAssertions() error {
	// Source IF_IVERBOSE is erased in the selected _EXTERNAL_RELEASE
	// profile; the ordinary IF_VERBOSE pass labels remain enabled.
	if k.preprocessor != nil {
		if err := k.flushAssertedCache(); err != nil {
			return err
		}
	}
	return k.reducePendingAssertions()
}

func (k *SMTKernel) appendTransformedAssertion(batch *macros.FormulaBatch, original SMTKernelAssertion, formula, proof ast.ASTHandle) error {
	if original.Formula == formula {
		return batch.Insert(macros.Formula{Expression: original.Formula, Proof: original.Proof})
	}
	return k.pushAssertionInto(formula, proof, batch)
}

func (k *SMTKernel) finishAssertedPass(batch *macros.FormulaBatch, reduce bool) error {
	if err := k.swapPendingFormulaBatch(batch); err != nil {
		return err
	}
	if err := k.traceAssertedState("asserted_formulas", "operator()", 416, false); err != nil {
		return err
	}
	if reduce {
		return k.reduceAndSolveAssertions()
	}
	return nil
}

func (k *SMTKernel) distributePendingUniversals() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess || !k.preprocessState.DistributeForall {
		return nil
	}
	if !k.assertedHasQuantifiers {
		return nil
	}
	proc, err := rewriter.NewDistributeForall(k.manager, k.paramContext, k.paramGlobals)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, proc.Close()) }()
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
	for _, assertion := range k.assertions[k.assertedHead:] {
		if err := proc.Apply(assertion.Formula, out); err != nil {
			return err
		}
		if err := k.appendSimplifiedAssertion(batch, assertion, out, proof); err != nil {
			return err
		}
		if canceled, err := k.manager.Canceled(); canceled || err != nil {
			return err
		}
	}
	return k.finishAssertedPass(batch, true)
}
