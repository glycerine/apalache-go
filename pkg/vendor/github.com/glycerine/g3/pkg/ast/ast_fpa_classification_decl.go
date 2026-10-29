package ast

// Source: src/ast/fpa_decl_plugin.cpp:313-331,731-738.

import (
	"math"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

// floatingPointClassificationDecl is the concrete unary relation declaration
// used by the FPA classification reducers. The source ignores parameters and
// the requested range, but validates the one argument's floating-point sort.
func (m *ASTManager) floatingPointClassificationDecl(k int32, domain []ASTHandle) (astNodeRef, error) {
	if len(domain) != 1 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("invalid number of arguments to floating point relation")}
	}
	sort, err := m.internal(domain[0])
	if err != nil {
		return astNodeRef{}, err
	}
	if sort.slot == 0 || m.nodeAt(sort.slot).kind != ASTSort {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected argument of FloatingPoint sort")}
	}
	info, err := m.sortInfo(m.sortAt(sort.slot).info)
	if err != nil {
		return astNodeRef{}, err
	}
	if info.family != m.floatingPointPlugin.family || info.kind != 0 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("sort mismatch, expected argument of FloatingPoint sort")}
	}
	var name string
	switch k {
	case astFPAIsZero:
		name = "fp.isZero"
	case astFPAIsNegative:
		name = "fp.isNegative"
	case astFPAIsPositive:
		name = "fp.isPositive"
	case astFPAIsNaN:
		name = "fp.isNaN"
	case astFPAIsInf:
		name = "fp.isInfinite"
	case astFPAIsNormal:
		name = "fp.isNormal"
	case astFPAIsSubnormal:
		name = "fp.isSubnormal"
	default:
		return astNodeRef{}, ErrASTDomain
	}
	sym, err := m.Symbols.Intern(name)
	if err != nil {
		return astNodeRef{}, err
	}
	if err = m.beginFormatInfo(false, m.floatingPointPlugin.family, k, nil); err != nil {
		return astNodeRef{}, err
	}
	defer m.endFormatInfo()
	return m.mkFuncDecl(sym, domain, m.external(m.boolSort), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
}
