package tactic

// Source: src/nlsat/tactic/goal2nlsat.cpp:298-445.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/nlsat"
	"github.com/glycerine/g3/pkg/sat"
)

// GoalConverter reconstructs AST literals from a retained NLSAT solver. The
// supplied mappings and solver are borrowed and must outlive the converter.
type GoalConverter struct {
	manager   *ast.ASTManager
	solver    *nlsat.Solver
	core      *polynomial.MultivariatePolynomialCore
	monomials *polynomial.MonomialManager
	booleans  map[uint32]ast.ASTHandle
	terms     map[uint32]ast.ASTHandle
}

func NewGoalConverter(manager *ast.ASTManager, solver *nlsat.Solver, atoms, terms *ast.Expr2Var) (*GoalConverter, error) {
	if manager == nil || solver == nil || atoms == nil || terms == nil {
		return nil, arena.ErrConfig
	}
	polynomials, err := solver.PolynomialManager()
	if err != nil {
		return nil, err
	}
	core, err := polynomials.ArithmeticCore()
	if err != nil {
		return nil, err
	}
	monomials, err := core.MonomialManager()
	if err != nil {
		return nil, err
	}
	converter := &GoalConverter{
		manager: manager, solver: solver, core: core, monomials: monomials,
		booleans: make(map[uint32]ast.ASTHandle), terms: make(map[uint32]ast.ASTHandle),
	}
	atomEntries, err := atoms.Entries()
	if err != nil {
		return nil, err
	}
	for _, entry := range atomEntries {
		converter.booleans[entry.Variable] = entry.Expr
	}
	termEntries, err := terms.Entries()
	if err != nil {
		return nil, err
	}
	for _, entry := range termEntries {
		converter.terms[entry.Variable] = entry.Expr
	}
	return converter, nil
}

func (c *GoalConverter) numeral(text string, integer bool) (ast.ASTHandle, error) {
	if integer {
		return c.manager.ArithmeticIntegerNumeralText(text)
	}
	return c.manager.ArithmeticRealNumeralText(text)
}

