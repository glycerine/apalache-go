package lp

// Source: src/util/lp/lar_solver.h:1393-1510; exact tableau scope removal.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *LARSolver) ColumnRepresentsRowInTableau(j uint32) (bool, error) {
	w, err := s.Bounds.Witnesses.At(j)
	return w.Row != InvalidLARIndex, err
}

func (s *LARSolver) MakeSureBottomRightElementIsNonzeroInTableau(i, j uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	a := s.Core.RA
	if sourceDebugEnabled && (a.RowCount() != i+1 || a.ColumnCount() != j+1) {
		return ErrAssertion
	}
	if uint64(j) >= uint64(len(a.Columns)) {
		return ErrBounds
	}
	index := -1
	for k := len(a.Columns[j]); k > 0; {
		k--
		if a.Columns[j][k].Row == i {
			return nil
		}
		index = k
	}
	if sourceDebugEnabled && (index < 0 || uint32(index) == i) {
		return ErrAssertion
	}
	if index < 0 {
		return ErrBounds
	}
	return s.Core.RPrimal.TransposeRowsTableau(a.Columns[j][index].Row, i)
}

func (s *LARSolver) RemoveLastRowAndColumnFromTableau(j uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	c := s.Core
	a := c.RA
	r := c.RPrimal
	if sourceDebugEnabled && a.ColumnCount() != uint32(len(c.Costs.Data)) {
		return ErrAssertion
	}
	i := a.RowCount() - 1
	if err = s.MakeSureBottomRightElementIsNonzeroInTableau(i, j); err != nil {
		return err
	}
	if uint64(j) >= uint64(len(c.RHeading)) {
		return ErrBounds
	}
	if c.RHeading[j] < 0 {
		// The retained source ignores the bool, including empty eliminated rows.
		if _, err = r.PivotColumnTableau(j, i); err != nil {
			return err
		}
	}
	if uint64(j) >= uint64(len(c.Costs.Data)) || uint64(i) >= uint64(len(a.Rows)) {
		return ErrBounds
	}
	cost := &c.Costs.Data[j]
	nonzero, e := s.manager.IsZero(cost)
	if e != nil {
		return e
	}
	nonzero = !nonzero
	if nonzero {
		s.rationalWork = NewIndexedRationalVector(s.manager, 2)
	}
	for len(a.Rows[i]) > 0 {
		k := len(a.Rows[i]) - 1
		cell := &a.Rows[i][k]
		if nonzero {
			if uint64(cell.Column) >= uint64(len(r.D.Data)) {
				return ErrBounds
			}
			before, e := s.calls.begin(s.manager)
			if e != nil {
				return e
			}
			tmp, product := &s.rationalWork.Data[0], &s.rationalWork.Data[1]
			e = etaRationalProduct(s.manager, tmp, product, cost, &cell.Value)
			if e == nil {
				dst := &r.D.Data[cell.Column]
				e = s.manager.Add(dst, product, dst)
			}
			if e == nil {
				e = s.manager.Del(product)
				if e == nil {
					*product = numeral.NewRational()
				}
			}
			if err = s.calls.after(s.manager, before, e); err != nil {
				return err
			}
		}
		// The matrix owns the reached element destruction and its numeric calls.
		if err = a.RemoveElement(i, uint32(k)); err != nil {
			return err
		}
	}
	if uint64(j) >= uint64(len(a.Columns)) {
		return ErrBounds
	}
	if sourceDebugEnabled && len(a.Columns[j]) != 0 {
		return ErrAssertion
	}
	a.Rows = a.Rows[:len(a.Rows)-1]
	a.Columns = a.Columns[:len(a.Columns)-1]
	if len(c.RightSides.Data) == 0 {
		return ErrBounds
	}
	if err = c.RightSides.resizeData(uint32(len(c.RightSides.Data) - 1)); err != nil {
		return err
	}
	if s.rationalWork != nil {
		if err = s.rationalWork.Close(); err != nil {
			return err
		}
		s.rationalWork = nil
	}
	return nil
}

func (s *LARSolver) RemoveLastColumnFromTableau(j uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	a := s.Core.RA
	if sourceDebugEnabled && j != a.ColumnCount()-1 {
		return ErrAssertion
	}
	if uint64(j) >= uint64(len(a.Columns)) {
		return ErrBounds
	}
	if sourceDebugEnabled && len(a.Columns[j]) != 0 {
		return ErrAssertion
	}
	a.Columns = a.Columns[:len(a.Columns)-1]
	return nil
}

