package cmd_context

// Source: src/opt/opt_context.cpp:198-205, O_MAXSMT objective display term.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *Context) optimizationSoftExpression(objective *optimizationObjective) (result ast.ASTRef, resultErr error) {
	if objective == nil || !objective.soft {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	var temporaries []ast.ASTRef
	defer func() {
		for i := len(temporaries) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&temporaries[i]))
		}
	}()
	zero, err := c.manager.ArithmeticRealNumeralText("0")
	if err != nil {
		return ast.ASTRef{}, err
	}
	zeroRef, err := c.manager.NewRef(zero)
	if err != nil {
		return ast.ASTRef{}, err
	}
	temporaries = append(temporaries, zeroRef)
	penalties := make([]ast.ASTHandle, 0, len(objective.softTerms))
	for _, term := range objective.softTerms {
		weight, e := c.manager.ArithmeticNumeralRat(&term.weight, false)
		if e != nil {
			return ast.ASTRef{}, e
		}
		weightRef, e := c.manager.NewRef(weight)
		if e != nil {
			return ast.ASTRef{}, e
		}
		temporaries = append(temporaries, weightRef)
		penalty, e := c.manager.MkIte(term.expression, zero, weight)
		if e != nil {
			return ast.ASTRef{}, e
		}
		penaltyRef, e := c.manager.NewRef(penalty)
		if e != nil {
			return ast.ASTRef{}, e
		}
		temporaries = append(temporaries, penaltyRef)
		penalties = append(penalties, penalty)
	}
	if len(penalties) == 0 {
		return c.manager.NewRef(zero)
	}
	family, err := c.manager.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTRef{}, err
	}
	sum, err := c.manager.MkFamilyApp(family, 6, nil, penalties)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.manager.NewRef(sum)
}
