package ast

// Source: src/ast/fpa_decl_plugin.cpp:661-674.

import "github.com/glycerine/g3/pkg/util/z3exception"

const FPAOpToIEEEBV int32 = astFPAToIEEEBV

func (m *ASTManager) floatingPointToIEEEBVDecl(domain []ASTHandle) (astNodeRef, error) {
	if len(domain) != 1 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("invalid number of arguments to fp.to_ieee_bv")}
	}
	sort, err := m.internal(domain[0])
	if err != nil {
		return astNodeRef{}, err
	}
	valid := sort.slot != 0 && m.nodeAt(sort.slot).kind == ASTSort
	var info *astInfoRecord
	if valid {
		info, err = m.sortInfo(m.sortAt(sort.slot).info)
		valid = err == nil && info.family == m.floatingPointPlugin.family && info.kind == 0
	}
	if err != nil {
		return astNodeRef{}, err
	}
	if !valid {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected argument of FloatingPoint sort")}
	}
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
	width := uint32(eWidth.integer) + uint32(sWidth.integer)
	bvSort, err := m.bitVectorPlugin.mkSort(width)
	if err != nil {
		return astNodeRef{}, err
	}
	name, err := m.Symbols.Intern("fp.to_ieee_bv")
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(name, m.floatingPointPlugin.family, astFPAToIEEEBV, nil, domain, bvSort, ASTFuncFlags{}, false)
}
