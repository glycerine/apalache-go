package smtcore

// Source: src/smt/asserted_formulas.cpp setup(), called by
// smt::context::setup_components after logic/static-feature setup.

func (k *SMTKernel) setupAssertedPolicies() {
	p := &k.preprocessState
	if p.LiftITE == smtLiftITEFull || (p.LiftITE == smtLiftITEConservative && p.NonGroundLiftITE == smtLiftITEConservative) {
		p.NonGroundLiftITE = smtLiftITENone
	}
	if k.smtState.RelevancyLvl == 0 {
		k.smtState.RelevancyLemma = false
	}
	if k.smtParams != nil {
		k.smtParams.preprocess.state.NonGroundLiftITE = p.NonGroundLiftITE
		k.smtParams.state.RelevancyLemma = k.smtState.RelevancyLemma
	}
}
