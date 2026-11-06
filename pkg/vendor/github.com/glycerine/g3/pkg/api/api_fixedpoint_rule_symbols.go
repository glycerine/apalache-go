package api

// Source: src/api/api_datalog.cpp Z3_fixedpoint_add_rule/update_rule pass
// to_symbol(name) directly to the fixedpoint context.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// AddRuleWithSymbol preserves the rule label's source symbol identity.
func (f *APIFixedpoint) AddRuleWithSymbol(formula ast.ASTHandle, name symbol.Symbol) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.AddFixedpointRuleWithSymbol(formula, name)
}

// AddRuleWithSymbolAndBound also preserves the Duality recursion bound.
func (f *APIFixedpoint) AddRuleWithSymbolAndBound(formula ast.ASTHandle, name symbol.Symbol, bound uint32) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.AddFixedpointRuleWithSymbolAndBound(formula, name, bound)
}

// UpdateRuleWithSymbol matches a rule by its exact source symbol identity.
func (f *APIFixedpoint) UpdateRuleWithSymbol(formula ast.ASTHandle, name symbol.Symbol) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.UpdateFixedpointRuleWithSymbol(formula, name)
}

// AddFixedpointRuleWithSymbol preserves the rule label's source symbol identity.
func (c *SMTContext) AddFixedpointRuleWithSymbol(formula ast.ASTHandle, name symbol.Symbol) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddFixedpointRuleWithSymbol(formula, name)
}

// AddFixedpointRuleWithSymbolAndBound also preserves the Duality recursion bound.
func (c *SMTContext) AddFixedpointRuleWithSymbolAndBound(formula ast.ASTHandle, name symbol.Symbol, bound uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddFixedpointRuleWithSymbolAndBound(formula, name, bound)
}

// UpdateFixedpointRuleWithSymbol matches a rule by its source symbol identity.
func (c *SMTContext) UpdateFixedpointRuleWithSymbol(formula ast.ASTHandle, name symbol.Symbol) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.UpdateFixedpointRuleWithSymbol(formula, name)
}
