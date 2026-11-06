package smtcore

// Source: src/smt/smt_setup.cpp setup_mi_arith.

// installMixedArithmeticSetup applies the only source backend override owned by
// setup_mi_arith. Other modes retain the ordinary mixed linear implementation.
func (k *SMTKernel) installMixedArithmeticSetup(config SMTArithState) error {
	if config.Mode != 5 { // AS_OPTINF
		return nil
	}
	family, err := k.manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	theory, err := NewSMTInfiniteArithmeticTheory(family)
	if err != nil {
		return err
	}
	if err = theory.ConfigureArithmetic(config); err != nil {
		return err
	}
	return k.replaceUnusedSetupArithmetic(theory)
}
