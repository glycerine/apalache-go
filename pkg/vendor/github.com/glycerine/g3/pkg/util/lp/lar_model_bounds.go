package lp

// Source: src/util/lp/init_lar_solver.h:169-229; concrete registered LAR bounds.

import "github.com/glycerine/g3/pkg/util/numeral"

func (s *LARSolver) NumberOfConstraints() uint32 { return uint32(len(s.Constraints)) }
func (s *LARSolver) Constraint(ci uint32) (LARStoredConstraint, error) {
	if uint64(ci) >= uint64(len(s.Constraints)) {
		return nil, ErrBounds
	}
	return s.Constraints[ci], nil
}
func (s *LARSolver) Value(j uint32) (*RationalPair, error) {
	if uint64(j) >= uint64(len(s.Core.RX.Data)) {
		return nil, ErrBounds
	}
	return &s.Core.RX.Data[j], nil
}
func (s *LARSolver) subtractTermConstant(rhs, value *numeral.Rational) (err error) {
	before, err := s.calls.begin(s.manager)
	if err != nil {
		return err
	}
	defer func() { err = s.calls.after(s.manager, before, err) }()
	s.rationalWork = NewIndexedRationalVector(s.manager, 2)
	tmp, out := &s.rationalWork.Data[0], &s.rationalWork.Data[1]
	if err = s.manager.Copy(tmp, rhs); err != nil {
		return err
	}
	if err = s.manager.Sub(tmp, value, tmp); err != nil {
		return err
	}
	if err = s.manager.Copy(out, tmp); err != nil {
		return err
	}
	if err = s.manager.Del(tmp); err != nil {
		return err
	}
	*tmp = numeral.NewRational()
	return nil
}
func (s *LARSolver) AddVarBound(j uint32, kind LConstraintKind, rhs *numeral.Rational) (ci uint32, err error) {
	if err = s.ready(); err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			s.stopped = true
		}
	}()
	ci = uint32(len(s.Constraints))
	if !s.IsTerm(j) {
		s.pendingConstraint, err = NewLARVarConstraint(s.manager, j, kind, rhs)
		if err != nil {
			return ci, err
		}
		s.Constraints = append(s.Constraints, s.pendingConstraint)
		s.pendingConstraint = nil
		if err = s.Bounds.Update(j, kind, rhs, ci); err != nil {
			return ci, err
		}
	} else {
		term := s.OriginalTerms[j-LARTermStartIndex]
		column, registered := s.ExternalToColumns[j]
		if !registered {
			if err = s.AddRowFromTermNoConstraint(term, j); err != nil {
				return ci, err
			}
			column = s.Core.RA.ColumnCount() - 1
		}
		if err = s.subtractTermConstant(rhs, &term.Value); err != nil {
			return ci, err
		}
		shifted := &s.rationalWork.Data[1]
		if registered {
			s.pendingConstraint, err = NewLARTermConstraint(s.manager, term, kind, rhs)
			if err != nil {
				return ci, err
			}
			s.Constraints = append(s.Constraints, s.pendingConstraint)
			s.pendingConstraint = nil
		}
		if err = s.Bounds.Update(column, kind, shifted, ci); err != nil {
			return ci, err
		}
		if !registered {
			s.pendingConstraint, err = NewLARTermConstraint(s.manager, term, kind, rhs)
			if err != nil {
				return ci, err
			}
			s.Constraints = append(s.Constraints, s.pendingConstraint)
			s.pendingConstraint = nil
			if sourceDebugEnabled && s.Core.RA.ColumnCount() != uint32(len(*s.Core.RPrimal.Costs)) {
				return ci, ErrAssertion
			}
		}
		if err = s.rationalWork.Close(); err != nil {
			return ci, err
		}
		s.rationalWork = nil
	}
	_, err = s.SizesAreCorrect()
	return ci, err
}
