package lp

// Source: src/util/lp/lar_core_solver.hpp:177-195 and lar_solver.h:250-270,398-400.

// CalculatePivotRow sizes the actual precise scratch before calculating the
// inverse basis row and its nonbasic coefficients. It requires the source's
// prepared factorization and RowsNZ metadata; it does not initialize a solver.
func (c *LARCoreSolver) CalculatePivotRow(row uint32) (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	m := c.RPrimal
	if err = m.ready(); err != nil {
		return err
	}
	if sourceDebugEnabled && c.settings.UseTableau() {
		return ErrAssertion
	}
	if err = m.PivotRowB1.Clear(); err != nil {
		return err
	}
	if err = m.PivotRowB1.Resize(c.RA.RowCount()); err != nil {
		return err
	}
	if err = m.PivotRow.Clear(); err != nil {
		return err
	}
	if err = m.PivotRow.Resize(c.RA.ColumnCount()); err != nil {
		return err
	}
	// This source branch survives native compilation despite the preceding
	// debug assertion rejecting tableau mode.
	if c.settings.UseTableau() {
		if uint64(row) >= uint64(len(c.RBasis)) || uint64(row) >= uint64(len(c.RA.Rows)) {
			return ErrBounds
		}
		basis := c.RBasis[row]
		for k := range c.RA.Rows[row] {
			cell := &c.RA.Rows[row][k]
			if cell.Column != basis {
				if err = m.PivotRow.SetValue(&cell.Value, cell.Column); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err = m.CalculatePivotRowOfBInverse(row); err != nil {
		return err
	}
	return m.CalculatePivotRowWhenInverseReady(row)
}

func (s *LARSolver) TermIsUsedAsRow(term uint32) (bool, error) {
	if sourceDebugEnabled && !s.IsTerm(term) {
		return false, ErrAssertion
	}
	_, found := s.ExternalToColumns[term]
	return found, nil
}

func (s *LARSolver) SubstituteBasisVarInTermsForRow(row uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if uint64(row) >= uint64(len(s.Core.RBasis)) {
		return ErrBounds
	}
	basis := s.Core.RBasis[row]
	for k, term := range s.Terms {
		// Preserve the source's offset argument here. TermIsUsedAsRow expects
		// a full term ID: debug asserts; native looks up this literal offset.
		used, e := s.TermIsUsedAsRow(uint32(k))
		if e != nil {
			return e
		}
		if used || !term.Contains(basis) {
			continue
		}
		if err = term.Substitute(basis, s.Core.RPrimal.PivotRow); err != nil {
			return err
		}
	}
	return nil
}

func (s *LARSolver) CalculateImpliedBoundsForRow(row uint32, bp *LARBoundPropagator) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if s.Settings.UseTableau() {
		return s.AnalyzeNewBoundsOnRowTableau(row, bp)
	}
	if err = s.Core.CalculatePivotRow(row); err != nil {
		return err
	}
	if err = s.SubstituteBasisVarInTermsForRow(row); err != nil {
		return err
	}
	return s.AnalyzeNewBoundsOnRow(row, bp)
}
