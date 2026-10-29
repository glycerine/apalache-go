package smtcore

// Source: src/smt/smt_setup.cpp setup_unknown(static_features&).

import "errors"

func (k *SMTKernel) withSetupLogic(logic string, action func() error) error {
	previous := k.setupLogic
	k.setupLogic = logic
	defer func() { k.setupLogic = previous }()
	return action()
}

func (k *SMTKernel) setupUnknownStaticPolicy(useStaticFeatures bool) (result error) {
	if !useStaticFeatures || !smtLogicUsesUnknownSetup(k.setupLogic) {
		return nil
	}
	features, err := k.setupStaticFeatures(true)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, features.Close()) }()
	if features.NumQuantifiers != 0 {
		// setupQuantifiedLogicPolicy already applies setup_AUFLIA(false) or
		// setup_AUFLIRA(false), including the retained generic plugins.
		return nil
	}

	difference := (features.NumDiffTerms != 0 || features.NumDiffEqualities != 0 || features.NumDiffInequalities != 0) &&
		features.NumArithEqualities == features.NumDiffEqualities &&
		features.NumArithTerms == features.NumDiffTerms &&
		features.NumArithInequalities == features.NumDiffInequalities
	arithmetic := features.NumArithEqualities != 0 || features.NumArithTerms != 0 || features.NumArithInequalities != 0
	theories := features.NumTheories()

	if features.NumNonUFTheories() == 0 {
		return k.withSetupLogic("QF_UF", func() error { return k.setupSimpleLogicPolicy(true) })
	}
	if theories == 1 && difference {
		switch {
		case features.HasReal && !features.HasInt:
			return k.withSetupLogic("QF_RDL", func() error { return k.setupRealDifferencePolicy(true) })
		case !features.HasReal && features.HasInt:
			return k.withSetupLogic("QF_IDL", func() error { return k.setupIntegerDifferencePolicy(true) })
		default:
			return k.setupBasicArithmetic()
		}
	}
	if theories == 2 && features.HasUF() && difference {
		if !features.HasReal && features.HasInt {
			return k.withSetupLogic("QF_UFIDL", func() error { return k.setupUFIntegerDifferencePolicy(true) })
		}
		return k.setupBasicArithmetic()
	}
	if theories == 1 && arithmetic {
		switch {
		case features.HasInt && features.HasReal || features.NumNonLinear != 0:
			return k.installMixedArithmeticSetup(k.smtParams.arith.state)
		case features.HasReal:
			return k.withSetupLogic("QF_LRA", func() error { return k.setupRealLogicPolicy(true) })
		default:
			if err := k.installIntegerArithmeticSetup(k.smtParams.arith.state); err != nil {
				return err
			}
			return k.withSetupLogic("QF_LIA", func() error { return k.setupIntegerLogicPolicy(true) })
		}
	}
	if theories == 2 && features.HasUF() && arithmetic {
		switch {
		case !features.HasReal:
			return k.setupUnknownUFLIA()
		case !features.HasInt && features.NumNonLinear == 0:
			return k.withSetupLogic("QF_UFLRA", func() error { return k.setupSimpleLogicPolicy(true) })
		default:
			return k.setupBasicArithmetic()
		}
	}
	if theories == 1 && features.HasBitVector {
		return k.withSetupLogic("QF_BV", k.setupBitVectorLogicPolicy)
	}
	if theories == 1 && features.HasFloatingPoint {
		return k.withSetupLogic("QF_FP", k.setupBitVectorLogicPolicy)
	}
	if theories == 2 && features.HasFloatingPoint && features.HasBitVector {
		return k.withSetupLogic("QF_FPBV", k.setupBitVectorLogicPolicy)
	}
	if theories == 1 && features.HasArrays {
		return k.withSetupLogic("QF_AX", func() error { return k.setupArrayLogicPolicy(true) })
	}
	if theories == 2 && features.HasUF() && features.HasArrays && features.HasBitVector {
		return k.withSetupLogic("QF_AUFBV", k.setupBitVectorLogicPolicy)
	}
	if theories == 2 && features.HasUF() && features.HasArrays && features.HasInt {
		return k.withSetupLogic("QF_AUFLIA", func() error { return k.setupArrayLogicPolicy(false) })
	}
	return k.setupBasicArithmetic()
}

func (k *SMTKernel) setupUnknownUFLIA() error {
	smt := k.smtParams.state
	arithmetic := k.smtParams.arith.state
	preprocess := k.smtParams.preprocess.state
	smt.RelevancyLvl = 0
	arithmetic.Reflect = false
	arithmetic.PropagationThreshold = 1000
	preprocess.NNFCNF = false
	if err := k.installIntegerArithmeticSetup(arithmetic); err != nil {
		return err
	}
	return k.applyArithmeticSetup(smt, arithmetic, preprocess)
}

func (k *SMTKernel) installIntegerArithmeticSetup(arithmetic SMTArithState) error {
	family, err := k.manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	theory, err := NewSMTLinearIntegerArithmeticTheory(family)
	if err != nil {
		return err
	}
	if err = theory.ConfigureArithmetic(arithmetic); err != nil {
		return err
	}
	if err = k.replaceUnusedSetupArithmetic(theory); err != nil {
		return err
	}
	return nil
}
