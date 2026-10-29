package lp

// Source: src/util/lp/lar_core_solver.h:487-515.

// SolveOnSignatureTableau applies the floating basis/signature to the exact
// tableau. A singular suggestion reverses its last exchange, uses the remaining
// signature, and replaces floating basis metadata before discarding its LU.
func (c *LARCoreSolver) SolveOnSignatureTableau(signature LARSolutionSignature, changes []uint32) (err error) {
	if c.stopped || c.calls.pending() || c.signatureDelta != nil || c.signatureExpression != nil || c.signatureScalars != nil || c.catchUpWork != nil {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	// This source call is outside SASSERT (the validator itself erases in native).
	if _, err = c.RBasisIsOK(); err != nil {
		return err
	}
	if sourceDebugEnabled && !c.settings.UseTableau() {
		return ErrAssertion
	}
	caughtUp, e := c.CatchUpInLUTableau(changes, c.DHeading)
	if e != nil {
		return e
	}
	if err = c.PrepareSolverXWithSignatureTableau(signature); err != nil {
		return err
	}
	r := c.RPrimal
	if !caughtUp {
		if err = r.FindFeasibleSolution(); err != nil {
			return err
		}
		c.DBasis = append(c.DBasis[:0], c.RBasis...)
		c.DHeading = append(c.DHeading[:0], c.RHeading...)
		c.DNBasis = append(c.DNBasis[:0], c.RNBasis...)
		if c.DPrimal == nil {
			return ErrBounds
		}
		if c.DPrimal.Factorization != nil {
			c.DPrimal.Factorization.Close()
			c.DPrimal.Factorization = nil
		}
	} else {
		r.StartTracingBasisChanges()
		if err = r.FindFeasibleSolution(); err != nil {
			return err
		}
		cancel, e := c.settings.CancelFlag()
		if e != nil {
			return e
		}
		if cancel {
			return nil
		}
		r.StopTracingBasisChanges()
		if sourceDebugEnabled && r.TotalIterations < uint32(len(r.Trace)/2) {
			return ErrAssertion
		}
		if err = c.CatchUpInFloatLU(r.Trace, c.RHeading); err != nil {
			return err
		}
	}
	if sourceDebugEnabled {
		if _, err = c.RBasisIsOK(); err != nil {
			return err
		}
	}
	return nil
}
