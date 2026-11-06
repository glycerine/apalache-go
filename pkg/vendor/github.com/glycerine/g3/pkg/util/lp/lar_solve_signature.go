package lp

// Source: src/util/lp/lar_core_solver.h:483-485,603-636.

func (c *LARCoreSolver) NoRLU() bool {
	return c.RPrimal.Factorization == nil || c.RPrimal.Factorization.GetStatus() == LUDegenerated
}

// SolveOnSignature applies the floating basis/signature to the exact LU solver.
// A degenerate suggested basis is rolled back before exact feasibility search;
// that fallback deliberately uses the existing exact values, not the signature.
func (c *LARCoreSolver) SolveOnSignature(signature LARSolutionSignature, changes []uint32) (err error) {
	if c.stopped || c.catchUpWork != nil || c.calls.pending() {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if sourceDebugEnabled && c.settings.UseTableau() {
		return ErrAssertion
	}
	r := c.RPrimal
	if r == nil {
		return ErrBounds
	}
	if err = r.ready(); err != nil {
		return err
	}
	if r.Factorization == nil {
		for i := 0; i < len(changes); i += 2 {
			if i+1 >= len(changes) {
				return ErrBounds
			}
			if err = r.ChangeBasisUnconditionally(changes[i], changes[i+1]); err != nil {
				return err
			}
		}
		if err = InitPairFactorization(&r.Factorization, c.RA, c.RBasis, c.settings); err != nil {
			return err
		}
	} else if err = c.CatchUpInPairLU(changes, c.DHeading); err != nil {
		return err
	}
	if c.NoRLU() {
		if err = c.CatchUpInPairLUInReverse(changes); err != nil {
			return err
		}
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
		return nil
	}
	if err = c.PreparePairSolverXWithSignature(signature, r); err != nil {
		return err
	}
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
	return c.CatchUpInFloatLU(r.Trace, c.RHeading)
}
