package smtcore

// Source: src/smt/smt_b_justification.h, src/smt/smt_eq_justification.h,
// and src/smt/smt_bool_var_data.h compact dependency records.

import "github.com/glycerine/g3/pkg/ast"

// SMTJustification is the typed base implemented by retained concrete SMT
// justification objects. It mirrors the source abstract justification class.
type SMTJustification interface {
	smtJustification()
	InRegion() bool
	IsMarked() bool
	SetMarked(bool) error
	MarkAntecedents(SMTAntecedentMarker) error
	FromTheory() SMTTheoryID
	Name() string
	Release(*ast.ASTManager) error
}

type SMTBooleanJustificationKind uint8

const (
	SMTBooleanJustificationClause SMTBooleanJustificationKind = iota
	SMTBooleanJustificationBinaryClause
	SMTBooleanJustificationAxiom
	SMTBooleanJustificationObject
)

func (k SMTBooleanJustificationKind) String() string {
	switch k {
	case SMTBooleanJustificationClause:
		return "clause"
	case SMTBooleanJustificationBinaryClause:
		return "bin_clause"
	case SMTBooleanJustificationAxiom:
		return "axiom"
	case SMTBooleanJustificationObject:
		return "theory"
	default:
		return ""
	}
}

// SMTBooleanJustification is a typed Go tagged union replacing the source's
// low-pointer-bit encoding.
type SMTBooleanJustification struct {
	kind          SMTBooleanJustificationKind
	clause        *SMTClause
	literal       SMTLiteral
	justification SMTJustification
}

func NewSMTBooleanAxiom() SMTBooleanJustification {
	return SMTBooleanJustification{kind: SMTBooleanJustificationAxiom}
}
func NewSMTBooleanClause(value *SMTClause) SMTBooleanJustification {
	return SMTBooleanJustification{kind: SMTBooleanJustificationClause, clause: value}
}
func NewSMTBooleanBinaryClause(value SMTLiteral) SMTBooleanJustification {
	return SMTBooleanJustification{kind: SMTBooleanJustificationBinaryClause, literal: value}
}
func NewSMTBooleanJustification(value SMTJustification) (SMTBooleanJustification, error) {
	if value == nil {
		return SMTBooleanJustification{}, ast.ErrASTHandle
	}
	return SMTBooleanJustification{kind: SMTBooleanJustificationObject, justification: value}, nil
}
func NullSMTBooleanJustification() SMTBooleanJustification {
	return NewSMTBooleanClause(nil)
}
func (j SMTBooleanJustification) Kind() SMTBooleanJustificationKind { return j.kind }
func (j SMTBooleanJustification) Clause() (*SMTClause, bool) {
	return j.clause, j.kind == SMTBooleanJustificationClause
}
func (j SMTBooleanJustification) Literal() (SMTLiteral, bool) {
	return j.literal, j.kind == SMTBooleanJustificationBinaryClause
}
func (j SMTBooleanJustification) Justification() (SMTJustification, bool) {
	return j.justification, j.kind == SMTBooleanJustificationObject
}

type SMTEqualityJustificationKind uint8

const (
	SMTEqualityJustificationAxiom SMTEqualityJustificationKind = iota
	SMTEqualityJustificationCongruence
	SMTEqualityJustificationEquation
	SMTEqualityJustificationObject
)

// SMTEqualityJustification replaces the source tagged pointer with a concrete
// discriminant and typed payload fields.
type SMTEqualityJustification struct {
	kind          SMTEqualityJustificationKind
	literal       SMTLiteral
	commutativity bool
	justification SMTJustification
}

