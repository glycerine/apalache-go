package lp

// Source: src/util/lp/lar_solver.h:456-463,1090-1139,1158-1202,1242-1275.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"os"
)

func (s *LARSolver) checkExplanationInput(e *SparseRationalVector) error {
	if e == nil || e.manager != s.manager {
		return ErrExactVectorOwner
	}
	return e.ready()
}

func (s *LARSolver) TheRelationsAreOfSameType(e *SparseRationalVector) (same bool, kind LConstraintKind, err error) {
	if err = s.ready(); err != nil {
		return false, LConstraintEQ, err
	}
	defer func() { s.stopped = err != nil }()
	if err = s.checkExplanationInput(e); err != nil {
		return false, LConstraintEQ, err
	}
	s.rationalWork = NewIndexedRationalVector(s.manager, 1)
	var greater, less uint32
	strict := false
	for i := range e.Data {
		cell := &e.Data[i]
		if err = s.rationalWork.copySlot(&s.rationalWork.Data[0], &cell.Value); err != nil {
			return false, LConstraintEQ, err
		}
		if uint64(cell.Index) >= uint64(len(s.Constraints)) {
			return false, LConstraintEQ, ErrBounds
		}
		k := s.Constraints[cell.Index].Base().Kind
		positive, x := s.manager.IsPos(&s.rationalWork.Data[0])
		if x != nil {
			return false, LConstraintEQ, x
		}
		if !positive {
			k = FlipConstraintKind(k)
		}
		if k == LConstraintGT || k == LConstraintLT {
			strict = true
		}
		if k == LConstraintGE || k == LConstraintGT {
			greater++
		} else if k == LConstraintLE || k == LConstraintLT {
			less++
		}
		if err = s.manager.Del(&s.rationalWork.Data[0]); err != nil {
			return false, LConstraintEQ, err
		}
		s.rationalWork.Data[0] = numeral.NewRational()
	}
	kind = LConstraintEQ
	if greater != 0 {
		kind = LConstraintGE
	} else if less != 0 {
		kind = LConstraintLE
	}
	if strict {
		kind /= 2
	}
	return greater == 0 || less == 0, kind, s.closeConstraintValue()
}

// SumOfRightSidesOfExplanation subtracts each constraint's original free
// coefficient, including term constants, before multiplying by its evidence.
// The caller's output must outlive solver cleanup after an interrupted copy.
func (s *LARSolver) SumOfRightSidesOfExplanation(e *SparseRationalVector, out *numeral.Rational) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if err = s.checkExplanationInput(e); err != nil {
		return err
	}
	if out == nil {
		return ErrBounds
	}
	s.rationalWork = NewIndexedRationalVector(s.manager, 5)
	w := s.rationalWork.Data // ret, coefficient, free coefficient, difference, product.
	for i := range e.Data {
		cell := &e.Data[i]
		if err = s.rationalWork.copySlot(&w[1], &cell.Value); err != nil {
			return err
		}
		if uint64(cell.Index) >= uint64(len(s.Constraints)) {
			if sourceDebugEnabled {
				return ErrAssertion
			}
			return ErrBounds
		}
		c := s.Constraints[cell.Index]
		if err = c.FreeCoefficient(&w[2]); err != nil {
			return err
		}
		before, x := s.calls.begin(s.manager)
		if x != nil {
			return x
		}
		err = s.manager.Sub(&c.Base().RightSide, &w[2], &w[3])
		if err == nil {
			err = s.manager.Mul(&w[3], &w[1], &w[4])
		}
		if err == nil {
			err = s.manager.Add(&w[0], &w[4], &w[0])
		}
		if err = s.calls.after(s.manager, before, err); err != nil {
			return err
		}
		for k := 4; k > 0; k-- {
			if err = s.manager.Del(&w[k]); err != nil {
				return err
			}
			w[k] = numeral.NewRational()
		}
	}
	if err = s.rationalWork.copySlot(out, &w[0]); err != nil {
		return err
	}
	return s.closeConstraintValue()
}

