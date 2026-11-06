package cmd_context

import (
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

type OPBLoadSummary struct {
	Variables   int
	Constraints int
	Objectives  int
}

// LoadOPB expects an arithmetic-capable command context selected before its
// manager is initialized. NewOPBFrontend selects QF_LIA for the live shell
// consumer; direct users retain control of their solver strategy.
func (c *Context) LoadOPB(document opt.OPBDocument) (OPBLoadSummary, error) {
	if c == nil {
		return OPBLoadSummary{}, fmt.Errorf("opb: nil command context")
	}
	manager, err := c.Manager()
	if err != nil {
		return OPBLoadSummary{}, err
	}
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return OPBLoadSummary{}, err
	}
	declarations := make(map[int32]ast.ASTHandle)
	summary := OPBLoadSummary{}
	for index := range document.Objectives {
		objective := &document.Objectives[index]
		if len(objective.Terms) == 0 {
			return OPBLoadSummary{}, fmt.Errorf("opb: objective %d is empty", index+1)
		}
		expression, e := c.makeOPBSum(objective.Terms, objective.MinimizeSyntax, declarations, zero)
		if e != nil {
			return OPBLoadSummary{}, fmt.Errorf("opb: objective %d: %w", index+1, e)
		}
		if e = c.AddObjective(expression, false); e != nil {
			return OPBLoadSummary{}, fmt.Errorf("opb: objective %d: %w", index+1, e)
		}
		summary.Objectives++
	}
	for index := range document.Constraints {
		constraint := &document.Constraints[index]
		if len(constraint.Terms) == 0 {
			return OPBLoadSummary{}, fmt.Errorf("opb: constraint %d is empty", index+1)
		}
		if len(constraint.Terms) == 1 {
			formula, e := c.makeSingleOPBConstraint(constraint, declarations)
			if e == nil {
				e = c.Assert(formula, "")
			}
			if e != nil {
				return OPBLoadSummary{}, fmt.Errorf("opb: constraint %d: %w", index+1, e)
			}
			summary.Constraints++
			continue
		}
		left, e := c.makeOPBSum(constraint.Terms, true, declarations, zero)
		if e != nil {
			return OPBLoadSummary{}, fmt.Errorf("opb: constraint %d: %w", index+1, e)
		}
		right, e := manager.ArithmeticNumeralRat(new(big.Rat).SetInt(&constraint.Bound), true)
		if e != nil {
			return OPBLoadSummary{}, e
		}
		var formula ast.ASTHandle
		switch constraint.Relation {
		case opt.OPBGreaterEqual:
			formula, e = manager.MkArithmeticGE(left, right)
		case opt.OPBEqual:
			formula, e = manager.MkEq(left, right)
		case opt.OPBLessEqual:
			formula, e = manager.MkArithmeticLE(left, right)
		default:
			return OPBLoadSummary{}, fmt.Errorf("opb: constraint %d has invalid relation", index+1)
		}
		if e == nil {
			e = c.Assert(formula, "")
		}
		if e != nil {
			return OPBLoadSummary{}, fmt.Errorf("opb: constraint %d: %w", index+1, e)
		}
		summary.Constraints++
	}
	summary.Variables = len(declarations)
	return summary, nil
}
