package smtcore

// Source: src/smt/smt_context.cpp assume_eq and set_true_first_flag.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *SMTInternalizerState) setScopedTrueFirst(variable int32) (bool, error) {
	data, err := s.BooleanData(variable)
	if err != nil || data.TryTrueFirst() {
		return false, err
	}
	s.trueFirstTrail = append(s.trueFirstTrail, variable)
	data.SetTrueFirst()
	return true, nil
}

// An interface equality is a relevant case split with a preferred phase,
// never an asserted consequence of an incidental arithmetic model value.
func (s *SMTInternalizerState) assumeEquality(left, right *SMTEnode) (progress bool, result error) {
	if left == nil || right == nil {
		return false, ast.ErrASTDomain
	}
	if left.root == right.root {
		return false, nil
	}
	equality, err := s.manager.MkEq(left.owner, right.owner)
	if err != nil || s.manager.IsFalse(equality) {
		return false, err
	}
	ref, err := s.manager.NewRef(equality)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, s.manager.DelRef(&ref)) }()
	variable, internalized := s.BoolVar(equality)
	if !internalized {
		// Set the phase before the theory installs its auxiliary bound atoms.
		if err = s.internalizeFormulaCore(equality, true); err != nil {
			return false, err
		}
		variable, _ = s.BoolVar(equality)
		data, err := s.BooleanData(variable)
		if err != nil {
			return false, err
		}
		data.SetEquality()
		if _, err = s.setScopedTrueFirst(variable); err != nil {
			return false, err
		}
		sort, err := s.manager.GetSort(left.owner)
		if err != nil {
			return false, err
		}
		family, err := s.manager.GetFamilyID(sort)
		if err != nil {
			return false, err
		}
		if theory := s.theories[family]; theory != nil {
			if err = theory.InternalizeEquality(equality, variable); err != nil {
				return false, err
			}
		}
		s.statistics.NumInterfaceEqs++
		progress = true
	}
	changed, err := s.setScopedTrueFirst(variable)
	if err != nil {
		return false, err
	}
	progress = progress || changed
	assignment, err := s.Assignment(NewSMTLiteral(variable, false))
	if err != nil {
		return false, err
	}
	progress = progress || assignment == lbool.Undef
	if s.relevancy != nil && !s.relevancy.IsRelevant(equality) {
		if err = s.markRelevant(equality); err != nil {
			return false, err
		}
		progress = true
	}
	if progress {
		s.theoryProgress++
	}
	return progress, nil
}
