package smtcore

// Source: theory_datatype::propagate_recognizer.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTDatatypeTheory) propagateRecognizer(variable SMTTheoryVar, trigger *SMTEnode) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	root, data, err := t.rootData(variable)
	if err != nil {
		return err
	}
	if len(data.recognizers) == 0 {
		if err = t.traceDatatypeEmptyRecognizers(); err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	node := t.variables[root]
	literals := make([]SMTLiteral, 0)
	equalities := make([]SMTEnodePair, 0)
	unassigned, index := 0, 0
	for i, recognizer := range data.recognizers {
		if recognizer != nil {
			literal, err := s.Literal(recognizer.owner)
			if err != nil {
				return err
			}
			value, err := s.Assignment(literal)
			if err != nil {
				return err
			}
			if value == lbool.True {
				return nil
			}
			if value == lbool.False {
				literals = append(literals, literal.Negated())
				if node != recognizer.arguments[0] {
					equalities = append(equalities, SMTEnodePair{node, recognizer.arguments[0]})
				}
				continue
			}
		}
		if unassigned == 0 {
			index = i
		}
		unassigned++
	}
	if unassigned == 0 {
		if err = t.traceDatatypeRecognizerExhaustion(trigger, literals, equalities); err != nil {
			return err
		}
		return t.setDatatypeConflict(literals, equalities)
	}
	sort, err := s.manager.GetSort(node.owner)
	if err != nil {
		return err
	}
	if unassigned > 1 {
		infinite, err := s.manager.SortIsInfinite(sort)
		if err != nil {
			return err
		}
		if t.lazySplits() == 0 || t.lazySplits() == 1 && !infinite {
			return t.split(node.owner, sort)
		}
		return nil
	}
	var literal SMTLiteral
	var expression ast.ASTHandle
	if recognizer := data.recognizers[index]; recognizer != nil {
		expression = recognizer.owner
		literal, err = s.Literal(expression)
	} else {
		var constructors []ast.ASTHandle
		constructors, err = s.manager.DatatypeConstructors(sort)
		if err == nil {
			literal, expression, err = t.recognizerLiteral(constructors[index], node.owner)
		}
	}
	if err != nil {
		return err
	}
	if err = s.markRelevant(expression); err != nil {
		return err
	}
	owner, err := NewSMTExtTheoryPropagationJustification(s.manager, t.FamilyID(), literals, equalities, literal, nil)
	if err != nil {
		return err
	}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return errors.Join(err, owner.Release(s.manager))
	}
	s.ownedJustifications = append(s.ownedJustifications, owner)
	return s.AssignLiteral(literal, wrapped)
}
