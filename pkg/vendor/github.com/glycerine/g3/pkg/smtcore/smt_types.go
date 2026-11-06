package smtcore

// Source: src/smt/smt_types.h, src/smt/smt_failure.h, and
// src/smt/smt_theory_var_list.h.

import "github.com/glycerine/g3/pkg/ast"

type SMTBoolVar = int32
type SMTTheoryID = ast.FamilyID
type SMTTheoryVar = int32

const (
	SMTNullTheoryID  SMTTheoryID  = ast.NullFamilyID
	SMTNullTheoryVar SMTTheoryVar = -1
)

type SMTFinalCheckStatus uint8

const (
	SMTFinalCheckDone SMTFinalCheckStatus = iota
	SMTFinalCheckContinue
	SMTFinalCheckGiveUp
)

func (s SMTFinalCheckStatus) String() string {
	switch s {
	case SMTFinalCheckDone:
		return "done"
	case SMTFinalCheckContinue:
		return "continue"
	case SMTFinalCheckGiveUp:
		return "giveup"
	default:
		return ""
	}
}

// SMTFailure identifies the source reason for an undefined check result.
type SMTFailure uint8

const (
	SMTFailureOK SMTFailure = iota
	SMTFailureUnknown
	SMTFailureTimeout
	SMTFailureMemory
	SMTFailureCanceled
	SMTFailureConflictLimit
	SMTFailureTheory
	SMTFailureResourceLimit
	SMTFailureQuantifiers
)

func (f SMTFailure) String() string {
	switch f {
	case SMTFailureOK:
		return "OK"
	case SMTFailureUnknown:
		return "UNKNOWN"
	case SMTFailureTimeout:
		return "TIMEOUT"
	case SMTFailureMemory:
		return "MEMOUT"
	case SMTFailureCanceled:
		return "CANCELED"
	case SMTFailureConflictLimit:
		return "NUM_CONFLICTS"
	case SMTFailureTheory:
		return "THEORY"
	case SMTFailureResourceLimit:
		return "RESOURCE_LIMIT"
	case SMTFailureQuantifiers:
		return "QUANTIFIERS"
	default:
		return "?"
	}
}

// SMTTheoryVarList is the source intrusive list node. Next is borrowed; list
// storage is owned by the enclosing SMT context.
type SMTTheoryVarList struct {
	theory   SMTTheoryID
	variable SMTTheoryVar
	next     *SMTTheoryVarList
}

func NewSMTTheoryVarList(theory SMTTheoryID, variable SMTTheoryVar, next *SMTTheoryVarList) *SMTTheoryVarList {
	return &SMTTheoryVarList{theory: theory, variable: variable, next: next}
}

func NewDefaultSMTTheoryVarList() *SMTTheoryVarList {
	return NewSMTTheoryVarList(SMTNullTheoryID, SMTNullTheoryVar, nil)
}

func (n *SMTTheoryVarList) TheoryID() SMTTheoryID {
	if n == nil {
		return SMTNullTheoryID
	}
	return n.theory
}

func (n *SMTTheoryVarList) TheoryVar() SMTTheoryVar {
	if n == nil {
		return SMTNullTheoryVar
	}
	return n.variable
}

func (n *SMTTheoryVarList) Next() *SMTTheoryVarList {
	if n == nil {
		return nil
	}
	return n.next
}

func (n *SMTTheoryVarList) SetTheoryID(value SMTTheoryID) {
	if n != nil {
		n.theory = value
	}
}

func (n *SMTTheoryVarList) SetTheoryVar(value SMTTheoryVar) {
	if n != nil {
		n.variable = value
	}
}

func (n *SMTTheoryVarList) SetNext(value *SMTTheoryVarList) {
	if n != nil {
		n.next = value
	}
}
