package smtcore

// Source: src/smt/smt_context.cpp theory equality and disequality queues.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *SMTInternalizerState) enqueueTheoryEquality(theory SMTTheoryID, left, right SMTTheoryVar) error {
	if theory == SMTNullTheoryID || left == SMTNullTheoryVar || right == SMTNullTheoryVar || left == right || s.theories[theory] == nil {
		return ast.ErrASTDomain
	}
	s.theoryEqualities = append(s.theoryEqualities, SMTTheoryPropagation{Theory: theory, Left: left, Right: right})
	return nil
}

func (s *SMTInternalizerState) enqueueTheoryDisequality(theory SMTTheoryID, left, right SMTTheoryVar) error {
	plugin := s.theories[theory]
	if plugin == nil || theory == SMTNullTheoryID || left == SMTNullTheoryVar || right == SMTNullTheoryVar || left == right {
		return ast.ErrASTDomain
	}
	s.theoryDisequalities = append(s.theoryDisequalities, SMTTheoryPropagation{Theory: theory, Left: left, Right: right})
	return nil
}

func (s *SMTInternalizerState) enqueueExistingDisequalities(root *SMTEnode, variable SMTTheoryVar, plugin SMTTheoryInternalizer) error {
	if !plugin.UseDisequalities() {
		return nil
	}
	for _, parent := range root.parents {
		if parent == nil || !parent.IsEquality() || len(parent.arguments) != 2 {
			continue
		}
		boolVariable, found := s.boolVar[parent.owner]
		if !found {
			continue
		}
		value, err := s.Assignment(NewSMTLiteral(boolVariable, false))
		if err != nil {
			return err
		}
		if value != lbool.False {
			continue
		}
		left, right := parent.arguments[0], parent.arguments[1]
		if right.root == root {
			left, right = right, left
		}
		if left.root != root || right.root == root {
			continue
		}
		other := s.theoryVariableNear(right, plugin.FamilyID())
		if nearby := s.theoryVariableNear(left, plugin.FamilyID()); nearby != SMTNullTheoryVar {
			variable = nearby
		}
		if other != SMTNullTheoryVar && other != variable {
			if err = s.enqueueTheoryDisequality(plugin.FamilyID(), variable, other); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SMTInternalizerState) PendingTheoryEqualities() []SMTTheoryPropagation {
	if s == nil {
		return nil
	}
	return append([]SMTTheoryPropagation(nil), s.theoryEqualities[s.theoryEqualityHead:]...)
}

func (s *SMTInternalizerState) PendingTheoryDisequalities() []SMTTheoryPropagation {
	if s == nil {
		return nil
	}
	return append([]SMTTheoryPropagation(nil), s.theoryDisequalities[s.theoryDisequalityHead:]...)
}

func (s *SMTInternalizerState) PropagateTheoryEvents() error {
	if err := s.open(); err != nil {
		return err
	}
	for s.theoryEqualityHead < len(s.theoryEqualities) {
		event := s.theoryEqualities[s.theoryEqualityHead]
		plugin := s.theories[event.Theory]
		if plugin == nil {
			return ast.ErrASTDomain
		}
		if err := plugin.NewEquality(event.Left, event.Right); err != nil {
			return err
		}
		s.theoryEqualityHead++
	}
	for s.theoryDisequalityHead < len(s.theoryDisequalities) {
		event := s.theoryDisequalities[s.theoryDisequalityHead]
		plugin := s.theories[event.Theory]
		if plugin == nil {
			return ast.ErrASTDomain
		}
		left, err := plugin.Enode(event.Left)
		if err != nil {
			return err
		}
		right, err := plugin.Enode(event.Right)
		if err != nil {
			return err
		}
		if left.IsInterpreted() && right.IsInterpreted() {
			s.theoryDisequalityHead++
			continue
		}
		if err := plugin.NewDisequality(event.Left, event.Right); err != nil {
			return err
		}
		s.theoryDisequalityHead++
	}
	return nil
}

func (s *SMTInternalizerState) propagateAssignedAtom(literal SMTLiteral) error {
	variable := literal.Var()
	data, err := s.BooleanData(variable)
	if err != nil {
		return fmt.Errorf("assigned literal %d Boolean metadata: %w", variable, err)
	}
	if !data.IsAtom() || s.atomNotified[variable] {
		return nil
	}
	if node := s.context.enodes[s.boolExpr[variable]]; node != nil {
		if err = s.propagateBooleanEnode(variable, node); err != nil {
			return err
		}
		if s.inconsistent {
			return nil
		}
	}
	if data.IsEquality() {
		expression := s.boolExpr[variable]
		leftExpression, leftErr := s.manager.GetArg(expression, 0)
		rightExpression, rightErr := s.manager.GetArg(expression, 1)
		if leftErr != nil || rightErr != nil {
			return fmt.Errorf("assigned equality %d arguments: %w", variable, ast.ErrASTDomain)
		}
		left, right := s.context.enodes[leftExpression], s.context.enodes[rightExpression]
		if left == nil || right == nil {
			return fmt.Errorf("assigned equality %d enodes: %w", variable, ast.ErrASTDomain)
		}
		if literal.Sign() {
			if err = s.addDisequality(left, right, literal); err != nil {
				return fmt.Errorf("propagate equality disequality: %w", err)
			}
		} else if err = s.addEquality(left, right, NewSMTEqualityEquation(literal)); err != nil {
			return fmt.Errorf("propagate equality merge: %w", err)
		}
		if s.inconsistent {
			return nil
		}
	}
	if data.IsTheoryAtom() {
		if s.relevancyLevel >= 2 && s.relevancy != nil && !s.relevancy.IsRelevant(s.boolExpr[variable]) {
			return nil
		}
		plugin := s.theories[data.Theory()]
		if plugin == nil {
			return fmt.Errorf("assigned theory atom %d missing theory %d: %w", variable, data.Theory(), ast.ErrASTDomain)
		}
		if err = plugin.Assign(variable, !literal.Sign()); err != nil {
			return fmt.Errorf("assigned theory atom %d theory %d: %w", variable, data.Theory(), err)
		}
	}
	if s.atomNotified == nil {
		s.atomNotified = make(map[int32]bool)
	}
	s.atomNotified[variable] = true
	s.atomNotifiedTrail = append(s.atomNotifiedTrail, variable)
	return nil
}

func (s *SMTInternalizerState) PropagateTheoryAssignments() error {
	if err := s.open(); err != nil {
		return err
	}
	for s.atomPropagationHead < len(s.assignedLiterals) {
		if err := s.propagateAssignedAtom(s.assignedLiterals[s.atomPropagationHead]); err != nil {
			return err
		}
		if s.inconsistent {
			return nil
		}
		s.atomPropagationHead++
	}
	if err := s.propagateCoreEqualities(); err != nil {
		return fmt.Errorf("propagate core equalities: %w", err)
	}
	return nil
}

// Source context::relevant_eh is called by set_relevant immediately,
// before assignment propagation and at the scope where the mark was made.
func (s *SMTInternalizerState) notifyRelevant(expression ast.ASTHandle) error {
	if err := s.admitRelevanceCaseSplit(expression); err != nil {
		return err
	}
	kind, err := s.manager.GetKind(expression)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return nil
	}
	if s.quantifierManager != nil {
		s.quantifierManager.NotifyRelevant(s.context.enodes[expression])
	}
	notified := make(map[SMTTheoryID]struct{})
	family, err := s.manager.GetFamilyID(expression)
	if err != nil {
		return err
	}
	if family != s.manager.BasicFamilyID() {
		if plugin := s.theories[family]; plugin != nil {
			if err = plugin.Relevant(expression); err != nil {
				return fmt.Errorf("theory %d relevant expression %d: %w", family, expression.Slot(), err)
			}
			notified[family] = struct{}{}
		}
	}
	if node := s.context.enodes[expression]; node != nil {
		for item := &node.theoryVariables; item != nil && item.theory != SMTNullTheoryID; item = item.next {
			if _, found := notified[item.theory]; found {
				continue
			}
			plugin := s.theories[item.theory]
			if plugin == nil {
				return ast.ErrASTDomain
			}
			if err = plugin.Relevant(expression); err != nil {
				return fmt.Errorf("attached theory %d relevant expression %d: %w", item.theory, expression.Slot(), err)
			}
			notified[item.theory] = struct{}{}
		}
	}
	return nil
}

func (s *SMTInternalizerState) propagateTheoryRelevancy() error {
	if s.relevancy == nil {
		return nil
	}
	if err := s.relevancy.Propagate(); err != nil {
		return err
	}
	for _, expression := range s.relevancy.DrainNewlyRelevant() {
		kind, err := s.manager.GetKind(expression)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			continue
		}
		// Source relevant_eh queues an already assigned atom when it first
		// becomes relevant. Notify once per assignment and restore on pop.
		if variable, found := s.BoolVar(expression); found {
			literal := NewSMTLiteral(variable, false)
			value, err := s.Assignment(literal)
			if err != nil {
				return err
			}
			if value != lbool.Undef {
				if value == lbool.False {
					literal = literal.Negated()
				}
				if err = s.propagateAssignedAtom(literal); err != nil {
					return err
				}
				if s.inconsistent {
					return nil
				}
			}
		}
	}
	return nil
}