func NewSMTEqualityAxiom() SMTEqualityJustification { return SMTEqualityJustification{} }
func NewSMTEqualityCongruence(commutativity bool) SMTEqualityJustification {
	return SMTEqualityJustification{kind: SMTEqualityJustificationCongruence, commutativity: commutativity}
}
func NewSMTEqualityEquation(literal SMTLiteral) SMTEqualityJustification {
	return SMTEqualityJustification{kind: SMTEqualityJustificationEquation, literal: literal}
}
func NewSMTEqualityJustification(value SMTJustification) SMTEqualityJustification {
	return SMTEqualityJustification{kind: SMTEqualityJustificationObject, justification: value}
}
func NullSMTEqualityJustification() SMTEqualityJustification {
	return NewSMTEqualityJustification(nil)
}
func (j SMTEqualityJustification) Kind() SMTEqualityJustificationKind { return j.kind }
func (j SMTEqualityJustification) Literal() (SMTLiteral, bool) {
	return j.literal, j.kind == SMTEqualityJustificationEquation
}
func (j SMTEqualityJustification) UsedCommutativity() (bool, bool) {
	return j.commutativity, j.kind == SMTEqualityJustificationCongruence
}
func (j SMTEqualityJustification) Justification() (SMTJustification, bool) {
	return j.justification, j.kind == SMTEqualityJustificationObject
}

// SMTBoolVarData is the per-variable flag record. It contains no owning
// payload; all justification targets are borrowed from the context.
type SMTBoolVarData struct {
	justification  SMTBooleanJustification
	ScopeLevel     uint32
	Mark           bool
	Assumption     bool
	PhaseAvailable bool
	Phase          bool
	equality       bool
	trueFirst      bool
	enode          bool
	quantifier     bool
	internalLevel  uint32
	atom           bool
	notifyTheory   uint8
}

func (d *SMTBoolVarData) updateAtom() {
	d.atom = d.equality || d.notifyTheory != 0 || d.quantifier || d.enode
}
func (d *SMTBoolVarData) Initialize(internalLevel uint32) {
	if d == nil {
		return
	}
	*d = SMTBoolVarData{justification: NullSMTBooleanJustification(), internalLevel: internalLevel}
}
func (d *SMTBoolVarData) InternalLevel() uint32 { return d.internalLevel }
func (d *SMTBoolVarData) Justification() SMTBooleanJustification {
	return d.justification
}
func (d *SMTBoolVarData) SetAxiom() { d.justification = NewSMTBooleanAxiom() }
func (d *SMTBoolVarData) SetNullJustification() {
	d.justification = NullSMTBooleanJustification()
}
func (d *SMTBoolVarData) SetJustification(value SMTBooleanJustification) {
	d.justification = value
}
func (d *SMTBoolVarData) IsAtom() bool { return d.atom }
func (d *SMTBoolVarData) Theory() SMTTheoryID {
	if d.notifyTheory == 0 {
		return SMTNullTheoryID
	}
	return SMTTheoryID(d.notifyTheory)
}
func (d *SMTBoolVarData) IsTheoryAtom() bool { return d.notifyTheory != 0 }
func (d *SMTBoolVarData) SetNotifyTheory(value SMTTheoryID) error {
	if value <= 0 || value > 255 {
		return ast.ErrASTDomain
	}
	d.notifyTheory, d.atom = uint8(value), true
	return nil
}
func (d *SMTBoolVarData) ResetNotifyTheory() { d.notifyTheory = 0; d.updateAtom() }
func (d *SMTBoolVarData) IsENode() bool      { return d.enode }
func (d *SMTBoolVarData) SetENode()          { d.enode, d.atom = true, true }
func (d *SMTBoolVarData) ResetENode()        { d.enode = false; d.updateAtom() }
func (d *SMTBoolVarData) IsQuantifier() bool { return d.quantifier }
func (d *SMTBoolVarData) SetQuantifier()     { d.quantifier, d.atom = true, true }
func (d *SMTBoolVarData) IsEquality() bool   { return d.equality }
func (d *SMTBoolVarData) SetEquality()       { d.equality, d.atom = true, true }
func (d *SMTBoolVarData) ResetEquality()     { d.equality = false; d.updateAtom() }
func (d *SMTBoolVarData) TryTrueFirst() bool { return d.trueFirst }
func (d *SMTBoolVarData) SetTrueFirst()      { d.trueFirst = true }
func (d *SMTBoolVarData) ResetTrueFirst()    { d.trueFirst = false }
