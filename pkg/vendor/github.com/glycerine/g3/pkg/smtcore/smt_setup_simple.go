package smtcore

// Source: src/smt/smt_setup.cpp setup_QF_UF overloads and setup_QF_UFLRA.

import (
	"errors"
	"fmt"
)

func (k *SMTKernel) setupSimpleLogicPolicy(useStaticFeatures bool) (result error) {
	switch k.setupLogic {
	case "QF_UF", "QF_DT":
		if useStaticFeatures && k.setupLogic == "QF_UF" {
			features, err := k.setupStaticFeatures(true)
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, features.Close()) }()
			if features.NumArithInequalities > 0 || features.NumArithTerms > 0 || features.NumArithEqualities > 0 {
				// Retain the source exception's spelling.
				return fmt.Errorf("Benchmark constains arithmetic, but specified loging does not support it.")
			}
		}
		k.smtParams.state.RestartStrategy = 2       // RS_LUBY
		k.smtParams.state.PhaseSelection = 4        // PS_CACHING_CONSERVATIVE2
		k.smtParams.state.RandomInitialActivity = 2 // IA_RANDOM
	case "QF_UFLRA":
		k.smtParams.arith.state.Reflect = false
		k.arithState.Reflect = false
	default:
		return nil
	}
	k.smtParams.state.RelevancyLvl = 0
	k.smtParams.preprocess.state.NNFCNF = false
	if err := k.applySimpleSetupPolicy(); err != nil {
		return err
	}
	if k.setupLogic == "QF_UFLRA" {
		return k.installRealLRA()
	}
	return nil
}

func (k *SMTKernel) applySimpleSetupPolicy() error {
	k.smtState = k.smtParams.state
	k.preprocessState = k.smtParams.preprocess.state
	k.relevancy.enabled = k.smtState.RelevancyLvl > 0
	k.internalizer.relevancyLevel = k.smtState.RelevancyLvl
	// First-push setup precedes Boolean variable creation, so the selected
	// random initial-activity policy must reach the internalizer immediately.
	if err := k.internalizer.configureBooleanHeuristics(k.smtState); err != nil {
		return err
	}
	k.assertedPhaseReady = false
	return k.setAssertedEliminateAnd(false)
}
