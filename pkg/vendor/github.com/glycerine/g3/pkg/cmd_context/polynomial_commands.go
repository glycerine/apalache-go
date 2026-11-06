package cmd_context

// Source: src/cmd_context/extra_cmds/polynomial_cmds.cpp, to_poly.
// Factorization and root-isolation commands are separate port steps.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	primegen "github.com/glycerine/g3/pkg/util/prime"
)

// polynomialCommand owns the source command's temporary numeral manager,
// polynomial manager and expression translator.
type polynomialCommand struct {
	context   *Context
	ast       *ast.ASTManager
	rationals *unumeral.RationalManager
	primes    *primegen.Generator
	manager   *polynomial.MultivariatePolynomialManager
	converter *ast.Expr2Polynomial
}

func newPolynomialCommand(c *Context) (_ *polynomialCommand, resultErr error) {
	m, err := c.Manager()
	if err != nil {
		return nil, err
	}
	services, err := c.ModelEvaluatorConfig()
	if err != nil {
		return nil, err
	}
	p := &polynomialCommand{context: c, ast: m}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	p.rationals, err = m.NumeralService().NewPrivateRationalManager()
	if err != nil {
		return nil, err
	}
	p.primes, err = primegen.NewGenerator(m.Budget())
	if err != nil {
		return nil, err
	}
	p.manager, err = polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{
		Core: polynomial.MultivariatePolynomialConfig{
			Integer: p.rationals.Integers(), Rationals: p.rationals, Checkpoint: m.Limit.Inc,
			Cooperation: func() error { return services.Cooperation.Cooperate("polynomial") },
		},
		Factorization: polynomial.UPolynomialFactorizationConfig{Primes: p.primes},
	})
	if err != nil {
		return nil, err
	}
	p.converter, err = ast.NewExpr2Polynomial(ast.Expr2PolynomialConfig{AST: m, Polynomial: p.manager})
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *polynomialCommand) Close() (result error) {
	if p.converter != nil {
		result = errors.Join(result, p.converter.Close())
	}
	if p.manager != nil {
		result = errors.Join(result, p.manager.CloseChecked())
	}
	if p.primes != nil {
		result = errors.Join(result, p.primes.Close())
	}
	if p.rationals != nil {
		result = errors.Join(result, p.rationals.CloseChecked())
	}
	return result
}

// WritePolynomial translates an arithmetic expression into the source
// sum-of-monomials display. The polynomial manager is scoped to this command.
func (c *Context) WritePolynomial(out io.Writer, expression ast.ASTHandle) (resultErr error) {
	if out == nil {
		return ast.ErrASTHandle
	}
	command, err := newPolynomialCommand(c)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, command.Close()) }()
	m, converter := command.ast, command.converter
	value, denominator, matched, err := converter.ToPolynomial(expression)
	if err != nil {
		if errors.Is(err, ast.ErrExprNotPolynomial) {
			return fmt.Errorf("expression is not a polynomial")
		}
		return err
	}
	if !matched {
		return fmt.Errorf("expression is not a polynomial")
	}
	defer func() { resultErr = errors.Join(resultErr, value.Close(), denominator.Close()) }()
	p, err := value.Get()
	if err != nil {
		return err
	}
	translated, err := converter.ToExpr(p, true)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&translated)) }()
	result, err := m.RefGet(translated)
	if err != nil {
		return err
	}
	divisor, err := denominator.Display()
	if err != nil {
		return err
	}
	if divisor != "1" {
		if _, err = fmt.Fprintf(out, "(* %s ", divisor); err != nil {
			return err
		}
	}
	if err = c.WriteExpression(out, result); err != nil {
		return err
	}
	if divisor != "1" {
		if _, err = fmt.Fprint(out, ")"); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(out)
	return err
}

// WritePolynomialFactors factors an arithmetic expression and displays the
// source factor constant followed by distinct factors and their powers.
func (c *Context) WritePolynomialFactors(out io.Writer, expression ast.ASTHandle, params polynomial.UPolynomialFactorParams) (resultErr error) {
	if out == nil {
		return ast.ErrASTHandle
	}
	command, err := newPolynomialCommand(c)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, command.Close()) }()
	value, denominator, matched, err := command.converter.ToPolynomial(expression)
	if err != nil {
		if errors.Is(err, ast.ErrExprNotPolynomial) {
			return fmt.Errorf("expression is not a polynomial")
		}
		return err
	}
	if !matched {
		return fmt.Errorf("expression is not a polynomial")
	}
	defer func() { resultErr = errors.Join(resultErr, value.Close(), denominator.Close()) }()
	p, err := value.Get()
	if err != nil {
		return err
	}
	factors, err := polynomial.NewPolynomialFactors(command.manager)
	if err != nil {
		if factors != nil {
			err = errors.Join(err, factors.CloseChecked())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, factors.CloseChecked()) }()
	if err = command.manager.Factor(p, factors, params); err != nil {
		return err
	}
	constant, err := factors.Constant()
	if err != nil {
		return err
	}
	constantText, err := command.rationals.Integers().String(constant)
	if err != nil {
		return err
	}
	denominatorText, err := denominator.Display()
	if err != nil {
		return err
	}
	numerator, ok := new(big.Int).SetString(constantText, 10)
	if !ok {
		return ast.ErrASTDomain
	}
	divisor, ok := new(big.Int).SetString(denominatorText, 10)
	if !ok || divisor.Sign() == 0 {
		return ast.ErrASTDomain
	}
	factorConstant := new(big.Rat).SetFrac(numerator, divisor)
	if _, err = fmt.Fprintf(out, "(factors\n%s", factorConstant.RatString()); err != nil {
		return err
	}
	count, err := factors.Distinct()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		factor, err := factors.Factor(i)
		if err != nil {
			return err
		}
		degree, err := factors.Degree(i)
		if err != nil {
			return err
		}
		ref, err := command.converter.ToExpr(factor, true)
		if err != nil {
			return err
		}
		expression, err := command.ast.RefGet(ref)
		if err == nil {
			_, err = fmt.Fprintln(out)
		}
		if err == nil && degree > 1 {
			_, err = fmt.Fprint(out, "(^ ")
		}
		if err == nil {
			err = c.WriteExpression(out, expression)
		}
		if err == nil && degree > 1 {
			_, err = fmt.Fprintf(out, " %d)", degree)
		}
		err = errors.Join(err, command.ast.DelRef(&ref))
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(out, ")")
	return err
}
