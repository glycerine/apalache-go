package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp mk_app_core shape helpers.

import "github.com/glycerine/g3/pkg/ast"

func blastTermITEArguments(m *ast.ASTManager, arguments ast.AppArguments) ([]ast.ASTHandle, error) {
	args := make([]ast.ASTHandle, arguments.Count())
	for i := range args {
		var err error
		args[i], err = arguments.At(uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return args, nil
}

func blastTermITEParts(m *ast.ASTManager, h ast.ASTHandle) (condition, thenPart, elsePart ast.ASTHandle, found bool, err error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return condition, thenPart, elsePart, false, err
	}
	decl, err := m.GetDecl(h)
	if err != nil {
		return condition, thenPart, elsePart, false, err
	}
	family, err := m.GetFamilyID(decl)
	if err != nil || family != m.BasicFamilyID() {
		return condition, thenPart, elsePart, false, err
	}
	op, err := m.GetDeclKind(decl)
	if err != nil || op != ast.ASTOpIte {
		return condition, thenPart, elsePart, false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil || n != 3 {
		return condition, thenPart, elsePart, false, err
	}
	condition, err = m.GetArg(h, 0)
	if err == nil {
		thenPart, err = m.GetArg(h, 1)
	}
	if err == nil {
		elsePart, err = m.GetArg(h, 2)
	}
	return condition, thenPart, elsePart, err == nil, err
}

func blastTermITEIsBool(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	sort, err := m.GetSort(h)
	if err != nil {
		return false, err
	}
	boolean, err := m.MkBoolSort()
	return sort == boolean, err
}
