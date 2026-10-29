package ast

// Source: src/ast/ast_smt_pp.cpp:856-1011; src/ast/ast_smt_pp.h:39-85.

import (
	"io"
	"strings"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type LegacySMTBenchmarkConfig struct {
	BenchmarkName, SourceInfo, Status, Category, Logic usymbol.Symbol
	Attributes                                         string
	Assumptions, AssumptionsStar                       []ASTHandle
	IsDeclaredSort, IsDeclaredFunc                     func(ASTHandle) (bool, error)
}

func (p *LegacySMTPrinter) benchmarkSymbolText(symbol usymbol.Symbol) (string, error) {
	if symbol.IsNull() {
		return "", nil
	}
	return p.manager.Symbols.Display(symbol)
}

func (p *LegacySMTPrinter) benchmarkText(root ASTHandle, config LegacySMTBenchmarkConfig) (string, error) {
	collector := newASTDeclCollector(p.manager, true)
	for _, assumptions := range [][]ASTHandle{config.Assumptions, config.AssumptionsStar} {
		for _, assumption := range assumptions {
			if err := collector.visit(assumption); err != nil {
				return "", err
			}
		}
	}
	if err := collector.visit(root); err != nil {
		return "", err
	}
	if err := collector.orderDeps(); err != nil {
		return "", err
	}
	rootSort, err := p.manager.GetSort(root)
	if err != nil {
		return "", err
	}
	boolSort, err := p.manager.MkBoolSort()
	if err != nil {
		return "", err
	}
	isBool := rootSort == boolSort
	isProof, err := p.manager.IsProof(root)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	writeCommentSymbol := func(prefix string, symbol usymbol.Symbol, suffix string, omitEmpty bool) error {
		text, e := p.benchmarkSymbolText(symbol)
		if e != nil {
			return e
		}
		if symbol.IsNull() || omitEmpty && text == "" {
			return nil
		}
		out.WriteString(prefix)
		out.WriteString(text)
		out.WriteString(suffix)
		return nil
	}
	if isProof {
		out.WriteByte('(')
	}
	if err = writeCommentSymbol("; ", config.BenchmarkName, "\n", false); err != nil {
		return "", err
	}
	if err = writeCommentSymbol("; :source { ", config.SourceInfo, " }\n", true); err != nil {
		return "", err
	}
	if isBool {
		status := "unknown"
		if !config.Status.IsNull() {
			status, err = p.manager.Symbols.Display(config.Status)
			if err != nil {
				return "", err
			}
		}
		out.WriteString("(set-info :status ")
		out.WriteString(status)
		out.WriteString(")\n")
	}
	if err = writeCommentSymbol("; :category { ", config.Category, " }\n", true); err != nil {
		return "", err
	}
	if logic, e := p.benchmarkSymbolText(config.Logic); e != nil {
		return "", e
	} else if logic != "" {
		out.WriteString("(set-logic ")
		out.WriteString(logic)
		out.WriteString(")\n")
	}
	if config.Attributes != "" {
		out.WriteString("; ")
		out.WriteString(config.Attributes)
	}
	declaredDatatypes := make(map[ASTHandle]bool)
	for _, sort := range collector.sorts {
		if config.IsDeclaredSort != nil {
			declared, e := config.IsDeclaredSort(sort)
			if e != nil {
				return "", e
			}
			if declared {
				continue
			}
		}
		view, e := p.manager.smt2SortView(sort)
		if e != nil {
			return "", e
		}
		var text string
		if view.datatypeSort {
			text, e = p.datatypeDeclarationText(sort, declaredDatatypes)
		} else {
			text, e = p.sortDeclarationText(sort)
		}
		if e != nil {
			return "", e
		}
		out.WriteString(text)
	}
	for _, declarations := range [][]ASTHandle{collector.decls, collector.predicates} {
		for _, decl := range declarations {
			if config.IsDeclaredFunc != nil {
				declared, e := config.IsDeclaredFunc(decl)
				if e != nil {
					return "", e
				}
				if declared {
					continue
				}
			}
			text, e := p.funcDeclarationText(decl)
			if e != nil {
				return "", e
			}
			out.WriteString(text)
			out.WriteByte('\n')
		}
	}
	writeAssertion := func(expr ASTHandle) error {
		text, e := p.exprTextIndented(expr, nil, 1)
		if e != nil {
			return e
		}
		out.WriteString("(assert\n ")
		out.WriteString(text)
		out.WriteString(")\n")
		return nil
	}
	for _, assumption := range config.Assumptions {
		if err = writeAssertion(assumption); err != nil {
			return "", err
		}
	}
	for _, assumption := range config.AssumptionsStar {
		if err = writeAssertion(assumption); err != nil {
			return "", err
		}
	}
	if isBool {
		if !p.manager.IsTrue(root) {
			if err = writeAssertion(root); err != nil {
				return "", err
			}
		}
		out.WriteString("(check-sat)\n")
	} else if isProof {
		text, e := p.exprText(root, nil)
		if e != nil {
			return "", e
		}
		out.WriteString("(proof\n")
		out.WriteString(text)
		out.WriteString("))\n")
	} else {
		text, e := p.exprText(root, nil)
		if e != nil {
			return "", e
		}
		out.WriteString(text)
	}
	return out.String(), nil
}

func (p *LegacySMTPrinter) DisplayBenchmark(out io.Writer, root ASTHandle, config LegacySMTBenchmarkConfig) error {
	if p == nil || p.manager == nil || p.renaming == nil {
		return ErrASTHandle
	}
	body, err := p.benchmarkText(root, config)
	if err != nil {
		return err
	}
	return legacySMTWriteText(out, body)
}
