package ast

// Source: src/ast/ast_smt2_pp.cpp:34-73.

import (
	usmt2 "github.com/glycerine/g3/pkg/util/smt2"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// SMT2FdeclName constructs smt2_pp_environment::pp_fdecl_name(symbol).
// The returned format node is borrowed from this AST manager.
func (m *ASTManager) SMT2FdeclName(name usymbol.Symbol) (ASTHandle, uint32, error) {
	quoted, err := usmt2.IsSMT2QuotedSymbol(m.Symbols, name)
	if err != nil {
		return ASTHandle{}, 0, err
	}
	var spelling string
	if quoted {
		spelling, err = usmt2.MkSMT2QuotedSymbol(m.Symbols, name, nil)
	} else if name.IsNumerical() {
		spelling, err = m.Symbols.String(name)
	} else {
		var present bool
		spelling, present, err = m.Symbols.BareString(name)
		if err == nil && !present {
			spelling = "null"
		}
	}
	if err != nil {
		return ASTHandle{}, 0, err
	}
	if uint64(len(spelling)) > uint64(^uint32(0)) {
		return ASTHandle{}, 0, ErrASTDomain
	}
	format, err := m.FormatString(spelling)
	if err != nil {
		return ASTHandle{}, 0, err
	}
	return format, uint32(len(spelling)), nil
}

// SMT2FdeclNameDecl follows the source's three built-in spellings before the
// ordinary declaration-symbol route.
func (m *ASTManager) SMT2FdeclNameDecl(decl ASTHandle) (ASTHandle, uint32, error) {
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return ASTHandle{}, 0, err
	}
	if family == m.BasicFamilyID() {
		kind, err := m.GetDeclKind(decl)
		if err != nil {
			return ASTHandle{}, 0, err
		}
		spelling := ""
		switch kind {
		case ASTOpImplies:
			spelling = "=>"
		case ASTOpIte:
			spelling = "ite"
		case ASTOpIff:
			spelling = "="
		}
		if spelling != "" {
			format, err := m.FormatString(spelling)
			return format, uint32(len(spelling)), err
		}
	}
	name, err := m.GetASTName(decl)
	if err != nil {
		return ASTHandle{}, 0, err
	}
	return m.SMT2FdeclName(name)
}
