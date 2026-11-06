package ast

// Source: src/ast/fpa_decl_plugin.cpp:423-517,531-564.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

const FPAOpToFP int32 = astFPAToFP

func (m *ASTManager) floatingPointToFPDecl(ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	if len(domain) == 3 {
		allBV := true
		for _, sort := range domain {
			ref, err := m.internal(sort)
			if err != nil {
				return astNodeRef{}, err
			}
			if ref.slot == 0 || m.nodeAt(ref.slot).kind != ASTSort {
				return astNodeRef{}, ErrASTDomain
			}
			isBV, err := m.bitVectorSortRecognized(ref)
			if err != nil {
				return astNodeRef{}, err
			}
			allBV = allBV && isBV
		}
		if allBV {
			return m.floatingPointToFPThreeBVDecl(domain)
		}
		return m.floatingPointToFPThreeArithmeticDecl(ps, domain)
	}
	if len(domain) == 1 {
		ref, err := m.internal(domain[0])
		if err != nil {
			return astNodeRef{}, err
		}
		if ref.slot == 0 || m.nodeAt(ref.slot).kind != ASTSort {
			return astNodeRef{}, ErrASTDomain
		}
		isBV, err := m.bitVectorSortRecognized(ref)
		if err != nil {
			return astNodeRef{}, err
		}
		if !isBV {
			return astNodeRef{}, ErrASTDomain
		}
		fail := func(message string) (astNodeRef, error) {
			return astNodeRef{}, ASTException{z3exception.NewDefaultException(message)}
		}
		if len(ps) != 2 {
			return fail("invalid number of parameters to to_fp")
		}
		eParam, eErr := m.parameterOfKind(ps[0], ASTParameterInt)
		sParam, sErr := m.parameterOfKind(ps[1], ASTParameterInt)
		if eErr != nil || sErr != nil {
			return fail("invalid parameter type to to_fp")
		}
		width, err := m.bitVectorSortSize(ref, nil)
		if err != nil {
			return astNodeRef{}, err
		}
		if width != uint32(eParam.integer+sParam.integer) {
			return fail("sort mismatch; invalid bit-vector size, expected bitvector of size (ebits+sbits)")
		}
		fpSort, err := m.mkFloatSort(uint32(eParam.integer), uint32(sParam.integer))
		if err != nil {
			return astNodeRef{}, err
		}
		name, err := m.Symbols.Intern("to_fp")
		if err != nil {
			return astNodeRef{}, err
		}
		return m.pluginDecl(name, m.floatingPointPlugin.family, astFPAToFP, ps, domain, fpSort, ASTFuncFlags{}, false)
	}
	if len(domain) != 2 {
		return astNodeRef{}, ErrASTDomain
	}
	arithmetic := false
	for i := range domain {
		ref, err := m.internal(domain[i])
		if err != nil {
			return astNodeRef{}, err
		}
		if ref.slot == 0 || m.nodeAt(ref.slot).kind != ASTSort {
			return astNodeRef{}, ErrASTDomain
		}
		info, err := m.sortInfo(m.sortAt(ref.slot).info)
		if err != nil {
			return astNodeRef{}, err
		}
		if i == 1 {
			arithmetic = ref == m.floatingPointPlugin.real || ref == m.floatingPointPlugin.integer
		}
		if i == 0 && (info.family != m.floatingPointPlugin.family || info.kind != 1) ||
			i == 1 && !arithmetic && !((info.family == m.bitVectorPlugin.family || info.family == m.floatingPointPlugin.family) && info.kind == 0) {
			return astNodeRef{}, ErrASTDomain
		}
	}
	fail := func(message string) (astNodeRef, error) {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException(message)}
	}
	if len(ps) != 2 {
		if arithmetic {
			return fail("expecting two integer parameters to to_fp")
		}
		return fail("invalid number of parameters to to_fp")
	}
	eParam, eErr := m.parameterOfKind(ps[0], ASTParameterInt)
	sParam, sErr := m.parameterOfKind(ps[1], ASTParameterInt)
	if eErr != nil || sErr != nil {
		if arithmetic {
			return fail("expecting two integer parameters to to_fp")
		}
		return fail("invalid parameter type to to_fp")
	}
	fpSort, err := m.mkFloatSort(uint32(eParam.integer), uint32(sParam.integer))
	if err != nil {
		return astNodeRef{}, err
	}
	name, err := m.Symbols.Intern("to_fp")
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(name, m.floatingPointPlugin.family, astFPAToFP, ps, domain, fpSort, ASTFuncFlags{}, false)
}

