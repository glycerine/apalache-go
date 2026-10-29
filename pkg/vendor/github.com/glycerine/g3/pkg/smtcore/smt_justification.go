package smtcore

// Source: context-independent dependency records in smt_justification.h/.cpp.

import "github.com/glycerine/g3/pkg/ast"

type SMTEnodePair struct{ First, Second *SMTEnode }
type SMTAntecedentMarker interface {
	MarkJustification(SMTJustification) error
	MarkLiteral(SMTLiteral) error
	MarkEquality(*SMTEnode, *SMTEnode) error
	MarkJustifiedEquality(*SMTEnode, *SMTEnode, SMTEqualityJustification) error
	LiteralForENode(*SMTEnode) (SMTLiteral, error)
}
type SMTJustificationBase struct{ heap, marked bool }

func NewSMTJustificationBase(region bool) SMTJustificationBase {
	return SMTJustificationBase{heap: !region}
}
func (*SMTJustificationBase) smtJustification() {}
func (j *SMTJustificationBase) InRegion() bool  { return j != nil && !j.heap }
func (j *SMTJustificationBase) IsMarked() bool  { return j != nil && j.marked }
func (j *SMTJustificationBase) SetMarked(v bool) error {
	if j == nil || j.marked == v {
		return ast.ErrASTDomain
	}
	j.marked = v
	return nil
}
func (*SMTJustificationBase) MarkAntecedents(SMTAntecedentMarker) error { return nil }
func (*SMTJustificationBase) FromTheory() SMTTheoryID                   { return SMTNullTheoryID }
func (*SMTJustificationBase) Name() string                              { return "unknown" }
func (*SMTJustificationBase) Release(*ast.ASTManager) error             { return nil }

type SMTUnitResolutionJustification struct {
	SMTJustificationBase
	Antecedent SMTJustification
	Literals   []SMTLiteral
}

func NewSMTUnitResolutionJustification(region bool, antecedent SMTJustification, literals []SMTLiteral) (*SMTUnitResolutionJustification, error) {
	if len(literals) == 0 || antecedent != nil && antecedent.InRegion() != region {
		return nil, ast.ErrASTDomain
	}
	return &SMTUnitResolutionJustification{SMTJustificationBase: NewSMTJustificationBase(region), Antecedent: antecedent, Literals: append([]SMTLiteral(nil), literals...)}, nil
}
func (j *SMTUnitResolutionJustification) MarkAntecedents(m SMTAntecedentMarker) error {
	if m == nil {
		return ast.ErrASTHandle
	}
	if j.Antecedent != nil {
		if err := m.MarkJustification(j.Antecedent); err != nil {
			return err
		}
	}
	for _, l := range j.Literals {
		if err := m.MarkLiteral(l); err != nil {
			return err
		}
	}
	return nil
}
func (*SMTUnitResolutionJustification) Name() string { return "unit-resolution" }
func (j *SMTUnitResolutionJustification) Release(m *ast.ASTManager) error {
	if !j.InRegion() && j.Antecedent != nil {
		if err := j.Antecedent.Release(m); err != nil {
			return err
		}
		j.Antecedent = nil
	}
	j.Literals = nil
	return nil
}

type SMTSimpleJustification struct {
	SMTJustificationBase
	Literals []SMTLiteral
}

func NewSMTSimpleJustification(region bool, literals []SMTLiteral) (*SMTSimpleJustification, error) {
	for _, literal := range literals {
		if literal == SMTNullLiteral {
			return nil, ast.ErrASTDomain
		}
	}
	return &SMTSimpleJustification{NewSMTJustificationBase(region), append([]SMTLiteral(nil), literals...)}, nil
}
func (j *SMTSimpleJustification) MarkAntecedents(m SMTAntecedentMarker) error {
	if m == nil {
		return ast.ErrASTHandle
	}
	for _, l := range j.Literals {
		if err := m.MarkLiteral(l); err != nil {
			return err
		}
	}
	return nil
}
func (*SMTSimpleJustification) Name() string { return "simple" }

type SMTExtSimpleJustification struct {
	SMTSimpleJustification
	Equalities []SMTEnodePair
}

func NewSMTExtSimpleJustification(region bool, literals []SMTLiteral, equalities []SMTEnodePair) (*SMTExtSimpleJustification, error) {
	for _, p := range equalities {
		if p.First == nil || p.Second == nil || p.First.root != p.Second.root {
			return nil, ast.ErrASTDomain
		}
	}
	simple, err := NewSMTSimpleJustification(region, literals)
	if err != nil {
		return nil, err
	}
	return &SMTExtSimpleJustification{*simple, append([]SMTEnodePair(nil), equalities...)}, nil
}
func (j *SMTExtSimpleJustification) MarkAntecedents(m SMTAntecedentMarker) error {
	if err := j.SMTSimpleJustification.MarkAntecedents(m); err != nil {
		return err
	}
	for _, p := range j.Equalities {
		if err := m.MarkEquality(p.First, p.Second); err != nil {
			return err
		}
	}
	return nil
}
func (*SMTExtSimpleJustification) Name() string { return "ext-simple" }

