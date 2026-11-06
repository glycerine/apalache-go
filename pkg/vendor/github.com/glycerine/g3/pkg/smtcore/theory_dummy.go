package smtcore

// Source: src/smt/theory_dummy.cpp and src/smt/theory_dummy.h.

import "github.com/glycerine/g3/pkg/ast"

// SMTDummyTheory records whether an unsupported theory expression was reached.
// It deliberately declines internalization and makes final check give up.
type SMTDummyTheory struct {
	*SMTTheoryBase
	name        string
	theoryExprs bool
	scopeValues []bool
}

func NewSMTDummyTheory(family ast.FamilyID, name string) (*SMTDummyTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	return &SMTDummyTheory{SMTTheoryBase: base, name: name}, nil
}

func (t *SMTDummyTheory) foundTheoryExpression() error {
	if t == nil || t.SMTTheoryBase == nil {
		return ast.ErrASTHandle
	}
	if _, err := t.State(); err != nil {
		return err
	}
	t.theoryExprs = true
	return nil
}

func (t *SMTDummyTheory) InternalizeAtom(ast.ASTHandle, bool) (bool, error) {
	return false, t.foundTheoryExpression()
}

func (t *SMTDummyTheory) InternalizeTerm(ast.ASTHandle) (bool, error) {
	return false, t.foundTheoryExpression()
}

func (*SMTDummyTheory) InternalizeEquality(ast.ASTHandle, int32) error     { return nil }
func (*SMTDummyTheory) ApplySortConstraint(*SMTEnode, ast.ASTHandle) error { return nil }
func (*SMTDummyTheory) UseDisequalities() bool                             { return false }
func (*SMTDummyTheory) NewEquality(SMTTheoryVar, SMTTheoryVar) error {
	return ast.ErrASTDomain
}
func (*SMTDummyTheory) NewDisequality(SMTTheoryVar, SMTTheoryVar) error {
	return ast.ErrASTDomain
}
func (*SMTDummyTheory) Assign(int32, bool) error { return nil }

func (t *SMTDummyTheory) PushScope() error {
	if t == nil || t.SMTTheoryBase == nil {
		return ast.ErrASTHandle
	}
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.scopeValues = append(t.scopeValues, t.theoryExprs)
	return nil
}

func (t *SMTDummyTheory) PopScopes(scopes uint32) error {
	if t == nil || t.SMTTheoryBase == nil || scopes == 0 || scopes > uint32(len(t.scopeValues)) {
		return ast.ErrASTDomain
	}
	level := len(t.scopeValues) - int(scopes)
	value := t.scopeValues[level]
	if err := t.SMTTheoryBase.PopScopes(scopes); err != nil {
		return err
	}
	t.theoryExprs = value
	t.scopeValues = t.scopeValues[:level]
	return nil
}

func (t *SMTDummyTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if t == nil || t.SMTTheoryBase == nil {
		return SMTFinalCheckGiveUp, ast.ErrASTHandle
	}
	if t.theoryExprs {
		return SMTFinalCheckGiveUp, nil
	}
	return SMTFinalCheckDone, nil
}

// Reset preserves the source reset_eh behavior: after reset the dummy theory
// reports incompleteness until it is replaced by a real theory.
func (t *SMTDummyTheory) Reset() error {
	if t == nil || t.SMTTheoryBase == nil {
		return ast.ErrASTHandle
	}
	t.theoryExprs = true
	t.scopeValues = nil
	t.variables = nil
	t.limits = nil
	return nil
}

func (t *SMTDummyTheory) BuildModels() bool { return false }
func (t *SMTDummyTheory) Name() string {
	if t == nil {
		return ""
	}
	return t.name
}

func (t *SMTDummyTheory) Fresh() (*SMTDummyTheory, error) {
	if t == nil || t.SMTTheoryBase == nil {
		return nil, ast.ErrASTHandle
	}
	return NewSMTDummyTheory(t.family, t.name)
}
