package api

// Source: src/api/api_pb.cpp:24-108.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c *SMTContext) pseudoBooleanManager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	m, err := c.command.Manager()
	if err != nil {
		return nil, err
	}
	if _, err = m.RegisterPseudoBooleanPlugin(); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *SMTContext) PseudoBooleanAtMost(arguments []ast.ASTHandle, bound uint32) (ast.ASTHandle, error) {
	m, err := c.pseudoBooleanManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkPseudoBooleanAtMost(arguments, bound)
}

func (c *SMTContext) PseudoBooleanAtLeast(arguments []ast.ASTHandle, bound uint32) (ast.ASTHandle, error) {
	m, err := c.pseudoBooleanManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkPseudoBooleanAtLeast(arguments, bound)
}

func apiIntegerRationals(m *ast.ASTManager, coefficients []int64, bound int64) (values []unumeral.OwnedRational, limit unumeral.OwnedRational, cleanup func() error, err error) {
	service := m.NumeralService()
	values = make([]unumeral.OwnedRational, len(coefficients))
	created := 0
	limitCreated := false
	cleanup = func() (result error) {
		for i := created - 1; i >= 0; i-- {
			result = errors.Join(result, service.Del(&values[i]))
		}
		if limitCreated {
			result = errors.Join(result, service.Del(&limit))
		}
		return result
	}
	limit, err = service.NewValue()
	if err != nil {
		return
	}
	limitCreated = true
	if err = service.SetInt64(&limit, bound); err != nil {
		return
	}
	for i, coefficient := range coefficients {
		values[i], err = service.NewValue()
		if err != nil {
			return
		}
		created++
		if err = service.SetInt64(&values[i], coefficient); err != nil {
			return
		}
	}
	return
}

func (c *SMTContext) pseudoBooleanLinear(arguments []ast.ASTHandle, coefficients []int64, bound int64, build func(*ast.ASTManager, []unumeral.OwnedRational, []ast.ASTHandle, *unumeral.OwnedRational) (ast.ASTHandle, error)) (result ast.ASTHandle, err error) {
	if len(arguments) != len(coefficients) {
		return result, ast.ErrASTDomain
	}
	m, err := c.pseudoBooleanManager()
	if err != nil {
		return result, err
	}
	values, limit, cleanup, err := apiIntegerRationals(m, coefficients, bound)
	if cleanup != nil {
		defer func() { err = errors.Join(err, cleanup()) }()
	}
	if err != nil {
		return result, err
	}
	return build(m, values, arguments, &limit)
}

func (c *SMTContext) PseudoBooleanLE(arguments []ast.ASTHandle, coefficients []int64, bound int64) (ast.ASTHandle, error) {
	return c.pseudoBooleanLinear(arguments, coefficients, bound, (*ast.ASTManager).MkPseudoBooleanLE)
}
func (c *SMTContext) PseudoBooleanGE(arguments []ast.ASTHandle, coefficients []int64, bound int64) (ast.ASTHandle, error) {
	return c.pseudoBooleanLinear(arguments, coefficients, bound, (*ast.ASTManager).MkPseudoBooleanGE)
}
func (c *SMTContext) PseudoBooleanEQ(arguments []ast.ASTHandle, coefficients []int64, bound int64) (ast.ASTHandle, error) {
	return c.pseudoBooleanLinear(arguments, coefficients, bound, (*ast.ASTManager).MkPseudoBooleanEQ)
}
