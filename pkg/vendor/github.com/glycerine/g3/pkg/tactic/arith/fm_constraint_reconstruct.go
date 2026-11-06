package arith

// Source: src/tactic/arith/fm_tactic.cpp:870-913.
import "github.com/glycerine/g3/pkg/ast"

func (t *FMExpressionTable) ConstraintExpression(c *FMConstraint) (ast.ASTHandle, error) {
	if t == nil || t.closed || c == nil {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	var inequality ast.ASTHandle
	if len(c.Variables) == 0 {
		if c.Constant.Sign() > 0 || !c.Strict && c.Constant.Sign() == 0 {
			return t.manager.MkTrue()
		}
	} else {
		lhs, rhs, err := t.constraintPolynomial(c)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if c.Strict {
			inequality, err = t.manager.MkArithmeticGE(lhs, rhs)
			if err == nil {
				inequality, err = t.manager.MkNot(inequality)
			}
		} else {
			inequality, err = t.manager.MkArithmeticLE(lhs, rhs)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if len(c.Literals) == 0 {
		if inequality.IsNull() {
			return t.manager.MkFalse()
		}
		return inequality, nil
	}
	args := make([]ast.ASTHandle, 0, len(c.Literals)+1)
	for _, literal := range c.Literals {
		term, err := t.Boolean(fmLiteralVariable(literal))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if literal < 0 {
			term, err = t.manager.MkNot(term)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		args = append(args, term)
	}
	if !inequality.IsNull() {
		args = append(args, inequality)
	}
	return t.manager.MkOrArray(args)
}
