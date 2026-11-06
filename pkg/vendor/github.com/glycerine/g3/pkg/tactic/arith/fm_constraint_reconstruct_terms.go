package arith

// Source: src/tactic/arith/fm_tactic.cpp:879-903.
import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func fmIsOne(v *big.Rat) bool { return v.IsInt() && v.Num().IsInt64() && v.Num().Int64() == 1 }

func (t *FMExpressionTable) constraintPolynomial(c *FMConstraint) (ast.ASTHandle, ast.ASTHandle, error) {
	integer := t.variables.AllInteger(c)
	terms := make([]ast.ASTHandle, 0, len(c.Variables))
	for i, v := range c.Variables {
		x, err := t.Variable(v)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		if !integer && t.variables.IsInteger(v) {
			family, err := t.manager.RegisterArithmeticPlugin()
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
			x, err = t.manager.MkFamilyApp(family, 14, nil, []ast.ASTHandle{x})
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
		}
		if !fmIsOne(&c.Coefficients[i]) {
			numeral, err := t.numeral(&c.Coefficients[i], integer)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
			x, err = t.manager.MkArithmeticMul(numeral, x)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
		}
		terms = append(terms, x)
	}
	lhs := terms[0]
	if len(terms) > 1 {
		family, err := t.manager.RegisterArithmeticPlugin()
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		lhs, err = t.manager.MkFamilyApp(family, 6, nil, terms)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
	}
	rhs, err := t.numeral(&c.Constant, integer)
	return lhs, rhs, err
}

func (t *FMExpressionTable) numeral(v *big.Rat, integer bool) (ast.ASTHandle, error) {
	if integer {
		return t.manager.ArithmeticIntegerNumeralText(v.RatString())
	}
	return t.manager.ArithmeticRealNumeralText(v.RatString())
}
