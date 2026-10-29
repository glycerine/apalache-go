package lp

// Source: src/util/lp/lar_solver.h:1069-1081,1347-1357.

import "github.com/glycerine/g3/pkg/util/numeral"

// GetLeftSideVal evaluates the actual polymorphic constraint, including its
// free coefficient, in a borrowed concrete model. Output copying follows the
// complete sum. On error, close a borrowed constraint before this solver if its
// own coefficient/free-value operation stopped while borrowing an output local.
// The borrowed model must stay alive through solver cleanup after an error.
func (s *LARSolver) GetLeftSideVal(c LARStoredConstraint, model *LARRationalModel, out *numeral.Rational) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	if err = s.prepareConstraintValue(c, model); err != nil {
		return err
	}
	before, e := s.calls.begin(s.manager)
	if e != nil {
		return e
	}
	e = s.manager.Copy(out, &s.rationalWork.Data[0])
	if err = s.calls.after(s.manager, before, e); err != nil {
		return err
	}
	return s.closeConstraintValue()
}

func (s *LARSolver) ConstraintHolds(c LARStoredConstraint, model *LARRationalModel) (holds bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	if err = s.prepareConstraintValue(c, model); err != nil {
		return false, err
	}
	kind := c.Base().Kind
	switch kind {
	case LConstraintLE, LConstraintLT, LConstraintGE, LConstraintGT, LConstraintEQ:
	default:
		if sourceDebugEnabled {
			return false, ErrAssertion
		}
		return false, s.closeConstraintValue()
	}
	before, e := s.calls.begin(s.manager)
	if e != nil {
		return false, e
	}
	cmp, e := s.manager.Compare(&s.rationalWork.Data[0], &c.Base().RightSide)
	if err = s.calls.after(s.manager, before, e); err != nil {
		return false, err
	}
	switch kind {
	case LConstraintLE:
		holds = cmp <= 0
	case LConstraintLT:
		holds = cmp < 0
	case LConstraintGE:
		holds = cmp >= 0
	case LConstraintGT:
		holds = cmp > 0
	case LConstraintEQ:
		holds = cmp == 0
	}
	return holds, s.closeConstraintValue()
}

func (s *LARSolver) prepareConstraintValue(c LARStoredConstraint, model *LARRationalModel) (err error) {
	if c == nil || model == nil || c.Base().manager != s.manager || model.manager != s.manager {
		return ErrExactVectorOwner
	}
	if err = c.Base().ready(); err != nil {
		return err
	}
	if model.stopped || model.calls.pending() {
		return ErrExactVectorPending
	}
	s.rationalWork = NewIndexedRationalVector(s.manager, 2)
	w := s.rationalWork.Data // source ret and lexical product.
	if err = c.FreeCoefficient(&w[0]); err != nil {
		return err
	}
	s.validationCoefficients, err = c.CoefficientsAsVector()
	if err != nil {
		return err
	}
	for i := range s.validationCoefficients.Data {
		cell := &s.validationCoefficients.Data[i]
		value, found := model.Values[cell.Index]
		if !found || value == nil {
			if sourceDebugEnabled {
				return ErrAssertion
			}
			return ErrBounds
		}
		before, e := s.calls.begin(s.manager)
		if e != nil {
			return e
		}
		err = func() error {
			if e := s.manager.Mul(&cell.Value, value, &w[1]); e != nil {
				return e
			}
			if e := s.manager.Add(&w[0], &w[1], &w[0]); e != nil {
				return e
			}
			if e := s.manager.Del(&w[1]); e != nil {
				return e
			}
			w[1] = numeral.NewRational()
			return nil
		}()
		if err = s.calls.after(s.manager, before, err); err != nil {
			return err
		}
	}
	if err = s.validationCoefficients.Close(); err != nil {
		return err
	}
	s.validationCoefficients = nil
	return nil
}

func (s *LARSolver) closeConstraintValue() error {
	if err := s.rationalWork.Close(); err != nil {
		return err
	}
	s.rationalWork = nil
	return nil
}
