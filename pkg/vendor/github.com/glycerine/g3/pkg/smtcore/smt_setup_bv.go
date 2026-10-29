package smtcore

// Source: src/smt/smt_setup.cpp setup_QF_BV, setup_QF_AUFBV,
// setup_QF_BVRE, setup_QF_FP and setup_QF_FPBV.

func (k *SMTKernel) setupBitVectorLogicPolicy() error {
	arrays := false
	switch k.setupLogic {
	case "QF_BV", "QF_BVRE", "QF_FP", "QF_FPBV", "QF_BVFP":
		k.smtParams.arith.state.Reflect = false
		k.arithState.Reflect = false
	case "QF_AUFBV", "QF_ABV", "QF_UFBV":
		arrays = true
	default:
		return nil
	}
	k.smtParams.state.RelevancyLvl = 0
	k.smtParams.bv.state.CC = false
	k.bvState = k.smtParams.bv.state
	k.smtParams.preprocess.bit.ExtGates = true
	k.smtParams.preprocess.state.NNFCNF = false
	if arrays {
		k.smtParams.array.state.Mode = 1 // AR_SIMPLE
		k.smtParams.preprocess.state.PropagateBooleans = true
	}
	if err := k.configureBitVectorTheories(k.bvState, k.smtParams.preprocess.bit); err != nil {
		return err
	}
	return k.applySimpleSetupPolicy()
}
