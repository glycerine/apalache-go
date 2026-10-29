package smtcore

// Source: src/smt/smt_context_inv.cpp context::validate_unsat_core and
// src/smt/smt_context.cpp context::mk_unsat_core.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// Validation runs after the temporary assumption scope has been removed.
// The fresh context receives assertions and the extracted core as formulas,
// so its ordinary assumption-free check does not recursively validate a core.
func (k *SMTKernel) validateUnsatCore() (resultErr error) {
	if !k.smtState.CoreValidate {
		return nil
	}
	check, err := newSMTSolver(k.manager, &k.assertedParams, "", SMTSolverConfig{
		Params: k.paramContext, Globals: k.paramGlobals, ModelEvaluator: k.modelConfig,
	}, k.smtParams)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, check.Close()) }()
	for _, row := range k.assertions {
		if err = check.Assert(row.Formula); err != nil {
			return err
		}
	}
	for _, expression := range k.unsatCore {
		if err = check.Assert(expression); err != nil {
			return err
		}
	}
	status, err := check.CheckSatCore(nil)
	if err != nil {
		return err
	}
	if status != lbool.False {
		return z3exception.NewDefaultException("Core could not be validated")
	}
	return nil
}
