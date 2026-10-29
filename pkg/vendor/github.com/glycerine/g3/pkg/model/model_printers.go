package model

// Source: src/model/model_pp.cpp; src/model/model_smt2_pp.cpp.

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usmt2 "github.com/glycerine/g3/pkg/util/smt2"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type modelPrinter struct {
	model   *ModelCore
	out     io.Writer
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
}

func (p *modelPrinter) write(text string) error {
	_, err := io.WriteString(p.out, text)
	return err
}

func (p *modelPrinter) indent(n uint32) error {
	return p.write(strings.Repeat(" ", int(n)))
}

func (p *modelPrinter) render(h ast.ASTHandle, names []usymbol.Symbol) (text string, result error) {
	format, err := p.model.manager.SMT2PPExprTree(h, names)
	if err != nil {
		return "", err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: p.model.manager.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	empty := p.context.EmptyValue()
	if err = p.model.manager.PrintFormat(stream, format, p.context, &empty, p.globals); err != nil {
		return "", err
	}
	return stream.String()
}

func (p *modelPrinter) renderSort(h ast.ASTHandle) (text string, result error) {
	format, err := p.model.manager.SMT2PPSort(h)
	if err != nil {
		return "", err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: p.model.manager.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	empty := p.context.EmptyValue()
	if err = p.model.manager.PrintFormat(stream, format, p.context, &empty, p.globals); err != nil {
		return "", err
	}
	return stream.String()
}

func (p *modelPrinter) rawName(d ast.ASTHandle) (string, error) {
	name, err := p.model.manager.GetASTName(d)
	if err != nil {
		return "", err
	}
	return p.model.manager.Symbols.String(name)
}

func (p *modelPrinter) smt2Name(d ast.ASTHandle) (string, error) {
	name, err := p.model.manager.GetASTName(d)
	if err != nil {
		return "", err
	}
	quoted, err := usmt2.IsSMT2QuotedSymbol(p.model.manager.Symbols, name)
	if err != nil {
		return "", err
	}
	if quoted {
		return usmt2.MkSMT2QuotedSymbol(p.model.manager.Symbols, name, nil)
	}
	return p.model.manager.Symbols.String(name)
}

func newModelPrinter(md *ModelCore, out io.Writer, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*modelPrinter, error) {
	if err := md.open(); err != nil {
		return nil, err
	}
	if out == nil || context == nil || globals == nil || globals.Context != context || context.SymbolsField != md.manager.Symbols {
		return nil, ast.ErrASTHandle
	}
	return &modelPrinter{model: md, out: out, context: context, globals: globals}, nil
}

// WriteDebug implements model_pp's compact debugging syntax in registration
// order. The parameter services are borrowed for the AST pretty printer.
func (md *ModelCore) WriteDebug(out io.Writer, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) error {
	p, err := newModelPrinter(md, out, context, globals)
	if err != nil {
		return err
	}
	if md.Derived != nil {
		for _, sort := range md.Derived.sorts {
			sortText, e := p.renderSort(sort)
			if e != nil {
				return e
			}
			if e = p.write("(define-sort " + sortText); e != nil {
				return e
			}
			u, found := md.Derived.Universes.Get2(sort)
			if !found || u == nil {
				return ast.ErrASTDomain
			}
			for _, value := range u.values {
				text, e := p.render(value, nil)
				if e != nil {
					return e
				}
				if e = p.write(" " + text); e != nil {
					return e
				}
			}
			if e = p.write(")\n"); e != nil {
				return e
			}
		}
	}
	for _, decl := range md.ConstantDeclarations {
		name, e := p.rawName(decl)
		if e != nil {
			return e
		}
		value, e := md.ConstantInterpretation(decl)
		if e != nil {
			return e
		}
		text, e := p.render(value, nil)
		if e != nil {
			return e
		}
		if e = p.write("(define " + name + " " + text + ")\n"); e != nil {
			return e
		}
	}
	for _, decl := range md.functionDeclarations {
		name, e := p.rawName(decl)
		if e != nil {
			return e
		}
		fi, e := md.FunctionInterpretation(decl)
		if e != nil || fi == nil {
			if e != nil {
				return e
			}
			return ast.ErrASTPending
		}
		vars, exprNames, e := p.functionNames(fi.arity, 0)
		if e != nil {
			return e
		}
		if e = p.write("(define (" + name); e != nil {
			return e
		}
		for _, variable := range vars {
			if e = p.write(" " + variable); e != nil {
				return e
			}
		}
		if e = p.write(")\n"); e != nil {
			return e
		}
		for _, entry := range fi.entries {
			if e = p.write("  (if "); e != nil {
				return e
			}
			if fi.arity > 1 {
				if e = p.write("(and "); e != nil {
					return e
				}
			}
			for j, variable := range vars {
				arg, x := entry.Argument(uint32(j))
				if x != nil {
					return x
				}
				text, x := p.render(arg, nil)
				if x != nil {
					return x
				}
				if e = p.write("(= " + variable + " " + text + ")"); e != nil {
					return e
				}
				if j+1 < len(vars) {
					if e = p.write(" "); e != nil {
						return e
					}
				}
			}
			if fi.arity > 1 {
				if e = p.write(")"); e != nil {
					return e
				}
			}
			result, x := entry.Result()
			if x != nil {
				return x
			}
			text, x := p.render(result, nil)
			if x != nil {
				return x
			}
			if e = p.write(" " + text + "\n"); e != nil {
				return e
			}
		}
		if len(fi.entries) > 0 {
			if e = p.write("  "); e != nil {
				return e
			}
		}
		if fi.fallback.IsNull() {
			if e = p.write("  #unspecified"); e != nil {
				return e
			}
		} else {
			text, x := p.render(fi.fallback, exprNames)
			if x != nil {
				return x
			}
			if e = p.write("  " + text); e != nil {
				return e
			}
		}
		if e = p.write(strings.Repeat(")", len(fi.entries)) + ")\n"); e != nil {
			return e
		}
	}
	return nil
}

func (p *modelPrinter) functionNames(arity uint32, first uint32) ([]string, []usymbol.Symbol, error) {
	vars := make([]string, arity)
	names := make([]usymbol.Symbol, arity)
	for i := uint32(0); i < arity; i++ {
		vars[i] = "x!" + strconv.FormatUint(uint64(i+first), 10)
		s, err := p.model.manager.Symbols.Intern(vars[i])
		if err != nil {
			return nil, nil, err
		}
		names[arity-i-1] = s
	}
	return vars, names, nil
}

func (md *ModelCore) sortedFunctions() ([]ast.ASTHandle, error) {
	visited := make(map[ast.ASTHandle]bool)
	result := make([]ast.ASTHandle, 0, len(md.functionDeclarations))
	for _, root := range md.functionDeclarations {
		if visited[root] {
			continue
		}
		visited[root] = true
		stack := []ast.ASTHandle{root}
		for len(stack) != 0 {
			current := stack[len(stack)-1]
			fi, err := md.FunctionInterpretation(current)
			if err != nil || fi == nil {
				if err != nil {
					return nil, err
				}
				return nil, ast.ErrASTPending
			}
			added := false
			if !fi.fallback.IsNull() {
				deps := make(ast.FuncDeclSet)
				if err = md.manager.CollectFuncDecls(fi.fallback, deps, false); err != nil {
					return nil, err
				}
				// Native map order is deliberately not observable. Visit model
				// declarations in registration order while preserving the source
				// dependency-before-dependent postorder.
				for _, candidate := range md.functionDeclarations {
					if _, found := deps[candidate]; found && !visited[candidate] {
						visited[candidate] = true
						stack = append(stack, candidate)
						added = true
					}
				}
			}
			if added {
				continue
			}
			stack = stack[:len(stack)-1]
			result = append(result, current)
		}
	}
	return result, nil
}

// WriteSMT2 implements model_smt2_pp, including universe declarations and
// cardinality constraints, constants, and dependency-ordered functions.
func (md *ModelCore) WriteSMT2(out io.Writer, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, indent uint32) error {
	p, err := newModelPrinter(md, out, context, globals)
	if err != nil {
		return err
	}
	if md.Derived != nil {
		for _, sort := range md.Derived.sorts {
			u, found := md.Derived.Universes.Get2(sort)
			if !found || u == nil || len(u.values) == 0 {
				return ast.ErrASTDomain
			}
			sortText, e := p.renderSort(sort)
			if e != nil {
				return e
			}
			names := make([]string, len(u.values))
			for i, value := range u.values {
				decl, x := md.manager.GetDecl(value)
				if x != nil {
					return x
				}
				names[i], x = p.smt2Name(decl)
				if x != nil {
					return x
				}
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";; universe for " + sortText + ":\n"); e != nil {
				return e
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";;   " + strings.Join(names, " ") + " \n"); e != nil {
				return e
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";; -----------\n"); e != nil {
				return e
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";; definitions for universe elements:\n"); e != nil {
				return e
			}
			for _, name := range names {
				if e = p.indent(indent); e != nil {
					return e
				}
				if e = p.write("(declare-fun " + name + " () " + sortText + ")\n"); e != nil {
					return e
				}
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";; cardinality constraint:\n"); e != nil {
				return e
			}
			conds := make([]string, len(names))
			for i, name := range names {
				conds[i] = "(= x " + name + ")"
			}
			condition := conds[0]
			if len(conds) > 1 {
				condition = "(or " + strings.Join(conds, " ") + ")"
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write("(forall ((x " + sortText + ")) " + condition + ")\n"); e != nil {
				return e
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";; -----------\n"); e != nil {
				return e
			}
		}
	}
	for _, decl := range md.ConstantDeclarations {
		name, e := p.smt2Name(decl)
		if e != nil {
			return e
		}
		rangeSort, e := md.manager.GetRange(decl)
		if e != nil {
			return e
		}
		sortText, e := p.renderSort(rangeSort)
		if e != nil {
			return e
		}
		value, e := md.ConstantInterpretation(decl)
		if e != nil {
			return e
		}
		valueText, e := p.render(value, nil)
		if e != nil {
			return e
		}
		if e = p.indent(indent); e != nil {
			return e
		}
		if e = p.write("(define-fun " + name + " () " + sortText + "\n"); e != nil {
			return e
		}
		if e = p.indent(indent + 2); e != nil {
			return e
		}
		if e = p.write(valueText + ")\n"); e != nil {
			return e
		}
	}
	functions, err := md.sortedFunctions()
	if err != nil {
		return err
	}
	for _, decl := range functions {
		name, e := p.smt2Name(decl)
		if e != nil {
			return e
		}
		fi, e := md.FunctionInterpretation(decl)
		if e != nil || fi == nil {
			if e != nil {
				return e
			}
			return ast.ErrASTPending
		}
		vars, exprNames, e := p.functionNames(fi.arity, 0)
		if e != nil {
			return e
		}
		args := make([]string, fi.arity)
		for i := uint32(0); i < fi.arity; i++ {
			domain, x := md.manager.GetDomain(decl, i)
			if x != nil {
				return x
			}
			domainText, x := p.renderSort(domain)
			if x != nil {
				return x
			}
			args[i] = "(" + vars[i] + " " + domainText + ")"
		}
		rangeSort, e := md.manager.GetRange(decl)
		if e != nil {
			return e
		}
		rangeText, e := p.renderSort(rangeSort)
		if e != nil {
			return e
		}
		body := "#unspecified"
		if !fi.fallback.IsNull() {
			body, e = p.render(fi.fallback, exprNames)
			if e != nil {
				return e
			}
		}
		for i := len(fi.entries) - 1; i >= 0; i-- {
			entry := fi.entries[i]
			conds := make([]string, fi.arity)
			for j := uint32(0); j < fi.arity; j++ {
				arg, x := entry.Argument(j)
				if x != nil {
					return x
				}
				argText, x := p.render(arg, nil)
				if x != nil {
					return x
				}
				conds[j] = "(= " + vars[j] + " " + argText + ")"
			}
			condition := conds[0]
			if len(conds) > 1 {
				condition = "(and " + strings.Join(conds, " ") + ")"
			}
			result, x := entry.Result()
			if x != nil {
				return x
			}
			resultText, x := p.render(result, nil)
			if x != nil {
				return x
			}
			body = "(ite " + condition + " " + resultText + " " + body + ")"
		}
		if e = p.indent(indent); e != nil {
			return e
		}
		if e = p.write("(define-fun " + name + " (" + strings.Join(args, " ") + ") " + rangeText + "\n"); e != nil {
			return e
		}
		if e = p.indent(indent + 2); e != nil {
			return e
		}
		if e = p.write(body + ")\n"); e != nil {
			return e
		}
	}
	return nil
}
