package api

// Source: src/api/api_ast.cpp:819-851 and src/ast/ast_pp.h mk_pp alias.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (p *APIASTPrinter) smt2String(m *ast.ASTManager, value ast.ASTHandle) (string, error) {
	return p.smt2StringIndented(m, value, 0)
}

func (p *APIASTPrinter) smt2StringIndented(m *ast.ASTManager, value ast.ASTHandle, indent uint32) (text string, resultErr error) {
	globals, err := p.owner.command.GlobalParameters()
	if err != nil {
		return "", err
	}
	local, err := globals.Context.NewValue()
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, globals.Context.Del(&local)) }()
	if p.mode == APIASTPrintSMTFull {
		for _, key := range []string{"max_depth", "min_alias_size"} {
			if err := globals.Context.SetUIntName(&local, &key, ^uint32(0)); err != nil {
				return "", err
			}
		}
	}
	kind, err := m.GetKind(value)
	if err != nil {
		return "", err
	}
	var format ast.ASTHandle
	switch kind {
	case ast.ASTApp, ast.ASTVar, ast.ASTQuantifier:
		format, err = m.SMT2PPExprTreeWithParams(value, nil, globals.Context, globals, &local)
	case ast.ASTSort:
		format, err = m.SMT2PPSort(value)
	case ast.ASTFuncDecl:
		format, err = m.SMT2PPDeclareFun(value)
	default:
		return "", ast.ErrASTDomain
	}
	if err != nil {
		return "", err
	}
	ref, err := m.Format.NewRef(format)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, m.Format.DelRef(&ref)) }()
	if indent != 0 {
		format, err = m.FormatIndent(indent, format)
		if err != nil {
			return "", err
		}
		if err = m.Format.SetRef(ref, format); err != nil {
			return "", err
		}
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, stream.Close()) }()
	if err := m.PrintFormat(stream, format, globals.Context, &local, globals); err != nil {
		return "", err
	}
	return stream.String()
}
