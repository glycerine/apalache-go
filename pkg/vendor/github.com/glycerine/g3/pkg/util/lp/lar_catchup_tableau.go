package lp

// Source: src/util/lp/lar_core_solver.h:533-582.

// CatchUpInLUTableau selects the same delayed endpoints as LU catch-up, then
// changes basis before pivoting the actual tableau. A false pivot result rolls
// back just that exchange and repivots its old basic column.
func (c *LARCoreSolver) CatchUpInLUTableau(trace []uint32, target []int32) (ok bool, err error) {
	if c.stopped || c.calls.pending() {
		return false, ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	r := c.RPrimal
	if r == nil {
		return false, ErrBounds
	}
	if err = r.ready(); err != nil {
		return false, err
	}
	if sourceDebugEnabled {
		if _, err = c.RBasisIsOK(); err != nil {
			return false, err
		}
	}
	var queues larCatchUpQueues
	for i := 0; i < len(trace); i += 2 {
		if i+1 >= len(trace) {
			return false, ErrBounds
		}
		enter, leave, use, e := queues.selectColumns(trace[i], trace[i+1], target, *r.Heading)
		if e != nil {
			return false, e
		}
		if !use {
			continue
		}
		if sourceDebugEnabled && ((*r.Heading)[enter] >= 0 || (*r.Heading)[leave] < 0) {
			return false, ErrAssertion
		}
		if err = r.ChangeBasisUnconditionally(enter, leave); err != nil {
			return false, err
		}
		pivoted, e := r.PivotColumnTableau(enter, uint32((*r.Heading)[enter]))
		if e != nil {
			return false, e
		}
		if !pivoted {
			if err = r.ChangeBasisUnconditionally(leave, enter); err != nil {
				return false, err
			}
			restored, e := r.PivotColumnTableau(leave, uint32((*r.Heading)[leave]))
			if e != nil {
				return false, e
			}
			if sourceDebugEnabled && !restored {
				return false, ErrAssertion
			}
			return false, nil
		}
	}
	if sourceDebugEnabled {
		if _, err = c.RBasisIsOK(); err != nil {
			return false, err
		}
	}
	return true, nil
}
