package smtcore

// Source: src/smt/asserted_formulas.h cheap_quant_fourier_motzkin;
// src/smt/asserted_formulas.cpp simplify_fmls, reduce.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// Source setup_AUFLIA / setup_AUFLIRA enable this hidden preprocessor field.
// Ordinary checks and push select CFG_LOGIC. A fresh tactic's SetupAndCheck
// selects static formula features for automatic setup_unknown.
func (k *SMTKernel) setupAssertedBounds(useStaticFeatures bool) error {
	if k.preprocessingSetupDone || k.smtParams == nil {
		return nil
	}
	if !k.smtState.AutoConfig {
		if err := k.setupBasicArithmetic(); err != nil {
			return err
		}
		if err := k.setupArrayBackend(); err != nil {
			return err
		}
		if err := k.setupSequenceBackend(useStaticFeatures); err != nil {
			return err
		}
		k.setupAssertedPolicies()
		k.preprocessingSetupDone = true
		return nil
	}
	if !useStaticFeatures && smtLogicUsesUnknownSetup(k.setupLogic) {
		if err := k.setupBasicArithmetic(); err != nil {
			return err
		}
	}
	if err := k.setupBitVectorLogicPolicy(); err != nil {
		return err
	}
	if err := k.setupIntegerDifferencePolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupRealDifferencePolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupUFIntegerDifferencePolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupIntegerLogicPolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupRealLogicPolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupSimpleLogicPolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupArrayLogicPolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupQuantifiedLogicPolicy(useStaticFeatures); err != nil {
		return err
	}
	if err := k.setupUnknownStaticPolicy(useStaticFeatures); err != nil {
		return err
	}
	enabled := false
	switch k.setupLogic {
	case "AUFLIA", "AUFLIRA", "AUFNIRA", "AUFLIA+", "AUFLIA-", "AUFLIRA+", "AUFLIRA-", "AUFNIRA+", "AUFNIRA-", "UFNIA":
		enabled = true
	case "UFLRA":
		// setup_auto_config routes this logic through setup_unknown(st).
		enabled = !useStaticFeatures
	}
	if !enabled && useStaticFeatures && (smtLogicUsesUnknownSetup(k.setupLogic) || k.setupLogic == "UFLRA") {
		for _, row := range k.assertions {
			has, err := k.manager.HasQuantifiers(row.Formula)
			if err != nil {
				return err
			}
			if has {
				enabled = true
				break
			}
		}
	}
	if enabled {
		k.preprocessState.EliminateBounds = true
		k.smtParams.preprocess.state.EliminateBounds = true
	}
	if err := k.setupArrayBackend(); err != nil {
		return err
	}
	if err := k.setupSequenceBackend(useStaticFeatures); err != nil {
		return err
	}
	k.setupAssertedPolicies()
	k.preprocessingSetupDone = true
	return nil
}

func (k *SMTKernel) eliminatePendingBounds() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess || !k.preprocessState.EliminateBounds {
		return nil
	}
	if !k.assertedHasQuantifiers {
		return nil
	}
	proc, err := rewriter.NewElimBounds(k.manager)
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
