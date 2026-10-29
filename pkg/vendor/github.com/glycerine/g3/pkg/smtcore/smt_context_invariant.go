package smtcore

// Source: src/smt/smt_context_inv.cpp retained logical-context invariant and
// Boolean clause-justification validation.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// CheckInvariant validates the relationships shared by Boolean variables,
// assignments, watched clauses, enodes, and the congruence table.
func (s *SMTInternalizerState) CheckInvariant() error {
	if err := s.open(); err != nil {
		return err
	}
	variables := len(s.booleanData)
	if len(s.assignment) != 2*variables || len(s.watches) != 2*variables ||
		len(s.activity) != variables || len(s.boolExpr) != variables ||
		len(s.booleanStack) != variables-1 {
		return ast.ErrASTDomain
	}
	if s.assignment[SMTTrueLiteral.Index()] != lbool.True || s.assignment[SMTFalseLiteral.Index()] != lbool.False {
		return ast.ErrASTDomain
	}
	for variable := SMTFirstBoolVar; int(variable) < variables; variable++ {
		expression := s.boolExpr[variable]
		if expression.IsNull() || s.boolVar[expression] != variable {
			return ast.ErrASTDomain
		}
		positive := NewSMTLiteral(variable, false)
		if s.assignment[positive.Negated().Index()] != s.assignment[positive.Index()].Negated() {
			return ast.ErrASTDomain
		}
	}
	if len(s.boolVar) != variables-1 {
		return ast.ErrASTDomain
	}
	for index := range s.watches {
		watched := SMTLiteralFromIndex(int32(index)).Negated()
		for _, clause := range s.watches[index].clauses {
			if clause == nil || clause.Deleted() || !clause.IsWatch(watched) {
				return ast.ErrASTDomain
			}
		}
		for _, literal := range s.watches[index].literals {
			if literal.Index() < 0 || int(literal.Index()) >= len(s.assignment) {
				return ast.ErrASTDomain
			}
			reverse := &s.watches[literal.Negated().Index()]
			if reverse.FindLiteral(SMTLiteralFromIndex(int32(index)).Negated()) < 0 {
				return ast.ErrASTDomain
			}
		}
	}
	for index := range s.clauses {
		record := &s.clauses[index]
		if record.Binary {
			if record.clause != nil || len(record.Literals) != 2 {
				return ast.ErrASTDomain
			}
			first, second := record.Literals[0], record.Literals[1]
			if s.watches[first.Negated().Index()].FindLiteral(second) < 0 ||
				s.watches[second.Negated().Index()].FindLiteral(first) < 0 {
				return ast.ErrASTDomain
			}
			continue
		}
		if record.clause == nil || record.clause.Deleted() || len(record.Literals) < 2 {
			return ast.ErrASTDomain
		}
		literals := record.clause.Literals()
		if len(literals) != len(record.Literals) {
			return ast.ErrASTDomain
		}
		for literalIndex := range literals {
			if literals[literalIndex] != record.Literals[literalIndex] && literalIndex >= 2 {
				return ast.ErrASTDomain
			}
			if literals[literalIndex].Index() < 0 || int(literals[literalIndex].Index()) >= len(s.assignment) {
				return ast.ErrASTDomain
			}
		}
		for watchIndex := 0; watchIndex < 2; watchIndex++ {
			literal := literals[watchIndex]
			if s.watches[literal.Negated().Index()].FindClause(record.clause) < 0 {
				return ast.ErrASTDomain
			}
		}
	}
	for _, node := range s.enodeStack {
		if node == nil || node.manager != s.manager {
			return ast.ErrASTDomain
		}
		if err := node.CheckInvariant(); err != nil {
			return err
		}
		if node.NumArgs() != 0 && node.CongruenceEnabled() {
			contained, err := s.congruence.ContainsPointer(node)
			if err != nil {
				return err
			}
			if node.IsTrueEquality() {
				if contained {
					return ast.ErrASTDomain
				}
			} else if node.IsCongruenceRoot() != contained {
				return ast.ErrASTDomain
			}
		}
	}
	if err := s.congruence.CheckInvariant(); err != nil {
		return err
	}
	if s.propagationHead < 0 || s.propagationHead > len(s.assignedLiterals) ||
		s.pendingHead < 0 || s.pendingHead > len(s.pending) ||
		s.atomPropagationHead < 0 || s.atomPropagationHead > len(s.assignedLiterals) ||
		s.theoryEqualityHead < 0 || s.theoryEqualityHead > len(s.theoryEqualities) ||
		s.theoryDisequalityHead < 0 || s.theoryDisequalityHead > len(s.theoryDisequalities) {
		return ast.ErrASTDomain
	}
	for _, literal := range s.assignedLiterals {
		if literal.Index() < 0 || int(literal.Index()) >= len(s.assignment) || s.assignment[literal.Index()] != lbool.True {
			return ast.ErrASTDomain
		}
	}
	return s.checkEqualityClassBooleanAssignments()
}

func (s *SMTInternalizerState) checkEqualityClassBooleanAssignments() error {
	for _, node := range s.enodeStack {
		if !node.IsBool() {
			continue
		}
		if s.context.assignments[node.Owner()] != s.context.assignments[node.Root().Owner()] {
			return ast.ErrASTDomain
		}
	}
	return nil
}

// ValidateJustification checks the source clause invariant for an assigned
// Boolean variable: one of the two watched literals is the assigned true
// literal for that variable.
func (s *SMTInternalizerState) ValidateJustification(variable int32) error {
	if err := s.open(); err != nil {
		return err
	}
	if variable < SMTFirstBoolVar || int(variable) >= len(s.booleanData) {
		return ast.ErrASTDomain
	}
	justification := s.booleanData[variable].Justification()
	clause, clauseKind := justification.Clause()
	if !clauseKind {
		return nil
	}
	if clause == nil {
		if s.assignment[NewSMTLiteral(variable, false).Index()] == lbool.Undef {
			return nil
		}
		return ast.ErrASTDomain
	}
	for index := 0; index < 2; index++ {
		literal, err := clause.Literal(index)
		if err != nil {
			return err
		}
		if literal.Var() == variable && s.assignment[literal.Index()] == lbool.True {
			return nil
		}
	}
	return ast.ErrASTDomain
}
