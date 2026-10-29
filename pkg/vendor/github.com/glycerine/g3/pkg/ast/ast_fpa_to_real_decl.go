package ast

// Source: src/ast/fpa_decl_plugin.cpp:650-659.

import "github.com/glycerine/g3/pkg/util/z3exception"

const FPAOpToReal int32 = astFPAToReal

func (m *ASTManager) floatingPointToRealDecl(domain []ASTHandle) (astNodeRef, error) {
	if len(domain) != 1 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("invalid number of arguments to fp.to_real")}
	}
	sort, err := m.internal(domain[0])
	if err != nil {
		return astNodeRef{}, err
	}
	valid := sort.slot != 0 && m.nodeAt(sort.slot).kind == ASTSort
	if valid {
		var info *astInfoRecord
		info, err = m.sortInfo(m.sortAt(sort.slot).info)
		valid = err == nil && info.family == m.floatingPointPlugin.family && info.kind == 0
	}
	if err != nil {
		return astNodeRef{}, err
	}
	if !valid {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected argument of FloatingPoint sort")}
	}
	name, err := m.Symbols.Intern("fp.to_real")
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(name, m.floatingPointPlugin.family, astFPAToReal, nil, domain, m.floatingPointPlugin.real, ASTFuncFlags{}, false)
}
