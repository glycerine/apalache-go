package smtcore

// Source: src/smt/theory_str.cpp z3str2_get_eqc_value, mk_value_helper,
// init_model, mk_value and finalize_model.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

func (t *SMTStringTheory) eqcStringValue(expression ast.ASTHandle) (ast.ASTHandle, bool, error) {
	node := t.state.context.enodes[expression]
	if node == nil || !t.IsAttachedToVariable(node) {
		return expression, false, nil
	}
	first := t.find.Find(uint32(node.TheoryVar(t.FamilyID())))
	for current := first; ; {
		owner := t.variables[current].Owner()
		if _, constant, err := t.state.manager.SequenceStringValue(owner); err != nil || constant {
			return owner, constant, err
		}
		current = t.find.Next(current)
		if current == first {
			return expression, false, nil
		}
	}
}

func (t *SMTStringTheory) modelStringValue(expression ast.ASTHandle) (ast.ASTHandle, bool, error) {
	m := t.state.manager
	if _, constant, err := m.SequenceStringValue(expression); err != nil || constant {
		return expression, constant, err
	}
	left, right, concat, err := m.SequenceConcatParts(expression)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if concat {
		leftValue, leftConstant, err := t.modelStringValue(left)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		rightValue, rightConstant, err := t.modelStringValue(right)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if leftConstant && rightConstant {
			leftText, _, err := m.SequenceStringValue(leftValue)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			rightText, _, err := m.SequenceStringValue(rightValue)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			value, err := t.stringLiteral(leftText + rightText)
			if err == nil {
				// The Go model-value procedure returns a raw handle. Retain
				// uncached results through the procedure's eventual consumer.
				err = t.retain(value)
			}
			return value, err == nil, err
		}
	}
	return t.eqcStringValue(expression)
}

func (t *SMTStringTheory) InitializeModel(generator *SMTModelGenerator) error {
	state, err := t.State()
	if err != nil || generator == nil || generator.model == nil || generator.manager != state.manager {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	factory, err := protomodel.NewStringFactory(generator.model, t.FamilyID())
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

func (t *SMTStringTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	state, err := t.State()
	if err != nil || node == nil || !t.IsAttachedToVariable(node) || generator == nil || generator.manager != state.manager {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	if state.context.enodes[node.Owner()] != node {
		return nil, ast.ErrASTDomain
	}
	value, concrete, err := t.modelStringValue(node.Owner())
	if err != nil {
		return nil, err
	}
	if !concrete {
		text := "**UNUSED**" + strconv.FormatUint(uint64(t.unusedID), 10)
		t.unusedID++
		value, err = t.stringLiteral(text)
		if err != nil {
			return nil, err
		}
		if err = t.retain(value); err != nil {
			return nil, err
		}
	}
	return NewSMTExpressionValueProcedure(value)
}

func (*SMTStringTheory) FinalizeModel(*SMTModelGenerator) error { return nil }

var _ SMTModelValueTheory = (*SMTStringTheory)(nil)
var _ SMTModelFinalizer = (*SMTStringTheory)(nil)
