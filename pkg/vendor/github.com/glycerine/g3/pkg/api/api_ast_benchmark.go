package api

// Source: src/api/api_ast.cpp:853-878.

import (
	"bytes"
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type APIBenchmarkConfig struct {
	Name, Logic, Status, Source, Category string
	Attributes                            string
	Assumptions                           []ast.ASTHandle
}

func apiBenchmarkSymbol(m *ast.ASTManager, text string) (usymbol.Symbol, error) {
	if text == "" {
		return usymbol.Symbol{}, nil
	}
	return m.Symbols.Intern(text)
}

func (c *SMTContext) BenchmarkSMT2(formula ast.ASTHandle, config APIBenchmarkConfig) (result string, resultErr error) {
	m, err := c.Manager()
	if err != nil {
		return "", err
	}
	name, err := apiBenchmarkSymbol(m, config.Name)
	if err != nil {
		return "", err
	}
	logic, err := apiBenchmarkSymbol(m, config.Logic)
	if err != nil {
		return "", err
	}
	status, err := apiBenchmarkSymbol(m, config.Status)
	if err != nil {
		return "", err
	}
	source, err := apiBenchmarkSymbol(m, config.Source)
	if err != nil {
		return "", err
	}
	category, err := apiBenchmarkSymbol(m, config.Category)
	if err != nil {
		return "", err
	}
	printer, err := ast.NewLegacySMTPrinter(m)
	if err != nil {
		return "", err
	}
	globals, err := c.command.GlobalParameters()
	if err != nil {
		return "", err
	}
	settings, err := ast.NewPPParams(globals.Context, globals, nil)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, settings.Close()) }()
	simplify, err := settings.SimplifyImplies()
	if err != nil {
		return "", err
	}
	if err := printer.SetSimplifyImplies(simplify); err != nil {
		return "", err
	}
	var output bytes.Buffer
	err = printer.DisplayBenchmark(&output, formula, ast.LegacySMTBenchmarkConfig{
		BenchmarkName: name, Logic: logic, Status: status, SourceInfo: source,
		Category: category, Attributes: config.Attributes, Assumptions: config.Assumptions,
	})
	if err != nil {
		return "", err
	}
	return output.String(), nil
}
