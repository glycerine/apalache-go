package smtcore

// Source: theory_datatype::sign_recognizer_conflict and merge_eh.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTDatatypeTheory) setDatatypeConflict(literals []SMTLiteral, equalities []SMTEnodePair) error {
	s, err := t.State()
	if err != nil || s.Inconsistent() {
		return err
	}
	owner, err := NewSMTExtTheoryConflictJustification(s.manager, t.FamilyID(), literals, equalities, nil)
	if err != nil {
		return err
	}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return errors.Join(err, owner.Release(s.manager))
	}
	s.inconsistent = true
	s.conflict = wrapped
	s.conflictLiteral = SMTNullLiteral
	s.statistics.NumConflicts++
	return nil
}

// Source oc_get_cstor looks up the datatype root through the egraph root.
func (t *SMTDatatypeTheory) classConstructor(node *SMTEnode) (*SMTEnode, error) {
	if node == nil || node.root == nil {
		return nil, ast.ErrASTDomain
	}
	if _, err := t.State(); err != nil {
		return nil, err
	}
	_, data, err := t.rootData(node.root.TheoryVar(t.FamilyID()))
	if err != nil {
		return nil, err
	}
	return data.constructor, nil
}

func (t *SMTDatatypeTheory) signRecognizerConflict(constructor, recognizer *SMTEnode) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	if constructor == nil || recognizer == nil || len(recognizer.arguments) != 1 {
		return ast.ErrASTDomain
	}
	if err = t.emitDatatype("recognizer_conflict", "sign_recognizer_conflict", 177, func(out io.Writer) error {
		if err := t.displayDatatypeExpression(out, constructor.owner); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		if err := t.displayDatatypeExpression(out, recognizer.owner); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	literal, err := s.Literal(recognizer.owner)
	if err != nil {
		return err
	}
	return t.setDatatypeConflict([]SMTLiteral{literal.Negated()}, []SMTEnodePair{{constructor, recognizer.arguments[0]}})
}
