package ast

// Source: src/ast/fpa_decl_plugin.cpp:352-373.

import "github.com/glycerine/g3/pkg/util/z3exception"

const FPAOpRem int32 = astFPARem
const FPAOpMin int32 = astFPAMin
const FPAOpMax int32 = astFPAMax

func (m *ASTManager) floatingPointBinaryDecl(kind int32, domain []ASTHandle) (astNodeRef, error) {
	if len(domain) != 2 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("invalid number of arguments to floating point operator")}
	}
	first, err := m.internal(domain[0])
	if err != nil {
		return astNodeRef{}, err
	}
	second, err := m.internal(domain[1])
	if err != nil {
		return astNodeRef{}, err
	}
	valid := first == second && first.slot != 0 && m.nodeAt(first.slot).kind == ASTSort
	if valid {
		var info *astInfoRecord
		info, err = m.sortInfo(m.sortAt(first.slot).info)
		valid = err == nil && info.family == m.floatingPointPlugin.family && info.kind == 0
	}
	if err != nil {
		return astNodeRef{}, err
	}
	if !valid {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected arguments of equal FloatingPoint sorts")}
	}
	var text string
	switch kind {
	case astFPARem:
		text = "fp.rem"
	case astFPAMin:
		text = "fp.min"
	case astFPAMax:
		text = "fp.max"
	default:
		return astNodeRef{}, ErrASTDomain
	}
	name, err := m.Symbols.Intern(text)
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(name, m.floatingPointPlugin.family, kind, nil, domain, first, ASTFuncFlags{}, false)
}
