package lp

// Source: src/util/lp/lar_solver.h:776-795; exact basic-value delta updates.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *LARSolver) changeBasicValueByCoefficient(row uint32, coefficient *numeral.Rational, delta *RationalPair, trackCost bool) (err error) {
	c := s.Core
	if uint64(row) >= uint64(len(c.RBasis)) {
		return ErrBounds
	}
	j := c.RBasis[row]
	if uint64(j) >= uint64(len(c.RX.Data)) {
		return ErrBounds
	}
	before, e := s.calls.begin(s.manager)
	if e != nil {
		return e
	}
	product := &s.work.Data[0]
	tmp, px, py := &s.rationalWork.Data[0], &s.rationalWork.Data[1], &s.rationalWork.Data[2]
	e = etaRationalProduct(s.manager, tmp, px, coefficient, &delta.X)
	if e == nil {
		e = etaRationalProduct(s.manager, tmp, py, coefficient, &delta.Y)
	}
	if e == nil {
		e = s.manager.Copy(&product.X, px)
	}
	if e == nil {
		e = s.manager.Copy(&product.Y, py)
	}
	if e == nil {
		x := &c.RX.Data[j]
		e = x.Sub(s.manager, x, product)
	}
	if e == nil {
		e = product.Del(s.manager)
	}
	if e == nil {
		e = s.manager.Del(py)
		if e == nil {
			*py = numeral.NewRational()
		}
	}
	if e == nil {
		e = s.manager.Del(px)
		if e == nil {
			*px = numeral.NewRational()
		}
	}
	if err = s.calls.after(s.manager, before, e); err != nil {
		return err
	}
	if trackCost {
		if err = s.BasicColumnsWithChangedCost.Insert(j); err != nil {
			return err
		}
	}
	// The core owns any comparison refusal during membership maintenance.
	return c.RPrimal.UpdateColumnInInfSet(j)
}

// ChangeBasicXByDeltaOnColumn preserves the source row/solve order. Delta is
// borrowed, and both numeric children retain their own unfinished operations.
func (s *LARSolver) ChangeBasicXByDeltaOnColumn(j uint32, delta *RationalPair) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if delta == nil {
		return ErrBounds
	}
	c := s.Core
	tableau := s.Settings.UseTableau()
	if tableau {
		if uint64(j) >= uint64(len(c.RA.Columns)) {
			return ErrBounds
		}
	} else {
		if err = s.ColumnBuffer.Clear(); err != nil {
			return err
		}
		if err = s.ColumnBuffer.Resize(c.RA.RowCount()); err != nil {
			return err
		}
		if err = c.RPrimal.SolveBdIndexed(j, s.ColumnBuffer); err != nil {
			return err
		}
	}
	s.work = NewIndexedPairVector(s.manager, 1)
	s.rationalWork = NewIndexedRationalVector(s.manager, 3)
	if tableau {
		for _, cell := range c.RA.Columns[j] {
			value, e := c.RA.ColumnValue(cell)
			if e != nil {
				return e
			}
			if err = s.changeBasicValueByCoefficient(cell.Row, value, delta, s.Settings.SimplexStrategy == StrategyTableauCosts); err != nil {
				return err
			}
		}
	} else {
		for _, i := range s.ColumnBuffer.Index {
			if uint64(i) >= uint64(len(s.ColumnBuffer.Data)) {
				return ErrBounds
			}
			if err = s.changeBasicValueByCoefficient(i, &s.ColumnBuffer.Data[i], delta, false); err != nil {
				return err
			}
		}
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
