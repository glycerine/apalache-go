package lp

// Source: src/util/lp/lar_solver.h:487-499,1515-1555.

// CleanLargeElementsAfterPop preserves the source snapshot and swap-pop order.
// It removes members, retaining the old set domain.
func CleanLargeElementsAfterPop(n uint32, set *IntSet) error {
	if set == nil {
		return ErrBounds
	}
	var remove []uint32
	for _, j := range set.Indices() {
		if j >= n {
			remove = append(remove, j)
		}
	}
	for _, j := range remove {
		set.Erase(j)
	}
	return nil
}
func ShrinkInfSetAfterPop(n uint32, set *IntSet) error {
	if err := CleanLargeElementsAfterPop(n, set); err != nil {
		return err
	}
	return set.Resize(n)
}

// CleanInfSetOfRSolverAfterPop restores nonbasic feasibility, then removes
// feasible basic members and updates their tableau costs. The source's local
// changed-cost set is empty; member BasicColumnsWithChangedCost is not consumed.
func (s *LARSolver) CleanInfSetOfRSolverAfterPop() (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	c := s.Core
	r := c.RPrimal
	if err = CleanLargeElementsAfterPop(c.RA.ColumnCount(), r.InfSet); err != nil {
		return err
	}
	var becameFeasible []uint32
	// The per-iteration source delta remains a separate owner while child calls
	// borrow it. Transfer it to the stopped outer owner only on an error return.
	var delta *RationalPair
	defer func() {
		if delta != nil {
			s.pendingCleanupDelta = delta
		}
	}()
	copyIndex := append([]uint32(nil), r.InfSet.Indices()...)
	for _, j := range copyIndex {
		if uint64(j) >= uint64(len(c.RHeading)) {
			return ErrBounds
		}
		if c.RHeading[j] >= 0 {
			continue
		}
		value := NewRationalPair()
		delta = &value
		changed, e := r.MakeColumnFeasible(j, delta)
		if e != nil {
			return e
		}
		if changed {
			if err = s.ChangeBasicXByDeltaOnColumn(j, delta); err != nil {
				return err
			}
		}
		if err = delta.Del(s.manager); err != nil {
			return err
		}
		delta = nil
		becameFeasible = append(becameFeasible, j)
	}
	for _, j := range becameFeasible {
		if sourceDebugEnabled {
			if uint64(j) >= uint64(len(c.RHeading)) {
				return ErrBounds
			}
			if c.RHeading[j] >= 0 {
				return ErrAssertion
			}
		}
		if uint64(j) >= uint64(len(r.D.Data)) || uint64(j) >= uint64(len(c.Costs.Data)) {
			return ErrBounds
		}
		before, e := s.calls.begin(s.manager)
		if e != nil {
			return e
		}
		err = s.manager.Sub(&r.D.Data[j], &c.Costs.Data[j], &r.D.Data[j])
		if err == nil {
			err = s.manager.SetInt32(&c.Costs.Data[j], 0)
		}
		if err = s.calls.after(s.manager, before, err); err != nil {
			return err
		}
		r.InfSet.Erase(j)
	}
	becameFeasible = becameFeasible[:0]
	for _, j := range r.InfSet.Indices() {
		if sourceDebugEnabled {
			if uint64(j) >= uint64(len(c.RHeading)) {
				return ErrBounds
			}
			if c.RHeading[j] < 0 {
				return ErrAssertion
			}
		}
		feasible, e := r.ColumnIsFeasible(j)
		if e != nil {
			return e
		}
		if feasible {
			becameFeasible = append(becameFeasible, j)
		}
	}
	for _, j := range becameFeasible {
		r.InfSet.Erase(j)
	}
	if s.Settings.SimplexStrategy == StrategyTableauCosts {
		for _, j := range becameFeasible {
			if err = r.UpdateInfCostForColumnTableau(j); err != nil {
				return err
			}
		}
		// Source iterates a separate, empty local changed-cost set here.
		if sourceDebugEnabled {
			correct, e := r.ReducedCostsAreCorrectTableau()
			if e != nil {
				return e
			}
			if !correct {
				return ErrAssertion
			}
		}
	}
	return nil
}
