package ast

// Source: src/ast/fpa_decl_plugin.cpp:567-591.

import "github.com/glycerine/g3/pkg/util/z3exception"

const FPAOpToFPUnsigned int32 = astFPAToFPUnsigned

func (m *ASTManager) floatingPointToFPUnsignedDecl(ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	fail := func(message string) (astNodeRef, error) {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException(message)}
	}
	if len(domain) != 2 {
		return fail("invalid number of arguments to to_fp_unsigned")
	}
	for i := range domain {
		ref, err := m.internal(domain[i])
		if err != nil {
			return astNodeRef{}, err
		}
		valid := false
		if ref.slot != 0 && m.nodeAt(ref.slot).kind == ASTSort {
			info, err := m.sortInfo(m.sortAt(ref.slot).info)
			if err != nil {
				return astNodeRef{}, err
			}
			if i == 0 {
				valid = info.family == m.floatingPointPlugin.family && info.kind == 1
			} else {
				valid = info.family == m.bitVectorPlugin.family && info.kind == 0
			}
		}
		if !valid {
			if i == 0 {
				return fail("sort mismatch, expected first argument of RoundingMode sort")
			}
			return fail("sort mismatch, expected second argument of bit-vector sort")
		}
	}
	if len(ps) != 2 {
		return fail("invalid number of parameters to to_fp_unsigned")
	}
	eParam, eErr := m.parameterOfKind(ps[0], ASTParameterInt)
	sParam, sErr := m.parameterOfKind(ps[1], ASTParameterInt)
	if eErr != nil || sErr != nil {
		return fail("invalid parameter type to to_fp_unsigned")
	}
	fpSort, err := m.mkFloatSort(uint32(eParam.integer), uint32(sParam.integer))
	if err != nil {
		return astNodeRef{}, err
	}
	name, err := m.Symbols.Intern("to_fp_unsigned")
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(name, m.floatingPointPlugin.family, astFPAToFPUnsigned, ps, domain, fpSort, ASTFuncFlags{}, false)
}
