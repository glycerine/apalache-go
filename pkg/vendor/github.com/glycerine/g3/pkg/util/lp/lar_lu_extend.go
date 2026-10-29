package lp

// Source: src/util/lp/lar_solver.h:902-925; retained exact and floating cores.

func (s *LARSolver) AddLastRowsToPairLU(r *PairPrimalCoreSolver) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if r == nil {
		return ErrBounds
	}
	if err = r.ready(); err != nil {
		return err
	}
	if f := r.Factorization; f != nil {
		columns, e := f.ColumnsToReplaceForAddLastRows(*r.Heading)
		if e != nil {
			return e
		}
		if uint64(f.RefactorCounter)+uint64(len(columns)) >= 200 || f.HasDenseSubmatrix() {
			if err = f.Close(); err != nil {
				return err
			}
			r.Factorization = nil
		} else {
			if err = f.AddLastRowsToB(*r.Heading, columns); err != nil {
				return err
			}
		}
	}
	if r.Factorization == nil {
		if err = InitPairFactorization(&r.Factorization, r.A, *r.Basis, s.Settings); err != nil {
			return err
		}
		if r.Factorization.GetStatus() != LUOK {
			if err = r.Factorization.Close(); err != nil {
				return err
			}
			r.Factorization = nil
		}
	}
	return nil
}

func (s *LARSolver) AddLastRowsToFloatLU(r *FloatPrimalCoreSolver) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if r == nil {
		return ErrBounds
	}
	if f := r.Factorization; f != nil {
		columns, e := f.ColumnsToReplaceForAddLastRows(*r.Heading)
		if e != nil {
			return e
		}
		if uint64(f.RefactorCounter)+uint64(len(columns)) >= 200 || f.HasDenseSubmatrix() {
			f.Close()
			r.Factorization = nil
		} else {
			if err = f.AddLastRowsToB(*r.Heading, columns); err != nil {
				return err
			}
		}
	}
	if r.Factorization == nil {
		if err = InitFloatFactorization(&r.Factorization, r.A, *r.Basis, s.Settings); err != nil {
			return err
		}
		if r.Factorization.GetStatus() != LUOK {
			r.Factorization.Close()
			r.Factorization = nil
		}
	}
	return nil
}
