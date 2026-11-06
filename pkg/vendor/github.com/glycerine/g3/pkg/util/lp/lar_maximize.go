package lp

// Source: src/util/lp/lar_solver.h:552-663; theory_lra.cpp:2355.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *LARSolver) CostsAreZerosForRSolver() (ok bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	if !sourceDebugEnabled {
		return true, nil
	}
	for i := range s.Core.Costs.Data {
		zero, e := s.manager.IsZero(&s.Core.Costs.Data[i])
		if e != nil {
			return false, e
		}
		if !zero {
			return false, ErrAssertion
		}
	}
	return true, nil
}
func (s *LARSolver) ReducedCostsAreZeroesForRSolver() (ok bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	if !sourceDebugEnabled {
		return true, nil
	}
	for i := range s.Core.RPrimal.D.Data {
		zero, e := s.manager.IsZero(&s.Core.RPrimal.D.Data[i])
		if e != nil {
			return false, e
		}
		if !zero {
			return false, ErrAssertion
		}
	}
	return true, nil
}

func (s *LARSolver) PrepareCostsForRSolver(term []RationalSparseEntry) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	c, r := s.Core, s.Core.RPrimal
	r.UsingInfeasCosts = false
	if sourceDebugEnabled {
		if _, err = s.CostsAreZerosForRSolver(); err != nil {
			return err
		}
		if _, err = s.ReducedCostsAreZeroesForRSolver(); err != nil {
			return err
		}
	}
	if err = c.Costs.resizeData(c.RA.ColumnCount()); err != nil {
		return err
	}
	s.rationalWork = NewIndexedRationalVector(s.manager, 1)
	for i := range term {
		cell := &term[i]
		j := cell.Index
		if uint64(j) >= uint64(len(c.Costs.Data)) {
			return ErrBounds
		}
		if err = c.Costs.copySlot(&c.Costs.Data[j], &cell.Value); err != nil {
			return err
		}
		if uint64(j) >= uint64(len(c.RHeading)) {
			return ErrBounds
		}
		if c.RHeading[j] < 0 {
			if uint64(j) >= uint64(len(r.D.Data)) {
				return ErrBounds
			}
			b, e := s.calls.begin(s.manager)
			if e != nil {
				return e
			}
			e = s.manager.Add(&r.D.Data[j], &cell.Value, &r.D.Data[j])
			if err = s.calls.after(s.manager, b, e); err != nil {
				return err
			}
		} else {
			negative := &s.rationalWork.Data[0]
			if err = s.rationalWork.copySlot(negative, &cell.Value); err != nil {
				return err
			}
			b, e := s.calls.begin(s.manager)
			if e != nil {
				return e
			}
			e = s.manager.Neg(negative)
			if err = s.calls.after(s.manager, b, e); err != nil {
				return err
			}
			if err = r.UpdateReducedCostForBasicColumnCostChange(negative, j); err != nil {
				return err
			}
			if err = s.manager.Del(negative); err != nil {
				return err
			}
			*negative = numeral.NewRational()
		}
	}
	if err = s.closeConstraintValue(); err != nil {
		return err
	}
	if sourceDebugEnabled {
		ok, e := r.ReducedCostsAreCorrectTableau()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
	}
	return nil
}
func (s *LARSolver) zeroObjectiveCoefficient(out *numeral.Rational) (err error) {
	b, err := s.calls.begin(s.manager)
	if err != nil {
		return err
	}
	err = s.manager.SetInt32(out, 0)
	return s.calls.after(s.manager, b, err)
}

