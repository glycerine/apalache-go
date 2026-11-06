package smtcore

// Source: src/smt/smt_context_inv.cpp context::validate_model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

func (c *SMTModelChecker) validateGroundAssignments(md *protomodel.ProtoModel) (valid bool, result error) {
	out, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, c.manager.DelRef(&out)) }()
	for _, literal := range c.state.assignedLiterals {
		expression, err := c.state.BoolExpression(literal.Var())
		if err != nil {
			return false, err
		}
		if !c.state.isRelevant(expression) {
			continue
		}
		ground, err := c.manager.IsGround(expression)
		if err != nil {
			return false, err
		}
		if !ground {
			continue
		}
		kind, err := c.manager.GetKind(expression)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTQuantifier {
			recursive, err := c.manager.IsRecursiveFunctionDefinition(expression)
			if err != nil {
				return false, err
			}
			if recursive {
				continue
			}
		}
		success, err := md.Eval(expression, out, false)
		if err != nil {
			return false, err
		}
		if !success {
			return true, nil
		}
		value, err := c.manager.RefGet(out)
		if err != nil {
			return false, err
		}
		if literal.Sign() && c.manager.IsTrue(value) || !literal.Sign() && c.manager.IsFalse(value) {
			return false, nil
		}
	}
	return true, nil
}
