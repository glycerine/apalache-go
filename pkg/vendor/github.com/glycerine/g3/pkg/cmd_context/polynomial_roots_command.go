package cmd_context

// Source: src/cmd_context/extra_cmds/polynomial_cmds.cpp,
// poly_isolate_roots_cmd::context.

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialRootStream struct{ out io.Writer }

func (s polynomialRootStream) write(value string) error {
	n, err := io.WriteString(s.out, value)
	if err == nil && n != len(value) {
		return io.ErrShortWrite
	}
	return err
}
func (s polynomialRootStream) AppendCString(value string) error     { return s.write(value) }
func (s polynomialRootStream) AppendNumeralText(value string) error { return s.write(value) }
func (s polynomialRootStream) AppendInt32(value int32) error {
	return s.write(strconv.FormatInt(int64(value), 10))
}

// WritePolynomialRoots isolates roots after applying the supplied alternating
// variable/value assignments. Input AST handles and the writer are borrowed.
func (c *Context) WritePolynomialRoots(out io.Writer, expression ast.ASTHandle, assignments []ast.ASTHandle) (resultErr error) {
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
	globals, err := c.GlobalParameters()
	if err != nil {
		return err
	}
	services, err := c.ModelEvaluatorConfig()
	if err != nil {
		return err
	}
	params, err := globals.Context.NewValue()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, globals.Context.Del(&params)) }()
	limit, err := control.NewReslimit(command.ast.Budget())
	if err != nil {
		return err
	}
	defer limit.Close()
	algebraic, err := polynomial.NewAlgebraicManager(polynomial.AlgebraicManagerConfig{
		Rationals: command.ast.NumeralService(), Numerals: command.rationals,
		Limit: limit, Cooperation: services.Cooperation, Params: &params, Globals: globals,
		Factorization: polynomial.UPolynomialFactorizationConfig{Primes: command.primes},
	})
	if err != nil {
		if algebraic != nil {
			err = errors.Join(err, algebraic.Close())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, algebraic.Close()) }()
	values, err := polynomial.NewSimpleAlgebraicValues(algebraic.AlgebraicRootCore)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, values.Close()) }()
	mapping, err := command.converter.Mapping()
	if err != nil {
		return err
	}
	for i := 0; i < len(assignments); i += 2 {
		valid, err := command.converter.IsVar(assignments[i])
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("invalid assignment, argument is not a variable in the given polynomial")
		}
		variable, err := mapping.ToVar(assignments[i])
		if err != nil {
			return err
		}
		if i+1 == len(assignments) {
			break
		}
		if err = assignPolynomialRootValue(command.ast, algebraic, values, variable, assignments[i+1]); err != nil {
			return err
		}
	}
	variables, err := polynomial.NewPolynomialVariableDegrees(command.ast.NumeralService().RuntimeOwner())
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, variables.CloseChecked()) }()
	if err = command.manager.Vars(p, variables); err != nil {
		return err
	}
	count, err := variables.Len()
	if err != nil {
		return err
	}
	var assigned uint32
	for i := uint32(0); i < count; i++ {
		v, err := variables.At(i)
		if err != nil {
			return err
		}
		found, err := values.Contains(v)
		if err != nil {
			return err
		}
		if found {
			assigned++
		}
	}
	if assigned != count && assigned+1 != count {
		return fmt.Errorf("given assignment is not sufficient to make the given polynomial univariate")
	}
	roots, err := algebraic.NewVector()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, roots.Close()) }()
	if err = algebraic.IsolateMultivariateRoots(command.manager, p, values, roots); err != nil {
		return err
	}
	pp, err := ast.NewPPParams(globals.Context, globals, nil)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, pp.Close()) }()
	decimal, err := pp.Decimal()
	if err != nil {
		return err
	}
	if _, err = fmt.Fprint(out, "(roots"); err != nil {
		return err
	}
	for i := uint32(0); i < roots.Len(); i++ {
		root, err := roots.Borrow(i)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintln(out); err != nil {
			return err
		}
		if decimal {
			err = algebraic.DisplayDecimal(out, root, 10)
		} else {
			err = algebraic.DisplayRootSMT2Into(polynomialRootStream{out}, root)
		}
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(out, ")")
	return err
}

func assignPolynomialRootValue(m *ast.ASTManager, algebraic *polynomial.AlgebraicManager, values *polynomial.SimpleAlgebraicValues, variable uint32, expression ast.ASTHandle) (resultErr error) {
	var value polynomial.Algebraic
	defer func() { resultErr = errors.Join(resultErr, algebraic.Del(&value)) }()
	text, _, isNumeral, err := m.ArithmeticNumeralText(expression)
	if err != nil {
		return err
	}
	if isNumeral {
		q, err := algebraic.RationalManager()
		if err != nil {
			return err
		}
		rational := numeral.NewRational()
		defer func() { resultErr = errors.Join(resultErr, q.Del(&rational)) }()
		if err = q.SetString(&rational, text); err != nil {
			return err
		}
		if err = algebraic.SetRational(&value, &rational); err != nil {
			return err
		}
	} else {
		source, err := m.ArithmeticAlgebraicManager()
		if err != nil {
			return err
		}
		var original polynomial.Algebraic
		defer func() { resultErr = errors.Join(resultErr, source.Del(&original)) }()
		matched, err := m.ArithmeticAlgebraicNumeralValue(expression, &original)
		if err != nil {
			return err
		}
		if !matched {
			return fmt.Errorf("invalid assignment, argument is not a value")
		}
		if err = algebraic.CopyFromCore(&value, source.AlgebraicRootCore, &original); err != nil {
			return err
		}
	}
	return values.Push(variable, &value)
}
