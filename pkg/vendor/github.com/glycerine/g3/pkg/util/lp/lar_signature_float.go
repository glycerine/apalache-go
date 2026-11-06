package lp

// Source: src/util/lp/lar_core_solver.h:717-734.

// FindSolutionSignatureWithDoubles runs the actual floating feasibility search.
// The returned trace is a value copy, independent of the presolver's storage.
func (c *LARCoreSolver) FindSolutionSignatureWithDoubles(signature *LARSolutionSignature) (trace []uint32, err error) {
	if c.stopped || c.calls.pending() {
		return nil, ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if c.DPrimal == nil {
		return nil, ErrBounds
	}
	d := c.DPrimal
	if d.Factorization == nil || d.Factorization.GetStatus() != LUOK {
		return nil, nil
	}
	if err = c.GetBoundsForDoubleSolver(); err != nil {
		return nil, err
	}
	if err = c.ExtractSignatureFromPairSolver(signature, c.RPrimal); err != nil {
		return nil, err
	}
	if err = c.PrepareFloatSolverXWithSignature(*signature, d); err != nil {
		return nil, err
	}
	d.StartTracingBasisChanges()
	if err = d.FindFeasibleSolution(); err != nil {
		return nil, err
	}
	cancel, e := c.settings.CancelFlag()
	if e != nil {
		return nil, e
	}
	if cancel {
		// Source returns before stop_tracing and signature extraction.
		return nil, nil
	}
	d.StopTracingBasisChanges()
	if err = c.ExtractSignatureFromFloatSolver(signature, d); err != nil {
		return nil, err
	}
	return append([]uint32(nil), d.Trace...), nil
}