func (s *LARSolver) TheLeftSidesSumToZero(e *SparseRationalVector) (zero bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	if err = s.checkExplanationInput(e); err != nil {
		return false, err
	}
	s.explanationCoefficients = NewLARRationalModel(s.manager)
	s.rationalWork = NewIndexedRationalVector(s.manager, 2)
	w := s.rationalWork.Data // copied evidence coefficient, lexical product.
	for i := range e.Data {
		cell := &e.Data[i]
		if err = s.rationalWork.copySlot(&w[0], &cell.Value); err != nil {
			return false, err
		}
		if uint64(cell.Index) >= uint64(len(s.Constraints)) {
			if sourceDebugEnabled {
				return false, ErrAssertion
			}
			return false, ErrBounds
		}
		s.validationCoefficients, err = s.Constraints[cell.Index].CoefficientsAsVector()
		if err != nil {
			return false, err
		}
		for k := range s.validationCoefficients.Data {
			coefficient := &s.validationCoefficients.Data[k]
			before, x := s.calls.begin(s.manager)
			if x != nil {
				return false, x
			}
			err = s.manager.Mul(&coefficient.Value, &w[0], &w[1])
			if err = s.calls.after(s.manager, before, err); err != nil {
				return false, err
			}
			value, found := s.explanationCoefficients.Values[coefficient.Index]
			if !found {
				// Source retains a newly inserted zero. Only an existing sum
				// is erased after addition cancels it, even when both are zero.
				if err = s.explanationCoefficients.set(coefficient.Index, &w[1]); err != nil {
					return false, err
				}
			} else {
				before, x = s.calls.begin(s.manager)
				if x != nil {
					return false, x
				}
				err = s.manager.Add(value, &w[1], value)
				if err = s.calls.after(s.manager, before, err); err != nil {
					return false, err
				}
				isZero, x := s.manager.IsZero(value)
				if x != nil {
					return false, x
				}
				if isZero {
					if err = s.manager.Del(value); err != nil {
						return false, err
					}
					delete(s.explanationCoefficients.Values, coefficient.Index)
				}
			}
			if err = s.manager.Del(&w[1]); err != nil {
				return false, err
			}
			w[1] = numeral.NewRational()
		}
		if err = s.validationCoefficients.Close(); err != nil {
			return false, err
		}
		s.validationCoefficients = nil
		if err = s.manager.Del(&w[0]); err != nil {
			return false, err
		}
		w[0] = numeral.NewRational()
	}
	zero = len(s.explanationCoefficients.Values) == 0
	if err = s.closeConstraintValue(); err != nil {
		return false, err
	}
	if !zero {
		if err = writeConstraintText(os.Stdout, "left side = "); err != nil {
			return false, err
		}
		s.validationCoefficients = NewSparseRationalVector(s.manager)
		for column, value := range s.explanationCoefficients.Values {
			if err = s.validationCoefficients.appendTermCoefficient(column, value); err != nil {
				return false, err
			}
		}
		s.rationalWork = NewIndexedRationalVector(s.manager, 3)
		if err = s.printColumnCombination(s.validationCoefficients.Data, os.Stdout); err != nil {
			return false, err
		}
		if err = writeConstraintText(os.Stdout, "\n"); err != nil {
			return false, err
		}
		if err = s.closeConstraintValue(); err != nil {
			return false, err
		}
		if err = s.validationCoefficients.Close(); err != nil {
			return false, err
		}
		s.validationCoefficients = nil
	}
	if err = s.explanationCoefficients.Close(); err != nil {
		return false, err
	}
	s.explanationCoefficients = nil
	return zero, nil
}

func (s *LARSolver) ExplanationIsCorrect(e *SparseRationalVector) (correct bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	if !sourceDebugEnabled {
		return true, nil
	}
	same, kind, x := s.TheRelationsAreOfSameType(e)
	if x != nil {
		return false, x
	}
	if !same {
		return false, ErrAssertion
	}
	zero, x := s.TheLeftSidesSumToZero(e)
	if x != nil {
		return false, x
	}
	if !zero {
		return false, ErrAssertion
	}
	// The nested sum needs a ready solver, so its result is a separate lexical
	// owner until it returns. Retain that owner if its final assignment stops.
	value := numeral.NewRational()
	if err = s.SumOfRightSidesOfExplanation(e, &value); err != nil {
		// Sum's initialized output may own digits even after a partial copy.
		s.certificateResult = &value
		return false, err
	}
	s.certificateResult = &value
	sign, x := s.manager.Sign(&value)
	if x != nil {
		return false, x
	}
	valid := false
	switch kind {
	case LConstraintLE:
		valid = sign < 0
	case LConstraintLT:
		valid = sign <= 0
	case LConstraintGE:
		valid = sign > 0
	case LConstraintGT:
		valid = sign >= 0
	case LConstraintEQ:
		valid = sign != 0
	default:
		return false, ErrAssertion
	}
	if !valid {
		return false, ErrAssertion
	}
	if err = s.manager.Del(s.certificateResult); err != nil {
		return false, err
	}
	s.certificateResult = nil
	return true, nil
}

