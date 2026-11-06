package lp

// Source: src/util/lp/init_lar_solver.h and lar_solver.h: retained term/model
// registration and direct basic-value reconstruction.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *LARSolver) AddTerm(coefficients []RationalSparseEntry, value *numeral.Rational) (index uint32, err error) {
	if err = s.ready(); err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			s.stopped = true
		}
	}()
	s.pendingTerm, err = NewLARTermFromCoefficients(s.manager, coefficients, value)
	if err != nil {
		return 0, err
	}
	s.Terms = append(s.Terms, s.pendingTerm)
	s.pendingTerm = nil
	s.pendingTerm, err = NewLARTermFromCoefficients(s.manager, coefficients, value)
	if err != nil {
		return 0, err
	}
	s.OriginalTerms = append(s.OriginalTerms, s.pendingTerm)
	s.pendingTerm = nil
	index = LARTermStartIndex + uint32(len(s.Terms)) - 1
	if s.StrategyIsUndecided() {
		return index, nil
	}
	if s.Settings.UseTableau() && len(coefficients) != 0 {
		if err = s.addRowForTerm(s.OriginalTerms[len(s.OriginalTerms)-1], index); err != nil {
			return index, err
		}
		if s.Settings.BoundPropagation {
			if err = s.RowsWithChangedBounds.Insert(s.Core.RA.RowCount() - 1); err != nil {
				return index, err
			}
		}
	}
	if sourceDebugEnabled && uint32(len(s.ExternalToColumns)) != s.Core.RA.ColumnCount() {
		return index, ErrAssertion
	}
	return index, nil
}
func (s *LARSolver) addBasicCoreFields() error {
	c := s.Core
	lu := c.NeedToPresolveWithDoubleSolver()
	if sourceDebugEnabled && lu && c.RA.ColumnCount() != c.DA.ColumnCount() {
		return ErrAssertion
	}
	s.Bounds.Types.Append(ColumnFree)
	if err := s.Bounds.ChangedColumns.IncreaseSizeByOne(); err != nil {
		return err
	}
	if err := s.RowsWithChangedBounds.IncreaseSizeByOne(); err != nil {
		return err
	}
	if err := s.addRationalCoreFields(true); err != nil {
		return err
	}
	if lu {
		return s.addDoubleCoreFields(true)
	}
	return nil
}
func (s *LARSolver) AddRowFromTermNoConstraint(term *LARTerm, external uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			s.stopped = true
		}
	}()
	if term.manager != s.manager {
		return ErrExactVectorOwner
	}
	if err = term.ready(); err != nil {
		return err
	}
	if err = s.registerNewExternalIndex(external); err != nil {
		return err
	}
	j := s.Core.RA.ColumnCount()
	s.Bounds.Witnesses.Append(NewLARBoundWitnesses(j)) // Source stores column j, not the new row index.
	if err = s.addBasicCoreFields(); err != nil {
		return err
	}
	if s.Settings.UseTableau() {
		s.pendingIterator, err = NewLARTermIterator(term, j)
		if err != nil {
			return err
		}
		if err = s.Core.RA.FillLastRowWithPivoting(s.pendingIterator, s.Core.RHeading); err != nil {
			return err
		}
		if err = s.pendingIterator.Close(); err != nil {
			return err
		}
		s.pendingIterator = nil
		if err = s.Core.RightSides.resizeData(s.Core.RA.ColumnCount()); err != nil {
			return err
		}
	} else {
		if err = s.fillLastRationalRow(term); err != nil {
			return err
		}
	}
	if err = s.setBasicValueFromRow(s.Core.RA.RowCount()-1, j); err != nil {
		return err
	}
	if s.UseLU() {
		if err = s.fillLastDoubleRow(term); err != nil {
			return err
		}
	}
	return err
}

