package smtcore

// Source: src/smt/smt_setup.cpp setup_QF_AX and setup_QF_AUFLIA overloads.

import (
	"errors"
	"fmt"
)

func (k *SMTKernel) setupArrayLogicPolicy(useStaticFeatures bool) (result error) {
	if k.setupLogic != "QF_AX" && k.setupLogic != "QF_AUFLIA" {
		return nil
	}
	features, err := k.setupStaticFeatures(useStaticFeatures)
	if err != nil {
		return err
	}
	if features != nil {
		defer func() { result = errors.Join(result, features.Close()) }()
		if k.setupLogic == "QF_AUFLIA" && features.HasReal {
			return fmt.Errorf("Benchmark has real variables but it is marked as QF_AUFLIA (arrays, uninterpreted functions and linear integer arithmetic).")
		}
	}
	k.smtParams.array.state.Mode = 1 // AR_SIMPLE
	k.smtParams.preprocess.state.NNFCNF = false
	if k.setupLogic == "QF_AUFLIA" && features == nil {
		k.smtParams.state.RelevancyLvl = 2
		k.smtParams.state.RestartStrategy = 0 // RS_GEOMETRIC
		k.smtParams.state.RestartFactor = 1.5
		k.smtParams.state.PhaseSelection = 4 // PS_CACHING_CONSERVATIVE2
	}
	if features != nil {
		if features.NumClauses == features.NumUnits {
			k.smtParams.state.RelevancyLvl = 0
			k.smtParams.state.PhaseSelection = 0 // PS_ALWAYS_FALSE
		} else if k.setupLogic == "QF_AX" {
			k.smtParams.state.RelevancyLvl = 2
		} else {
			k.smtParams.state.RelevancyLvl = 0
			k.smtParams.state.RestartStrategy = 0 // RS_GEOMETRIC
			k.smtParams.state.RestartFactor = 1.5
			k.smtParams.state.PhaseSelection = 4        // PS_CACHING_CONSERVATIVE2
			k.smtParams.state.RandomInitialActivity = 0 // IA_ZERO
		}
	}
	return k.applySimpleSetupPolicy()
}