// GetInfeasibilityExplanation clears the caller's vector first. On failure,
// close that vector before the solver: a pending append can borrow its local.
func (s *LARSolver) GetInfeasibilityExplanation(out *SparseRationalVector) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if err = s.checkExplanationInput(out); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	if s.Bounds.InfeasibleColumn.Value != -1 {
		return s.FillExplanationFromInfeasibleColumn(out)
	}
	if s.Core.InfeasibleSumSign == 0 {
		return nil
	}
	if err = s.GetInfeasibilityExplanationForInfSign(out, s.Core.Infeasibility, s.Core.InfeasibleSumSign); err != nil {
		return err
	}
	if sourceDebugEnabled {
		ok, x := s.ExplanationIsCorrect(out)
		if x != nil {
			return x
		}
		if !ok {
			return ErrAssertion
		}
	}
	return nil
}

func (s *LARSolver) FillExplanationFromInfeasibleColumn(out *SparseRationalVector) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if err = s.checkExplanationInput(out); err != nil {
		return err
	}
	index := s.Bounds.InfeasibleColumn.Value
	if index < 0 || uint64(index) >= uint64(s.Bounds.Witnesses.Size()) {
		return ErrBounds
	}
	witness, x := s.Bounds.Witnesses.At(uint32(index))
	if x != nil {
		return x
	}
	s.rationalWork = NewIndexedRationalVector(s.manager, 2)
	if err = s.manager.SetInt32(&s.rationalWork.Data[0], 1); err != nil {
		return err
	}
	if err = out.appendTermCoefficient(witness.UpperBoundWitness, &s.rationalWork.Data[0]); err != nil {
		return err
	}
	if err = s.manager.SetInt32(&s.rationalWork.Data[1], -1); err != nil {
		return err
	}
	if err = out.appendTermCoefficient(witness.LowBoundWitness, &s.rationalWork.Data[1]); err != nil {
		return err
	}
	return s.closeConstraintValue()
}

func (s *LARSolver) GetInfeasibilityExplanationForInfSign(out, row *SparseRationalVector, sign int32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if err = s.checkExplanationInput(out); err != nil {
		return err
	}
	if err = s.checkExplanationInput(row); err != nil {
		return err
	}
	s.rationalWork = NewIndexedRationalVector(s.manager, 1)
	for i := range row.Data {
		cell := &row.Data[i]
		if err = s.rationalWork.copySlot(&s.rationalWork.Data[0], &cell.Value); err != nil {
			return err
		}
		positive, x := s.manager.IsPos(&s.rationalWork.Data[0])
		if x != nil {
			return x
		}
		adjusted := sign
		if !positive {
			adjusted = -sign
		}
		witness, x := s.Bounds.Witnesses.At(cell.Index)
		if x != nil {
			return x
		}
		ci := witness.LowBoundWitness
		if adjusted < 0 {
			ci = witness.UpperBoundWitness
		}
		if sourceDebugEnabled && uint64(ci) >= uint64(len(s.Constraints)) {
			return ErrAssertion
		}
		if err = out.appendTermCoefficient(ci, &s.rationalWork.Data[0]); err != nil {
			return err
		}
		if err = s.manager.Del(&s.rationalWork.Data[0]); err != nil {
			return err
		}
		s.rationalWork.Data[0] = numeral.NewRational()
	}
	return s.closeConstraintValue()
}

func (s *LARSolver) InfExplanationIsCorrect() (correct bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	if !sourceDebugEnabled {
		return true, nil
	}
	s.validationExplanation = NewSparseRationalVector(s.manager)
	if err = s.GetInfeasibilityExplanation(s.validationExplanation); err != nil {
		return false, err
	}
	correct, err = s.ExplanationIsCorrect(s.validationExplanation)
	if err != nil {
		return false, err
	}
	if err = s.validationExplanation.Close(); err != nil {
		return false, err
	}
	s.validationExplanation = nil
	return correct, nil
}
