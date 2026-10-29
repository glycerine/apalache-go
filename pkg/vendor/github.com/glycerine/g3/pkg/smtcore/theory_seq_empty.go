package smtcore

// Source: src/smt/theory_seq_empty.h theory_seq_empty.

import (
	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

// SMTEmptySequenceTheory supplies sequence model values and reports
// incompleteness if a sequence expression reaches this placeholder theory.
type SMTEmptySequenceTheory struct {
	*SMTTheoryBase
	used   bool
	scopes []bool
}

func NewSMTEmptySequenceTheory(family ast.FamilyID) (*SMTEmptySequenceTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	return &SMTEmptySequenceTheory{SMTTheoryBase: base}, nil
}

func (t *SMTEmptySequenceTheory) markUsed() error {
	if _, err := t.State(); err != nil {
		return err
	}
	t.used = true
	return nil
}

func (t *SMTEmptySequenceTheory) InternalizeAtom(ast.ASTHandle, bool) (bool, error) {
	return false, t.markUsed()
}
func (t *SMTEmptySequenceTheory) InternalizeTerm(ast.ASTHandle) (bool, error) {
	return false, t.markUsed()
}
func (*SMTEmptySequenceTheory) InternalizeEquality(ast.ASTHandle, int32) error { return nil }
func (*SMTEmptySequenceTheory) ApplySortConstraint(*SMTEnode, ast.ASTHandle) error {
	return nil
}
func (*SMTEmptySequenceTheory) UseDisequalities() bool                       { return false }
func (*SMTEmptySequenceTheory) NewEquality(SMTTheoryVar, SMTTheoryVar) error { return nil }
func (*SMTEmptySequenceTheory) NewDisequality(SMTTheoryVar, SMTTheoryVar) error {
	return nil
}
func (*SMTEmptySequenceTheory) Assign(int32, bool) error { return nil }

func (t *SMTEmptySequenceTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.scopes = append(t.scopes, t.used)
	return nil
}

func (t *SMTEmptySequenceTheory) PopScopes(count uint32) error {
	if count == 0 || count > uint32(len(t.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(t.scopes) - int(count)
	used := t.scopes[level]
	if err := t.SMTTheoryBase.PopScopes(count); err != nil {
		return err
	}
	t.used = used
	t.scopes = t.scopes[:level]
	return nil
}

func (t *SMTEmptySequenceTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if _, err := t.State(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if t.used {
		return SMTFinalCheckGiveUp, nil
	}
	return SMTFinalCheckDone, nil
}

func (*SMTEmptySequenceTheory) Name() string { return "seq-empty" }

func (t *SMTEmptySequenceTheory) InitializeModel(generator *SMTModelGenerator) error {
	state, err := t.State()
	if err != nil || generator == nil || generator.model == nil || generator.manager != state.manager {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	factory, err := protomodel.NewSequenceFactory(generator.model, t.FamilyID())
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

// The placeholder never attaches theory variables, so model generation only
// consumes its value factory.
func (*SMTEmptySequenceTheory) ModelValue(*SMTEnode, *SMTModelGenerator) (SMTModelValueProcedure, error) {
	return nil, ast.ErrASTDomain
}

func (t *SMTEmptySequenceTheory) Fresh() (*SMTEmptySequenceTheory, error) {
	if t == nil || t.SMTTheoryBase == nil {
		return nil, ast.ErrASTHandle
	}
	return NewSMTEmptySequenceTheory(t.FamilyID())
}

var _ SMTTheoryInternalizer = (*SMTEmptySequenceTheory)(nil)
var _ SMTModelValueTheory = (*SMTEmptySequenceTheory)(nil)