func (m *ASTManager) floatingPointToFPThreeArithmeticDecl(ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	p := m.floatingPointPlugin
	first, err := m.internal(domain[0])
	if err != nil {
		return astNodeRef{}, err
	}
	second, err := m.internal(domain[1])
	if err != nil {
		return astNodeRef{}, err
	}
	third, err := m.internal(domain[2])
	if err != nil {
		return astNodeRef{}, err
	}
	if first.slot == 0 || second.slot == 0 || third.slot == 0 ||
		m.nodeAt(first.slot).kind != ASTSort || m.nodeAt(second.slot).kind != ASTSort || m.nodeAt(third.slot).kind != ASTSort {
		return astNodeRef{}, ErrASTDomain
	}
	firstInfo, err := m.sortInfo(m.sortAt(first.slot).info)
	if err != nil {
		return astNodeRef{}, err
	}
	if firstInfo.family != p.family || firstInfo.kind != 1 || !((second == p.real && third == p.integer) ||
		(second == p.integer && third == p.real)) {
		return astNodeRef{}, ErrASTDomain
	}
	fail := func() (astNodeRef, error) {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("expecting two integer parameters to to_fp")}
	}
	if len(ps) != 2 {
		return fail()
	}
	eParam, eErr := m.parameterOfKind(ps[0], ASTParameterInt)
	sParam, sErr := m.parameterOfKind(ps[1], ASTParameterInt)
	if eErr != nil || sErr != nil {
		return fail()
	}
	fpSort, err := m.mkFloatSort(uint32(eParam.integer), uint32(sParam.integer))
	if err != nil {
		return astNodeRef{}, err
	}
	name, err := m.Symbols.Intern("to_fp")
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(name, p.family, astFPAToFP, ps, domain, fpSort, ASTFuncFlags{}, false)
}

// The three-BV overload derives its format parameters from the exponent and
// significand domains; caller-supplied parameters are ignored by the source.
func (m *ASTManager) floatingPointToFPThreeBVDecl(domain []ASTHandle) (decl astNodeRef, result error) {
	var widths [3]uint32
	for i, sort := range domain {
		ref, err := m.internal(sort)
		if err != nil {
			return astNodeRef{}, err
		}
		if ref.slot == 0 || m.nodeAt(ref.slot).kind != ASTSort {
			return astNodeRef{}, ErrASTDomain
		}
		isBV, err := m.bitVectorSortRecognized(ref)
		if err != nil {
			return astNodeRef{}, err
		}
		if !isBV {
			return astNodeRef{}, ErrASTDomain
		}
		widths[i], err = m.bitVectorSortSize(ref, nil)
		if err != nil {
			return astNodeRef{}, err
		}
	}
	ebits, sbits := widths[1], widths[2]+1
	fpSort, err := m.mkFloatSort(ebits, sbits)
	if err != nil {
		return astNodeRef{}, err
	}
	name, err := m.Symbols.Intern("to_fp")
	if err != nil {
		return astNodeRef{}, err
	}
	eParam, err := m.newParameterInt(int32(ebits))
	if err != nil {
		return astNodeRef{}, err
	}
	defer func() { result = errors.Join(result, m.dropParameter(eParam)) }()
	sParam, err := m.newParameterInt(int32(sbits))
	if err != nil {
		return astNodeRef{}, err
	}
	defer func() { result = errors.Join(result, m.dropParameter(sParam)) }()
	return m.pluginDecl(name, m.floatingPointPlugin.family, astFPAToFP,
		[]ASTParameter{eParam, sParam}, domain, fpSort, ASTFuncFlags{}, false)
}
