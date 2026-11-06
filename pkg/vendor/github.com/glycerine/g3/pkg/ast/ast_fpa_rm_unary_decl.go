package ast

// Source: src/ast/fpa_decl_plugin.cpp:393-412.

import "github.com/glycerine/g3/pkg/util/z3exception"

const FPAOpSqrt int32 = astFPASqrt
const FPAOpRoundToIntegral int32 = astFPARoundToIntegral

func (m *ASTManager) floatingPointRMUnaryDecl(kind int32, domain []ASTHandle) (astNodeRef, error) {
	if len(domain) != 2 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("invalid number of arguments to floating point operator")}
	}
	rm, err := m.internal(domain[0])
	if err != nil {
		return astNodeRef{}, err
	}
	value, err := m.internal(domain[1])
	if err != nil {
		return astNodeRef{}, err
	}
	isSortOf := func(r astNodeRef, kind int32) (bool, error) {
		if r.slot == 0 || m.nodeAt(r.slot).kind != ASTSort {
			return false, nil
		}
		info, e := m.sortInfo(m.sortAt(r.slot).info)
		if e != nil {
			return false, e
		}
		return info.family == m.floatingPointPlugin.family && info.kind == kind, nil
	}
	rmValid, err := isSortOf(rm, 1)
	if err != nil {
		return astNodeRef{}, err
	}
	if !rmValid {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected RoundingMode as first argument")}
	}
	floatValid, err := isSortOf(value, 0)
	if err != nil {
		return astNodeRef{}, err
	}
	if !floatValid {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected FloatingPoint as second argument")}
	}
	var name string
	switch kind {
	case astFPASqrt:
		name = "fp.sqrt"
	case astFPARoundToIntegral:
		name = "fp.roundToIntegral"
	default:
		return astNodeRef{}, ErrASTDomain
	}
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(symbol, m.floatingPointPlugin.family, kind, nil, domain, value, ASTFuncFlags{}, false)
}