// PropagateCore runs the retained logical-context queues to a fixed point.
func (s *SMTInternalizerState) PropagateCore() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	for {
		caseSplitHead := s.propagationHead
		consistent, err := s.Propagate()
		if err != nil || !consistent {
			if err != nil {
				return consistent, fmt.Errorf("Boolean propagation: %w", err)
			}
			return consistent, nil
		}
		consistent, err = s.propagateTheoryCaseSplits(caseSplitHead)
		if err != nil || !consistent {
			return consistent, err
		}
		if err = s.propagateTheoryRelevancy(); err != nil {
			return false, fmt.Errorf("theory relevancy: %w", err)
		}
		if err = s.PropagateTheoryAssignments(); err != nil {
			return false, fmt.Errorf("theory assignments: %w", err)
		}
		if err = s.PropagateTheoryEvents(); err != nil {
			return false, fmt.Errorf("theory events: %w", err)
		}
		dynamicAckProgress, err := s.dynamicAck.propagate()
		if err != nil {
			return false, fmt.Errorf("dynamic Ackermann propagation: %w", err)
		}
		theoryProgress := false
		for _, plugin := range s.theoryOrder {
			if plugin.CanPropagate() {
				theoryProgress = true
				if err = plugin.Propagate(); err != nil {
					return false, fmt.Errorf("theory %d propagation: %w", plugin.FamilyID(), err)
				}
			}
		}
		quantifierProgress := false
		if s.quantifierManager != nil && s.quantifierManager.CanPropagate() {
			quantifierProgress = true
			if err = s.quantifierManager.Propagate(); err != nil {
				return false, fmt.Errorf("quantifier propagation: %w", err)
			}
		}
		if s.inconsistent {
			return false, nil
		}
		relevancyDone := s.relevancy == nil || !s.relevancy.CanPropagate()
		if s.propagationHead == len(s.assignedLiterals) && s.atomPropagationHead == len(s.assignedLiterals) && s.pendingHead == len(s.pending) && s.theoryEqualityHead == len(s.theoryEqualities) && s.theoryDisequalityHead == len(s.theoryDisequalities) && relevancyDone && !theoryProgress && !quantifierProgress && !dynamicAckProgress {
			return true, nil
		}
	}
}
