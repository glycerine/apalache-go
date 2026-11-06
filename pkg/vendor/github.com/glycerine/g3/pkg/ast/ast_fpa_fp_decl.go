package ast

// Source: src/ast/fpa_decl_plugin.cpp:591-606.

// FPAOpFP constructs a floating-point value from separate sign, exponent,
// and significand bit-vectors. Its format follows the latter two widths.
const FPAOpFP int32 = astFPAFP

func (m *ASTManager) floatingPointFPDecl(domain []ASTHandle) (astNodeRef, error) {
	if len(domain) != 3 {
		return astNodeRef{}, ErrASTDomain
	}
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
	if widths[0] != 1 {
		return astNodeRef{}, ErrASTDomain
	}
	fpSort, err := m.mkFloatSort(widths[1], widths[2]+1)
	if err != nil {
		return astNodeRef{}, err
	}
	name, err := m.Symbols.Intern("fp")
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(name, m.floatingPointPlugin.family, astFPAFP, nil, domain, fpSort, ASTFuncFlags{}, false)
}
