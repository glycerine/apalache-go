package lp

// Source: src/util/lp/lar_core_solver.hpp:275-319.

// Solve orchestrates the coupled LAR cores and source explanation/status
// publication. In particular, source maps every terminal status other than
// infeasible and unbounded to optimal, unless floating presolve timed out.
func (c *LARCoreSolver) Solve() (err error) {
	if c.stopped || c.calls.pending() || c.signatureDelta != nil || c.signatureExpression != nil || c.signatureScalars != nil || c.catchUpWork != nil || c.evidenceNegated != nil {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	r := c.RPrimal
	if r == nil {
		return ErrBounds
	}
	if err = r.ready(); err != nil {
		return err
	}
	if sourceDebugEnabled {
		if err = c.assertLARSolveInputs(); err != nil {
			return err
		}
	}
	if r.CurrentXIsFeasible() && r.LookForFeasibleSolutionOnly {
		r.Status = StatusOptimal
		return nil
	}
	c.settings.Stats.NeedToSolveInf++
	if sourceDebugEnabled {
		off, e := r.AMultXIsOff()
		if e != nil {
			return e
		}
		if off {
			return ErrAssertion
		}
		if c.settings.UseTableau() {
			if _, err = c.RBasisIsOK(); err != nil {
				return err
			}
		}
	}
	if c.NeedToPresolveWithDoubleSolver() {
		if c.DPrimal == nil {
			return ErrBounds
		}
		if err = c.PrefixD(); err != nil {
			return err
		}
		signature := LARSolutionSignature{}
		changes, e := c.FindSolutionSignatureWithDoubles(&signature)
		if e != nil {
			return e
		}
		if c.DPrimal.Status == StatusTimeExhausted {
			r.Status = StatusTimeExhausted
			return nil
		}
		// Settings remain live through the actual floating callbacks.
		if c.settings.UseTableau() {
			err = c.SolveOnSignatureTableau(signature, changes)
		} else {
			err = c.SolveOnSignature(signature, changes)
		}
		if err != nil {
			return err
		}
	} else {
		if !c.settings.UseTableau() {
			snapped, e := r.SnapNonBasicXToBound()
			if e != nil {
				return e
			}
			if sourceDebugEnabled {
				ok, e := r.NonBasicColumnsAreSetCorrectly()
				if e != nil {
					return e
				}
				if !ok {
					return ErrAssertion
				}
			}
			if snapped {
				if err = r.SolveAxEqualsB(); err != nil {
					return err
				}
			}
		}
		if r.LookForFeasibleSolutionOnly {
			err = r.FindFeasibleSolution()
		} else {
			_, err = r.Solve()
		}
		if err != nil {
			return err
		}
	}
	if sourceDebugEnabled && c.settings.UseTableau() {
		if _, err = c.RBasisIsOK(); err != nil {
			return err
		}
	}
	if r.Status == StatusInfeasible {
		if err = c.FillNotImprovableZeroSum(); err != nil {
			return err
		}
	} else if r.Status != StatusUnbounded {
		r.Status = StatusOptimal
	}
	if sourceDebugEnabled {
		if _, err = c.RBasisIsOK(); err != nil {
			return err
		}
		return c.assertLARSolveInputs()
	}
	return nil
}

func (c *LARCoreSolver) assertLARSolveInputs() error {
	ok, e := c.RPrimal.NonBasicColumnsAreSetCorrectly()
	if e != nil {
		return e
	}
	if !ok {
		return ErrAssertion
	}
	ok, e = c.RPrimal.InfSetIsCorrect()
	if e != nil {
		return e
	}
	if !ok {
		return ErrAssertion
	}
	return nil
}
