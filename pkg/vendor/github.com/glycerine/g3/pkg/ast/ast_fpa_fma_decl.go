package ast

// Source: src/ast/fpa_decl_plugin.cpp:414-428.

import "github.com/glycerine/g3/pkg/util/z3exception"

const FPAOpFMA int32 = astFPAFMA

func (m *ASTManager) floatingPointFMADecl(domain []ASTHandle) (astNodeRef, error) {
	if len(domain) != 4 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("invalid number of arguments to fused_ma operator")}
	}
	var sorts [4]astNodeRef
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
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected RoundingMode as first argument")}
	}
	floatValid, err := isSortOf(sorts[1], 0)
	if err != nil {
		return astNodeRef{}, err
	}
	if sorts[1] != sorts[2] || sorts[1] != sorts[3] || !floatValid {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected arguments 1,2,3 of equal FloatingPoint sort")}
	}
	name, err := m.Symbols.Intern("fp.fma")
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(name, m.floatingPointPlugin.family, astFPAFMA, nil, domain, sorts[1], ASTFuncFlags{}, false)
}
