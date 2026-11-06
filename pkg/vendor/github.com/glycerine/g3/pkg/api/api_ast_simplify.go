package api

// Source: src/api/api_ast.cpp:679-731.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Simplify rewrites expression with the source theory rewriter's default
// parameters. The returned reference is owning and must be released with
// Manager.DelRef before the context is closed.
func (c *SMTContext) Simplify(expression ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return c.simplifyWithAPIInterrupt(expression, nil)
}

// SimplifyWithParameters rewrites expression with a caller-owned parameter
// set. Parameters must belong to this context and remain borrowed for only the
// duration of the call. The returned reference is caller-owned.
func (c *SMTContext) SimplifyWithParameters(expression ast.ASTHandle, parameters *ParameterSet) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if err := parameters.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if parameters.owner != c.smtContextState {
		return ast.ASTRef{}, ErrSMTContextOwner
	}
	return c.simplifyWithAPIInterrupt(expression, &parameters.value)
}

// SimplifyParameterDescriptions returns an owning description object for the
// exact parameter set accepted by SimplifyWithParameters.
func (c *SMTContext) SimplifyParameterDescriptions() (*ParameterDescriptions, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	globals, err := c.command.GlobalParameters()
	if err != nil {
		return nil, err
	}
	descriptions, err := uparams.NewParamDescrs(globals.Context.Symbols(), uparams.ParamDescrsConfig{Budget: globals.Context.Budget()})
	if err != nil {
		return nil, err
	}
	if err = rewriter.TheoryRewriterParameterDescriptions(descriptions); err != nil {
		return nil, errors.Join(err, descriptions.Close())
	}
	return &ParameterDescriptions{owner: c.smtContextState, value: descriptions, symbols: globals.Context.Symbols()}, nil
}

// SimplifyHelp formats the source simplifier parameter descriptions.
func (c *SMTContext) SimplifyHelp() (text string, resultErr error) {
	descriptions, err := c.SimplifyParameterDescriptions()
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, descriptions.Close()) }()
	return descriptions.value.Display(0, false, true)
}
