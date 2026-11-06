package ast

// Source: src/ast/fpa_decl_plugin.cpp:689-710.

import "github.com/glycerine/g3/pkg/util/z3exception"

const FPAOpBVWrap int32 = astFPABVWrap

func (m *ASTManager) floatingPointBVWrapDecl(ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	if len(domain) != 1 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("invalid number of arguments to bv_wrap")}
	}
	sort, err := m.internal(domain[0])
	if err != nil {
		return astNodeRef{}, err
	}
	if sort.slot == 0 || m.nodeAt(sort.slot).kind != ASTSort {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected argument of FloatingPoint or RoundingMode sort")}
	}
	info, err := m.sortInfo(m.sortAt(sort.slot).info)
	if err != nil {
		return astNodeRef{}, err
	}
	if info.family != m.floatingPointPlugin.family || info.kind != 0 && info.kind != 1 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected argument of FloatingPoint or RoundingMode sort")}
	}
	width := uint32(3)
	if info.kind == 0 {
		eParam, err := m.infoParameter(info, 0)
		if err != nil {
			return astNodeRef{}, err
		}
		sParam, err := m.infoParameter(info, 1)
		if err != nil {
			return astNodeRef{}, err
		}
		eWidth, err := m.parameterOfKind(eParam, ASTParameterInt)
		if err != nil {
			return astNodeRef{}, err
		}
		sWidth, err := m.parameterOfKind(sParam, ASTParameterInt)
		if err != nil {
			return astNodeRef{}, err
		}
		width = uint32(eWidth.integer) + uint32(sWidth.integer)
	}
	bvSort, err := m.bitVectorPlugin.mkSort(width)
	if err != nil {
		return astNodeRef{}, err
	}
	name, err := m.Symbols.Intern("bv_wrap")
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(name, m.floatingPointPlugin.family, astFPABVWrap, ps, domain, bvSort, ASTFuncFlags{}, false)
}