func (s *LARSolver) SetCostsToZero(term []RationalSparseEntry) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	c, r := s.Core, s.Core.RPrimal
	if sourceDebugEnabled && !r.InfSet.Empty() {
		return ErrAssertion
	}
	for i := range term {
		j := term[i].Index
		if uint64(j) >= uint64(len(c.Costs.Data)) {
			return ErrBounds
		}
		if err = s.zeroObjectiveCoefficient(&c.Costs.Data[j]); err != nil {
			return err
		}
		if uint64(j) >= uint64(len(c.RHeading)) {
			return ErrBounds
		}
		row := c.RHeading[j]
		if row < 0 {
			if err = r.InfSet.Insert(j); err != nil {
				return err
			}
		} else {
			if uint64(row) >= uint64(len(c.RA.Rows)) {
				return ErrBounds
			}
			for _, cell := range c.RA.Rows[row] {
				if err = r.InfSet.Insert(cell.Column); err != nil {
					return err
				}
			}
		}
	}
	for _, j := range r.InfSet.index {
		if uint64(j) >= uint64(len(r.D.Data)) {
			return ErrBounds
		}
		if err = s.zeroObjectiveCoefficient(&r.D.Data[j]); err != nil {
			return err
		}
	}
	if err = r.InfSet.Clear(); err != nil {
		return err
	}
	if sourceDebugEnabled {
		if _, err = s.ReducedCostsAreZeroesForRSolver(); err != nil {
			return err
		}
		if _, err = s.CostsAreZerosForRSolver(); err != nil {
			return err
		}
	}
	return nil
}

// MaximizeTermOnTableau reads the actual core terminal status. Source returns
// false only for UNBOUNDED; other statuses still evaluate the current model.
// The caller's output must outlive solver cleanup after interrupted arithmetic.
func (s *LARSolver) MaximizeTermOnTableau(term []RationalSparseEntry, out *RationalPair) (bounded bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	if out == nil {
		return false, ErrBounds
	}
	if s.StrategyIsUndecided() {
		if err = s.DecideOnStrategyAndAdjustInitialState(); err != nil {
			return false, err
		}
	}
	if err = s.Core.Solve(); err != nil {
		return false, err
	}
	if s.Core.RPrimal.Status == StatusUnbounded {
		return false, nil
	}
	s.work = NewIndexedPairVector(s.manager, 1)
	if err = s.zeroObjectiveCoefficient(&out.X); err != nil {
		return false, err
	}
	if err = s.zeroObjectiveCoefficient(&out.Y); err != nil {
		return false, err
	}
	product := &s.work.Data[0]
	for i := range term {
		cell := &term[i]
		if uint64(cell.Index) >= uint64(len(s.Core.RX.Data)) {
			return false, ErrBounds
		}
		value := &s.Core.RX.Data[cell.Index]
		b, e := s.calls.begin(s.manager)
		if e != nil {
			return false, e
		}
		e = s.manager.Mul(&cell.Value, &value.X, &product.X)
		if e == nil {
			e = s.manager.Mul(&cell.Value, &value.Y, &product.Y)
		}
		if e == nil {
			e = out.Add(s.manager, out, product)
		}
		if err = s.calls.after(s.manager, b, e); err != nil {
			return false, err
		}
	}
	if err = s.work.Close(); err != nil {
		return false, err
	}
	s.work = nil
	return true, nil
}

func (s *LARSolver) MaximizeTermOnCorrectedRSolver(term []RationalSparseEntry, out *RationalPair) (bounded bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	s.Settings.BackupCosts = false
	switch s.Settings.SimplexStrategy {
	case StrategyTableauRows, StrategyTableauCosts:
		rows := s.Settings.SimplexStrategy == StrategyTableauRows
		if err = s.PrepareCostsForRSolver(term); err != nil {
			return false, err
		}
		if rows {
			s.Settings.SimplexStrategy = StrategyTableauCosts
		}
		bounded, err = s.MaximizeTermOnTableau(term, out)
		if err != nil {
			return false, err
		}
		if rows {
			s.Settings.SimplexStrategy = StrategyTableauRows
		}
		if err = s.SetCostsToZero(term); err != nil {
			return false, err
		}
		s.Core.RPrimal.Status = StatusOptimal
		return bounded, nil
	default:
		if sourceDebugEnabled {
			return false, ErrAssertion
		}
		return false, nil
	}
}
func (s *LARSolver) MaximizeTerm(term []RationalSparseEntry, out *RationalPair) (bounded bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	if sourceDebugEnabled && !s.Core.RPrimal.CurrentXIsFeasible() {
		return false, ErrAssertion
	}
	s.Core.RPrimal.LookForFeasibleSolutionOnly = false
	return s.MaximizeTermOnCorrectedRSolver(term, out)
}
