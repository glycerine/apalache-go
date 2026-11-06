package ast

// Source: src/ast/fpa_decl_plugin.cpp:675-686.

import "github.com/glycerine/g3/pkg/util/z3exception"

const FPAOpBV2RM int32 = astFPABV2RM

func (m *ASTManager) floatingPointBV2RMDecl(ps []ASTParameter, domain []ASTHandle, rangeSort ASTHandle) (astNodeRef, error) {
	fail := func(message string) (astNodeRef, error) {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException(message)}
	}
	if len(domain) != 1 {
		return fail("invalid number of arguments to bv2rm")
	}
	arg, err := m.internal(domain[0])
	if err != nil {
		return astNodeRef{}, err
	}
	valid := false
	if arg.slot != 0 && m.nodeAt(arg.slot).kind == ASTSort {
		info, e := m.sortInfo(m.sortAt(arg.slot).info)
		if e != nil {
			return astNodeRef{}, e
		}
		if info.family == m.bitVectorPlugin.family && info.kind == 0 {
			param, e := m.infoParameter(info, 0)
			if e != nil {
				return astNodeRef{}, e
			}
			width, e := m.parameterOfKind(param, ASTParameterInt)
			if e != nil {
				return astNodeRef{}, e
			}
			valid = width.integer == 3
		}
	}
	if !valid {
		return fail("sort mismatch, expected argument of sort bitvector, size 3")
	}
	var result astNodeRef
	if rangeSort.IsNull() {
		result, err = m.floatingPointSort(1, nil)
	} else {
		result, err = m.internal(rangeSort)
	}
	if err != nil {
		return astNodeRef{}, err
	}
	valid = false
	if result.slot != 0 && m.nodeAt(result.slot).kind == ASTSort {
		info, e := m.sortInfo(m.sortAt(result.slot).info)
		if e != nil {
			return astNodeRef{}, e
		}
		valid = info.family == m.floatingPointPlugin.family && info.kind == 1
	}
	if !valid {
		return fail("sort mismatch, expected range of RoundingMode sort")
	}
	name, err := m.Symbols.Intern("rm")
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(name, m.floatingPointPlugin.family, astFPABV2RM, ps, domain, result, ASTFuncFlags{}, false)
}