func (s *LARSolver) RemoveLastColumnFromBasisTableau(j uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	c := s.Core
	if uint64(j) >= uint64(len(c.RHeading)) {
		return ErrBounds
	}
	i := c.RHeading[j]
	if i >= 0 {
		last := len(c.RBasis) - 1
		if sourceDebugEnabled && last < 0 {
			return ErrAssertion
		}
		if last < 0 {
			return ErrBounds
		}
		if int64(i) != int64(last) {
			if uint64(i) >= uint64(len(c.RBasis)) {
				return ErrBounds
			}
			col := c.RBasis[last]
			c.RBasis[i] = col
			if uint64(col) >= uint64(len(c.RHeading)) {
				return ErrBounds
			}
			c.RHeading[col] = i
		}
		c.RBasis = c.RBasis[:last]
	} else {
		last := len(c.RNBasis) - 1
		if sourceDebugEnabled && last < 0 {
			return ErrAssertion
		}
		if last < 0 {
			return ErrBounds
		}
		i = -1 - i
		if int64(i) != int64(last) {
			if uint64(i) >= uint64(len(c.RNBasis)) {
				return ErrBounds
			}
			col := c.RNBasis[last]
			c.RNBasis[i] = col
			if uint64(col) >= uint64(len(c.RHeading)) {
				return ErrBounds
			}
			c.RHeading[col] = -i - 1
		}
		c.RNBasis = c.RNBasis[:last]
	}
	c.RHeading = c.RHeading[:len(c.RHeading)-1]
	if sourceDebugEnabled {
		if uint32(len(c.RBasis)) != c.RA.RowCount() {
			return ErrAssertion
		}
		ok, e := c.RPrimal.BasisHeadingIsCorrect()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
	}
	return nil
}

func (s *LARSolver) RemoveColumnFromTableau(j uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	c := s.Core
	if sourceDebugEnabled && (j != c.RA.ColumnCount()-1 || c.RA.ColumnCount() != uint32(len(c.Costs.Data))) {
		return ErrAssertion
	}
	row, e := s.ColumnRepresentsRowInTableau(j)
	if e != nil {
		return e
	}
	if row {
		if err = s.RemoveLastRowAndColumnFromTableau(j); err != nil {
			return err
		}
		if uint64(j) >= uint64(len(c.RHeading)) {
			return ErrBounds
		}
		if c.RHeading[j] < 0 {
			i := c.RA.RowCount()
			if uint64(i) >= uint64(len(c.RBasis)) {
				return ErrBounds
			}
			if err = c.RPrimal.ChangeBasisUnconditionally(j, c.RBasis[i]); err != nil {
				return err
			}
		}
	} else {
		if err = s.RemoveLastColumnFromTableau(j); err != nil {
			return err
		}
	}
	if len(c.RX.Data) == 0 {
		return ErrBounds
	}
	if err = c.RX.resizeData(uint32(len(c.RX.Data) - 1)); err != nil {
		return err
	}
	if len(c.RPrimal.D.Data) == 0 {
		return ErrBounds
	}
	if err = c.RPrimal.D.resizeData(uint32(len(c.RPrimal.D.Data) - 1)); err != nil {
		return err
	}
	if len(c.Costs.Data) == 0 {
		return ErrBounds
	}
	if err = c.Costs.resizeData(uint32(len(c.Costs.Data) - 1)); err != nil {
		return err
	}
	if err = s.RemoveLastColumnFromBasisTableau(j); err != nil {
		return err
	}
	if sourceDebugEnabled {
		ok, e := c.RBasisIsOK()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
		if c.RA.ColumnCount() != uint32(len(c.Costs.Data)) {
			return ErrAssertion
		}
	}
	return nil
}

// PopTableau runs after the outer column directory has been shortened. It
// removes numeric columns only; witnesses, bounds and model scopes are separate.
func (s *LARSolver) PopTableau() (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	check := func() error {
		if !sourceDebugEnabled {
			return nil
		}
		c := s.Core
		if len(c.Costs.Data) != int(c.RA.ColumnCount()) || len(c.RBasis) != int(c.RA.RowCount()) {
			return ErrAssertion
		}
		ok, e := c.RPrimal.BasisHeadingIsCorrect()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
		return nil
	}
	if err = check(); err != nil {
		return err
	}
	for j := s.Core.RA.ColumnCount(); uint64(j) > uint64(len(s.ColumnsToExternal)); {
		j--
		if err = s.RemoveColumnFromTableau(j); err != nil {
			return err
		}
	}
	return check()
}
