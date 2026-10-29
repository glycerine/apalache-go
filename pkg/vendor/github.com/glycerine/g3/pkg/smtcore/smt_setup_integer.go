package smtcore

// Source: src/smt/smt_setup.cpp setup_QF_LIA() and setup_QF_LIA(static_features).

import (
	"errors"
)

func (k *SMTKernel) setupIntegerLogicPolicy(useStaticFeatures bool) (result error) {
	if k.setupLogic != "QF_LIA" && k.setupLogic != "QF_BVRE" {
		return nil
	}
	if k.setupLogic == "QF_BVRE" {
		// setup_QF_BVRE calls the logic-only QF_LIA overload.
		useStaticFeatures = false
	}
	features, err := k.arithmeticSetupFeatures(useStaticFeatures)
	if err != nil {
		return err
	}
	if features != nil {
		defer func() { result = errors.Join(result, features.Close()) }()
	}
	smt := k.smtParams.state
	arithmetic := k.smtParams.arith.state
	preprocess := k.smtParams.preprocess.state
	smt.RelevancyLvl = 0
	arithmetic.Eq2Ineq = true
	arithmetic.Reflect = false
	arithmetic.PropagateEqs = false
	preprocess.NNFCNF = false
	if features != nil {
		switch {
		case features.MaxITETreeDepth > 50:
			arithmetic.Eq2Ineq = false
			preprocess.PullCheapITE = true
			arithmetic.PropagateEqs = true
			smt.RelevancyLvl = 2
			smt.RelevancyLemma = false
		case features.NumClauses == features.NumUnits:
			arithmetic.GCDTest = false
			arithmetic.BranchCutRatio = 4
			smt.RelevancyLvl = 2
			arithmetic.Eq2Ineq = true
			preprocess.EliminateTermITE = true
		default:
			preprocess.EliminateTermITE = true
			smt.PhaseSelection = 2 // PS_CACHING
			smt.RestartAdaptive = false
			smt.RestartStrategy = 0 // RS_GEOMETRIC
			smt.RestartFactor = 1.5
		}
		if features.NumBinClauses+features.NumUnits == features.NumClauses && features.CNF {
			large, err := k.manager.NumeralService().GreaterInt(&features.ArithKSum, 100000)
			if err != nil {
				return err
			}
			if large {
				arithmetic.BoundProp = 0 // BP_NONE
				arithmetic.StrongerLemmas = false
			}
		}
	}
	return k.applyArithmeticSetup(smt, arithmetic, preprocess)
}