// Only the source add_row_for_term wrapper evaluates sizes_are_correct on both
// sides. Deferred-row and constraint insertion call the unwrapped helper.
func (s *LARSolver) addRowForTerm(term *LARTerm, external uint32) error {
	if sourceDebugEnabled {
		if _, err := s.SizesAreCorrect(); err != nil {
			return err
		}
	}
	if err := s.AddRowFromTermNoConstraint(term, external); err != nil {
		return err
	}
	if sourceDebugEnabled {
		if _, err := s.SizesAreCorrect(); err != nil {
			return err
		}
	}
	return nil
}
func (s *LARSolver) fillLastRationalRow(term *LARTerm) error {
	a := s.Core.RA
	if a.RowCount() == 0 || a.ColumnCount() == 0 {
		return ErrBounds
	}
	row := a.RowCount() - 1
	if sourceDebugEnabled && len(a.Rows[row]) != 0 {
		return ErrAssertion
	}
	s.rationalWork = NewIndexedRationalVector(s.manager, 1)
	negative := &s.rationalWork.Data[0]
	for j, value := range term.coefficients.All() {
		before, err := s.calls.begin(s.manager)
		if err != nil {
			return err
		}
		if sourceDebugEnabled {
			zero, e := s.manager.IsZero(value)
			err = e
			if err == nil && zero {
				err = ErrAssertion
			}
		}
		if err == nil {
			err = s.manager.Copy(negative, value)
		}
		if err == nil {
			err = s.manager.Neg(negative)
		}
		if err = s.calls.after(s.manager, before, err); err != nil {
			return err
		}
		// Matrix Set retains its own stopped copy; negative remains borrowed until
		// matrix cleanup completes, including on failure.
		if err = a.Set(row, j, negative); err != nil {
			return err
		}
	}
	one := numeral.NewRational()
	one.Num = numeral.NewInt(1)
	if err := a.Set(row, a.ColumnCount()-1, &one); err != nil {
		return err
	}
	if err := s.rationalWork.Close(); err != nil {
		return err
	}
	s.rationalWork = nil
	return nil
}
func (s *LARSolver) fillLastDoubleRow(term *LARTerm) error {
	a := s.Core.DA
	if a.RowCount() == 0 || a.ColumnCount() == 0 {
		return ErrBounds
	}
	row := a.RowCount() - 1
	if sourceDebugEnabled && len(a.Rows[row]) != 0 {
		return ErrAssertion
	}
	for j, value := range term.coefficients.All() {
		if sourceDebugEnabled {
			zero, err := s.manager.IsZero(value)
			if err != nil {
				return err
			}
			if zero {
				return ErrAssertion
			}
		}
		v, err := s.manager.Float64(value)
		if err != nil {
			return err
		}
		if err = a.Set(row, j, -v); err != nil {
			return err
		}
	}
	return a.Set(row, a.ColumnCount()-1, -1) // Literal source floating basis sign.
}
func (s *LARSolver) setBasicValueFromRow(row, j uint32) (err error) {
	if uint64(row) >= uint64(len(s.Core.RBasis)) || uint64(j) >= uint64(len(s.Core.RX.Data)) {
		return ErrBounds
	}
	before, err := s.calls.begin(s.manager)
	if err != nil {
		return err
	}
	defer func() { err = s.calls.after(s.manager, before, err) }()
	s.work = NewIndexedPairVector(s.manager, 2)
	s.rationalWork = NewIndexedRationalVector(s.manager, 3)
	total, product := &s.work.Data[0], &s.work.Data[1]
	temporary, px, py := &s.rationalWork.Data[0], &s.rationalWork.Data[1], &s.rationalWork.Data[2]
	basis := s.Core.RBasis[row]
	for k := range s.Core.RA.Rows[row] {
		cell := &s.Core.RA.Rows[row][k]
		if cell.Column == basis {
			continue
		}
		if uint64(cell.Column) >= uint64(len(s.Core.RX.Data)) {
			return ErrBounds
		}
		x := &s.Core.RX.Data[cell.Column]
		zero, e := x.IsZero(s.manager)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		if err = etaRationalProduct(s.manager, temporary, px, &cell.Value, &x.X); err != nil {
			return err
		}
		if err = etaRationalProduct(s.manager, temporary, py, &cell.Value, &x.Y); err != nil {
			return err
		}
		if err = s.manager.Copy(&product.X, px); err != nil {
			return err
		}
		if err = s.manager.Copy(&product.Y, py); err != nil {
			return err
		}
		if err = total.Sub(s.manager, total, product); err != nil {
			return err
		}
		if err = product.Del(s.manager); err != nil {
			return err
		}
		*product = NewRationalPair()
		if err = s.manager.Del(py); err != nil {
			return err
		}
		*py = numeral.NewRational()
		if err = s.manager.Del(px); err != nil {
			return err
		}
		*px = numeral.NewRational()
	}
	if err = s.Core.RX.Data[j].Copy(s.manager, total); err != nil {
		return err
	}
	if err = s.rationalWork.Close(); err != nil {
		return err
	}
	s.rationalWork = nil
	if err = s.work.Close(); err != nil {
		return err
	}
	s.work = nil
	return nil
}
func (s *LARSolver) AdjustInitialStateForTableauRows() (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	for j, term := range s.Terms {
		ext := LARTermStartIndex + uint32(j)
		if _, exists := s.ExternalToColumns[ext]; exists {
			continue
		}
		if err = s.AddRowFromTermNoConstraint(term, ext); err != nil {
			return err
		}
	}
	return nil
}