type SMTEqualityConflictJustification struct {
	SMTJustificationBase
	First, Second *SMTEnode
	Equality      SMTEqualityJustification
}

func NewSMTEqualityConflictJustification(first, second *SMTEnode, equality SMTEqualityJustification) (*SMTEqualityConflictJustification, error) {
	if first == nil || second == nil || first == second {
		return nil, ast.ErrASTDomain
	}
	return &SMTEqualityConflictJustification{SMTJustificationBase: NewSMTJustificationBase(true), First: first, Second: second, Equality: equality}, nil
}
func (j *SMTEqualityConflictJustification) MarkAntecedents(m SMTAntecedentMarker) error {
	if m == nil || j.First.root == nil || j.Second.root == nil || !j.First.root.interpreted || !j.Second.root.interpreted {
		return ast.ErrASTDomain
	}
	if err := m.MarkEquality(j.First, j.First.root); err != nil {
		return err
	}
	if err := m.MarkEquality(j.Second, j.Second.root); err != nil {
		return err
	}
	return m.MarkJustifiedEquality(j.First, j.Second, j.Equality)
}
func (*SMTEqualityConflictJustification) Name() string { return "eq-conflict" }

// SMTDisequalityConflictJustification explains why a pending equality cannot
// merge two roots that are already constrained to be distinct.
type SMTDisequalityConflictJustification struct {
	SMTJustificationBase
	First, Second *SMTEnode
	Equality      SMTEqualityJustification
}

func NewSMTDisequalityConflictJustification(first, second *SMTEnode, equality SMTEqualityJustification) (*SMTDisequalityConflictJustification, error) {
	if first == nil || second == nil || first == second {
		return nil, ast.ErrASTDomain
	}
	return &SMTDisequalityConflictJustification{SMTJustificationBase: NewSMTJustificationBase(true), First: first, Second: second, Equality: equality}, nil
}

func (j *SMTDisequalityConflictJustification) MarkAntecedents(marker SMTAntecedentMarker) error {
	if marker == nil {
		return ast.ErrASTHandle
	}
	return marker.MarkJustifiedEquality(j.First, j.Second, j.Equality)
}

func (*SMTDisequalityConflictJustification) Name() string { return "diseq-conflict" }

type SMTEqualityRootJustification struct {
	SMTJustificationBase
	Node *SMTEnode
}

func NewSMTEqualityRootJustification(node *SMTEnode) (*SMTEqualityRootJustification, error) {
	if node == nil || node.root == nil {
		return nil, ast.ErrASTDomain
	}
	return &SMTEqualityRootJustification{NewSMTJustificationBase(true), node}, nil
}
func (j *SMTEqualityRootJustification) MarkAntecedents(m SMTAntecedentMarker) error {
	if m == nil {
		return ast.ErrASTHandle
	}
	return m.MarkEquality(j.Node, j.Node.root)
}
func (*SMTEqualityRootJustification) Name() string { return "eq-root" }

type SMTEqualityPropagationJustification struct {
	SMTJustificationBase
	First, Second *SMTEnode
}

func NewSMTEqualityPropagationJustification(first, second *SMTEnode) (*SMTEqualityPropagationJustification, error) {
	if first == nil || second == nil || first == second {
		return nil, ast.ErrASTDomain
	}
	return &SMTEqualityPropagationJustification{NewSMTJustificationBase(true), first, second}, nil
}
func (j *SMTEqualityPropagationJustification) MarkAntecedents(m SMTAntecedentMarker) error {
	if m == nil {
		return ast.ErrASTHandle
	}
	return m.MarkEquality(j.First, j.Second)
}
func (*SMTEqualityPropagationJustification) Name() string { return "eq-propagation" }

type SMTModusPonensIffJustification struct {
	SMTJustificationBase
	First, Second *SMTEnode
}

func NewSMTModusPonensIffJustification(first, second *SMTEnode) (*SMTModusPonensIffJustification, error) {
	if first == nil || second == nil || first == second {
		return nil, ast.ErrASTDomain
	}
	return &SMTModusPonensIffJustification{NewSMTJustificationBase(true), first, second}, nil
}
func (j *SMTModusPonensIffJustification) MarkAntecedents(m SMTAntecedentMarker) error {
	if m == nil {
		return ast.ErrASTHandle
	}
	if err := m.MarkEquality(j.First, j.Second); err != nil {
		return err
	}
	literal, err := m.LiteralForENode(j.First)
	if err != nil {
		return err
	}
	return m.MarkLiteral(literal)
}
func (*SMTModusPonensIffJustification) Name() string { return "mp-iff" }
