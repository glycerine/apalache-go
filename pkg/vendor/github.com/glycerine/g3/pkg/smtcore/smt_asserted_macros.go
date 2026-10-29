package smtcore

// Source: src/smt/asserted_formulas.cpp find_macros_core and commit;
// src/smt/smt_model_generator.cpp register_macros.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/model"
)

func (k *SMTKernel) findPendingMacros() (result error) {
	if !k.preprocess || !k.preprocessState.MacroFinder || !k.assertedHasQuantifiers {
		return nil
	}
	inputs := make([]macros.Formula, 0, len(k.assertions)-k.assertedHead)
	for _, assertion := range k.assertions[k.assertedHead:] {
		inputs = append(inputs, macros.Formula{Expression: assertion.Formula, Proof: assertion.Proof})
	}
	finder, err := macros.NewFinder(k.manager, k.assertedMacros)
	if err != nil {
		return err
	}
	batch, err := finder.Apply(inputs)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, batch.Close()) }()
	return k.replacePendingMacroBatch(batch)
}

func (k *SMTKernel) replacePendingMacroBatch(batch *macros.FormulaBatch) error {
	if err := k.swapPendingFormulaBatch(batch); err != nil {
		return err
	}
	return k.reduceAndSolveAssertions()
}

// swapPendingFormulaBatch copies already justified, flattened rows. Source
// swap_asserted_formulas does not push, rewrite or poll cancellation again.
func (k *SMTKernel) swapPendingFormulaBatch(batch *macros.FormulaBatch) (result error) {
	if err := k.assertedSourceAssertion(323, "!inconsistent() || !formulas.empty()", func() (bool, error) {
		return !k.assertedInconsistent || batch != nil && batch.Len() != 0, nil
	}); err != nil {
		return err
	}
	original := k.assertions
	k.assertions = append([]SMTKernelAssertion(nil), original[:k.assertedHead]...)
	defer func() {
		if result != nil {
			for _, assertion := range k.assertions[k.assertedHead:] {
				result = errors.Join(result, k.manager.DecRef(assertion.Proof), k.manager.DecRef(assertion.Formula))
			}
			k.assertions = original
			return
		}
		for _, assertion := range original[k.assertedHead:] {
			result = errors.Join(result, k.manager.DecRef(assertion.Proof), k.manager.DecRef(assertion.Formula))
		}
	}()
	for i := 0; i < batch.Len(); i++ {
		row, err := batch.At(i)
		if err != nil {
			return err
		}
		if err := k.appendAssertion(row.Expression, row.Proof); err != nil {
			return err
		}
	}
	return nil
}

// Discovery flags do not change definitions already removed from the formula
// list. Expand those definitions before committing any later assertions.
func (k *SMTKernel) expandPendingMacros() (result error) {
	if !k.assertedMacros.HasMacros() {
		return nil
	}
	batch, err := macros.NewFormulaBatch(k.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, batch.Close()) }()
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
	for _, assertion := range k.assertions[k.assertedHead:] {
		dependency, err := k.assertedMacros.ExpandMacros(assertion.Formula, assertion.Proof, ast.ASTDependency{}, out, proof)
		if err != nil {
			return err
		}
		row := macros.Formula{Dependency: dependency}
		row.Expression, err = k.manager.RefGet(out)
		if err == nil {
			row.Proof, err = k.manager.RefGet(proof)
		}
		if err == nil {
			err = batch.Insert(row)
		}
		if err = errors.Join(err, k.manager.DependencyDec(dependency)); err != nil {
			return err
		}
	}
	return k.replacePendingMacroBatch(batch)
}

// applyPendingQuasiMacros follows the source repeated quasi-macro pass. Each
// batch owns the formulas until the next pass has acquired its references.
func (k *SMTKernel) applyPendingQuasiMacros() (result error) {
	if !k.preprocess || !k.preprocessState.QuasiMacros || !k.assertedHasQuantifiers {
		return nil
	}
	if err := k.traceAssertedState("before_quasi_macros", "apply_quasi_macros", 338, false); err != nil {
		return err
	}
	proc, err := macros.NewQuasiMacros(k.manager, k.assertedMacros)
	if err != nil {
		return err
	}
	var batch *macros.FormulaBatch
	defer func() {
		if batch != nil {
			result = errors.Join(result, batch.Close())
		}
	}()
	for {
		var inputs []macros.Formula
		for _, assertion := range k.assertions[k.assertedHead:] {
			inputs = append(inputs, macros.Formula{Expression: assertion.Formula, Proof: assertion.Proof})
		}
		next, changed, err := proc.Apply(inputs)
		if err != nil {
			return err
		}
		batch = next
		if !changed {
			break
		}
		if err := k.swapPendingFormulaBatch(batch); err != nil {
			return err
		}
		if err := batch.Close(); err != nil {
			return err
		}
		batch = nil
	}
	if err := k.traceAssertedState("after_quasi_macros", "apply_quasi_macros", 347, false); err != nil {
		return err
	}
	return k.reduceAndSolveAssertions()
}

func (g *SMTModelGenerator) registerAssertedMacros(manager *macros.Manager) (result error) {
	count := 0
	if manager != nil {
		count = manager.NumMacros()
	}
	if err := g.traceModel("model", "register_macros", 512, func() bool { return true }, func(out io.Writer) error {
		_, traceErr := fmt.Fprintf(out, "num. macros: %d\n", count)
		return traceErr
	}); err != nil {
		return err
	}
	if manager == nil || !manager.HasMacros() {
		return nil
	}
	out, err := g.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, g.manager.DelRef(&out)) }()
	for i := 0; i < manager.NumMacros(); i++ {
		declaration, err := manager.MacroInterpretation(i, out)
		if err != nil {
			return err
		}
		definition, err := g.manager.RefGet(out)
		if err != nil {
			return err
		}
		if err = g.traceModel("model", "register_macros", 518, func() bool { return true }, func(traceOut io.Writer) error {
			if traceErr := g.manager.DisplayDeclarationHead(traceOut, declaration); traceErr != nil {
				return traceErr
			}
			if _, traceErr := io.WriteString(traceOut, "\n"); traceErr != nil {
				return traceErr
			}
			if traceErr := g.displayModelExpression(traceOut, definition); traceErr != nil {
				return traceErr
			}
			_, traceErr := io.WriteString(traceOut, "\n")
			return traceErr
		}); err != nil {
			return err
		}
		arity, err := g.manager.GetArity(declaration)
		if err != nil {
			return err
		}
		interpretation, err := model.NewFunctionInterpretation(g.manager, arity)
		if err != nil {
			return err
		}
		if err = interpretation.SetElse(definition); err == nil {
			err = g.model.Core().RegisterFunction(declaration, interpretation)
		}
		if err != nil {
			return errors.Join(err, interpretation.Close())
		}
	}
	return nil
}
