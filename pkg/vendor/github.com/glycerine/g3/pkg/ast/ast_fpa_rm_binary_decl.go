package ast

// Source: src/ast/fpa_decl_plugin.cpp:374-392.

import "github.com/glycerine/g3/pkg/util/z3exception"

const FPAOpAdd int32 = astFPAAdd
const FPAOpSub int32 = astFPASub
const FPAOpMul int32 = astFPAMul
const FPAOpDiv int32 = astFPADiv

func (m *ASTManager) floatingPointRMBinaryDecl(kind int32, domain []ASTHandle) (astNodeRef, error) {
	if len(domain) != 3 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("invalid number of arguments to floating point operator")}
	}
	var sorts [3]astNodeRef
	for i, h := range domain {
		r, err := m.internal(h)
		if err != nil {
			return astNodeRef{}, err
		}
		sorts[i] = r
	}
	isSortOf := func(r astNodeRef, kind int32) (bool, error) {
		if r.slot == 0 || m.nodeAt(r.slot).kind != ASTSort {
			return false, nil
		}
		info, err := m.sortInfo(m.sortAt(r.slot).info)
		if err != nil {
			return false, err
		}
		return info.family == m.floatingPointPlugin.family && info.kind == kind, nil
	}
	rmValid, err := isSortOf(sorts[0], 1)
	if err != nil {
		return astNodeRef{}, err
	}
	if !rmValid {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected first argument of RoundingMode sort")}
	}
	floatValid, err := isSortOf(sorts[1], 0)
	if err != nil {
		return astNodeRef{}, err
	}
	if sorts[1] != sorts[2] || !floatValid {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected arguments 1 and 2 of equal FloatingPoint sorts")}
	}
	var name string
	switch kind {
	case astFPAAdd:
		name = "fp.add"
	case astFPASub:
		name = "fp.sub"
	case astFPAMul:
		name = "fp.mul"
	case astFPADiv:
		name = "fp.div"
	default:
		return astNodeRef{}, ErrASTDomain
	}
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(symbol, m.floatingPointPlugin.family, kind, nil, domain, sorts[1], ASTFuncFlags{}, false)
}
