package smtcore

// Source: src/smt/smt_setup.cpp setup_unknown and setup_arith.

import "errors"

// Basic setup collects arithmetic features even on the logic-only first-push
// path. Difference, dense, UTVPI and disabled modes select their concrete
// retained implementations, including the full three-component inf_ext.
// Constructor-only int_only/fixnum flags have no retained writers and stay false.
func (k *SMTKernel) setupBasicArithmetic() (result error) {
	features, err := k.setupStaticFeatures(true)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, features.Close()) }()
	arithmetic := k.smtParams.arith.state
	family, err := k.manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	var theory SMTTheoryInternalizer
	switch arithmetic.Mode {
	case 0:
		if _, ok := k.internalizer.theories[family].(*SMTDummyTheory); ok {
			return nil
		}
		theory, err = NewSMTDummyTheory(family, "no arithmetic")
	case 1:
		arithmetic.Eq2Ineq = true
		theory, err = NewSMTMixedDiffTheory(family)
	case 3:
		arithmetic.Eq2Ineq = true
		theory, err = NewSMTDenseDiffTheory(family)
	case 4:
		arithmetic.Eq2Ineq = true
		theory, err = NewSMTUTVPITheory(family)
	case 5:
		theory, err = NewSMTInfiniteArithmeticTheory(family)
	default:
		if existing, ok := k.internalizer.theories[family].(*SMTLinearArithmeticTheory); ok {
			// Direct internalizer clients may have already created columns.
			// Their concrete specialization must remain stable thereafter.
			if existing.NumVariables() == 0 && !k.copiedArithmeticPlugin {
				existing.integer, existing.mixed = false, true
			}
			return nil
		}
		theory, err = NewSMTMixedLinearArithmeticTheory(family)
	}
	if err != nil {
		return err
	}
	if configurable, ok := theory.(interface{ ConfigureArithmetic(SMTArithState) error }); ok {
		if err := configurable.ConfigureArithmetic(arithmetic); err != nil {
			return err
		}
	}
	if err := k.replaceUnusedSetupArithmetic(theory); err != nil {
		return err
	}
	if arithmetic.Mode == 1 || arithmetic.Mode == 3 || arithmetic.Mode == 4 {
		return k.applyArithmeticSetup(k.smtParams.state, arithmetic, k.smtParams.preprocess.state)
	}
	return nil
}
