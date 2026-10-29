package smtcore

// Source: src/smt/smt_context.cpp theory setup, search initialization,
// restart, and round-robin final-check dispatch.

import "github.com/glycerine/g3/pkg/ast"

func (s *SMTInternalizerState) SetupTheories() error {
	if err := s.open(); err != nil {
		return err
	}
	if s.theoriesSetup {
		return nil
	}
	for _, theory := range s.theoryOrder {
		if err := theory.Setup(); err != nil {
			return err
		}
	}
	s.theoriesSetup = true
	return nil
}

func (s *SMTInternalizerState) InitializeTheorySearch() error {
	if err := s.SetupTheories(); err != nil {
		return err
	}
	for _, theory := range s.theoryOrder {
		if err := theory.InitializeSearch(); err != nil {
			return err
		}
	}
	if s.quantifierManager != nil {
		if err := s.quantifierManager.InitSearch(); err != nil {
			return err
		}
	}
	if err := s.dynamicAck.initSearch(); err != nil {
		return err
	}
	s.incompleteTheories = s.incompleteTheories[:0]
	s.finalCheckIndex = 0
	return nil
}

func (s *SMTInternalizerState) RestartTheories() error {
	if err := s.open(); err != nil {
		return err
	}
	if s.inconsistent {
		return nil
	}
	for _, theory := range s.theoryOrder {
		if s.inconsistent {
			break
		}
		if err := theory.Restart(); err != nil {
			return err
		}
	}
	if !s.inconsistent && s.quantifierManager != nil && s.quantifierManager.mbqi {
		if err := s.quantifierManager.modelFinder.Restart(); err != nil {
			return err
		}
		if s.quantifierManager.modelChecker != nil {
			return s.quantifierManager.modelChecker.Restart()
		}
	}
	return nil
}

func (s *SMTInternalizerState) FinalCheckTheories() (SMTFinalCheckStatus, error) {
	if err := s.open(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	s.statistics.NumFinalChecks++
	if s.quantifierManager != nil {
		status, err := s.quantifierManager.FinalCheck(false)
		if err != nil || status != SMTFinalCheckDone {
			return status, err
		}
	}
	s.incompleteTheories = s.incompleteTheories[:0]
	count := len(s.theoryOrder)
	// Source includes the full quantifier check as a rotating slot even when
	// no quantifiers are registered. CONTINUE resumes at the following slot.
	rangeSize := count + 1
	start := s.finalCheckIndex % rangeSize
	result := SMTFinalCheckDone
	for offset := 0; offset < rangeSize; offset++ {
		index := (start + offset) % rangeSize
		status := SMTFinalCheckDone
		var err error
		if index < count {
			status, err = s.theoryOrder[index].FinalCheck()
		} else if s.quantifierManager != nil {
			status, err = s.quantifierManager.FinalCheck(true)
		}
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		s.finalCheckIndex = (index + 1) % rangeSize
		switch status {
		case SMTFinalCheckDone:
		case SMTFinalCheckContinue:
			return status, nil
		case SMTFinalCheckGiveUp:
			result = status
			if index < count {
				s.incompleteTheories = append(s.incompleteTheories, s.theoryOrder[index])
			}
		default:
			return SMTFinalCheckGiveUp, ast.ErrASTDomain
		}
	}
	if s.canPropagate() {
		return SMTFinalCheckContinue, nil
	}
	return result, nil
}

// Source context::can_propagate checks every logical-context queue, not only
// the theories' callbacks. New assignments/equalities must run before give-up.
func (s *SMTInternalizerState) canPropagate() bool {
	if s.propagationHead != len(s.assignedLiterals) || s.atomPropagationHead != len(s.assignedLiterals) ||
		s.pendingHead != len(s.pending) || s.theoryEqualityHead != len(s.theoryEqualities) ||
		s.theoryDisequalityHead != len(s.theoryDisequalities) ||
		s.relevancy != nil && s.relevancy.CanPropagate() ||
		s.quantifierManager != nil && s.quantifierManager.CanPropagate() {
		return true
	}
	for _, theory := range s.theoryOrder {
		if theory.CanPropagate() {
			return true
		}
	}
	return false
}

func (s *SMTInternalizerState) IncompleteTheories() []SMTTheoryInternalizer {
	if s == nil {
		return nil
	}
	return append([]SMTTheoryInternalizer(nil), s.incompleteTheories...)
}