func (c *GoalConverter) product(arguments []ast.ASTHandle, integer bool) (ast.ASTHandle, error) {
	if len(arguments) == 0 {
		return c.numeral("1", integer)
	}
	result := arguments[0]
	var err error
	for _, argument := range arguments[1:] {
		result, err = c.manager.MkArithmeticMul(result, argument)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

func (c *GoalConverter) sum(arguments []ast.ASTHandle, integer bool) (ast.ASTHandle, error) {
	if len(arguments) == 0 {
		return c.numeral("0", integer)
	}
	result := arguments[0]
	var err error
	for _, argument := range arguments[1:] {
		result, err = c.manager.MkArithmeticAdd(result, argument)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

func (c *GoalConverter) monomialExpression(monomial polynomial.Monomial, coefficientText string, coefficientOne, integer bool) (ast.ASTHandle, error) {
	info, err := c.monomials.Info(monomial)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 0, info.Size+1)
	for i := uint32(0); i < info.Size; i++ {
		power, cause := c.monomials.ReadPower(monomial, i)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		term, found := c.terms[power.Var]
		if !found {
			return ast.ASTHandle{}, arena.ErrBounds
		}
		if power.Degree != 1 {
			exponent, cause := c.numeral(fmt.Sprint(power.Degree), integer)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			term, cause = c.manager.MkArithmeticPower(term, exponent)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
		}
		arguments = append(arguments, term)
	}
	if !coefficientOne {
		coefficient, cause := c.numeral(coefficientText, integer)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		arguments = append(arguments, coefficient)
	}
	return c.product(arguments, integer)
}

func (c *GoalConverter) polynomialIsInteger(p polynomial.Polynomial) (bool, error) {
	info, err := c.core.Info(p)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < info.Size; i++ {
		monomial, cause := c.core.TermMonomial(p, i)
		if cause != nil {
			return false, cause
		}
		monomialInfo, cause := c.monomials.Info(monomial)
		if cause != nil {
			return false, cause
		}
		for j := uint32(0); j < monomialInfo.Size; j++ {
			power, cause := c.monomials.ReadPower(monomial, j)
			if cause != nil {
				return false, cause
			}
			integer, cause := c.solver.IsExprPolynomialInteger(power.Var)
			if cause != nil || !integer {
				return false, cause
			}
		}
	}
	return true, nil
}

func (c *GoalConverter) polynomialExpression(p polynomial.Polynomial, integer bool) (ast.ASTHandle, error) {
	info, err := c.core.Info(p)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	integers, err := c.core.IntegerManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 0, info.Size)
	for i := uint32(0); i < info.Size; i++ {
		coefficient, cause := c.core.Coefficient(p, i)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		text, cause := integers.String(&coefficient)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		one, cause := integers.IsOne(&coefficient)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		monomial, cause := c.core.TermMonomial(p, i)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		term, cause := c.monomialExpression(monomial, text, one, integer)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		arguments = append(arguments, term)
	}
	return c.sum(arguments, integer)
}

func (c *GoalConverter) rootSymbol(variable sat.BoolVar) (ast.ASTHandle, error) {
	sort, err := c.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	name, err := c.manager.Symbols.Intern(fmt.Sprintf("nlsat-root-%d", variable))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.manager.MkConst(name, sort)
}

// ConvertLiteral returns an owning reference to the AST reconstruction of l.
func (c *GoalConverter) ConvertLiteral(l sat.Literal) (_ ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || c.solver == nil {
		return ast.ASTRef{}, arena.ErrClosed
	}
	if expression, found := c.booleans[uint32(l.Variable())]; found {
		if l.Sign() {
			var err error
			expression, err = c.manager.MkNot(expression)
			if err != nil {
				return ast.ASTRef{}, err
			}
		}
		return c.manager.NewRef(expression)
	}
	atom, found, err := c.solver.AtomForBooleanVariable(l.Variable())
	if err != nil || !found {
		return ast.ASTRef{}, errors.Join(ast.ErrASTDomain, err)
	}
	inequality, ok := atom.(*nlsat.InequalityAtom)
	if !ok {
		expression, cause := c.rootSymbol(l.Variable())
		if cause != nil {
			return ast.ASTRef{}, cause
		}
		if l.Sign() {
			expression, cause = c.manager.MkNot(expression)
			if cause != nil {
				return ast.ASTRef{}, cause
			}
		}
		return c.manager.NewRef(expression)
	}
	integer := true
	factors := make([]ast.ASTHandle, 0, inequality.Size())
	for i := uint32(0); i < inequality.Size(); i++ {
		p, cause := inequality.Polynomial(i)
		if cause != nil {
			return ast.ASTRef{}, cause
		}
		factorInteger, cause := c.polynomialIsInteger(p)
		if cause != nil {
			return ast.ASTRef{}, cause
		}
		integer = integer && factorInteger
	}
	for i := uint32(0); i < inequality.Size(); i++ {
		p, cause := inequality.Polynomial(i)
		if cause != nil {
			return ast.ASTRef{}, cause
		}
		factor, cause := c.polynomialExpression(p, integer)
		if cause != nil {
			return ast.ASTRef{}, cause
		}
		even, cause := inequality.IsEven(i)
		if cause != nil {
			return ast.ASTRef{}, cause
		}
		if even {
			two, cause := c.numeral("2", integer)
			if cause != nil {
				return ast.ASTRef{}, cause
			}
			factor, cause = c.manager.MkArithmeticPower(factor, two)
			if cause != nil {
				return ast.ASTRef{}, cause
			}
		}
		factors = append(factors, factor)
	}
	product, err := c.product(factors, integer)
	if err != nil {
		return ast.ASTRef{}, err
	}
	zero, err := c.numeral("0", integer)
	if err != nil {
		return ast.ASTRef{}, err
	}
	var expression ast.ASTHandle
	switch inequality.Kind() {
	case nlsat.AtomEQ:
		expression, err = c.manager.MkEq(product, zero)
		if err == nil && l.Sign() {
			expression, err = c.manager.MkNot(expression)
		}
	case nlsat.AtomLT:
		if l.Sign() {
			expression, err = c.manager.MkArithmeticGE(product, zero)
		} else {
			expression, err = c.manager.MkArithmeticLT(product, zero)
		}
	case nlsat.AtomGT:
		if l.Sign() {
			expression, err = c.manager.MkArithmeticLE(product, zero)
		} else {
			expression, err = c.manager.MkArithmeticGT(product, zero)
		}
	default:
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.manager.NewRef(expression)
}
