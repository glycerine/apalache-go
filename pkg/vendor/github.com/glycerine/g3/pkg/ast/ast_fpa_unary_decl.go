package ast

// Source: src/ast/fpa_decl_plugin.cpp:333-350,738-742.

import "github.com/glycerine/g3/pkg/util/z3exception"

const FPAOpNeg int32 = astFPANeg
const FPAOpAbs int32 = astFPAAbs

func (m *ASTManager) floatingPointUnaryDecl(kind int32, domain []ASTHandle) (astNodeRef, error) {
	if len(domain) != 1 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("invalid number of arguments to floating point operator")}
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
	var name string
	switch kind {
	case astFPANeg:
		name = "fp.neg"
	case astFPAAbs:
		name = "fp.abs"
	default:
		return astNodeRef{}, ErrASTDomain
	}
	sym, err := m.Symbols.Intern(name)
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(sym, m.floatingPointPlugin.family, kind, nil, domain, sort, ASTFuncFlags{}, false)
}
