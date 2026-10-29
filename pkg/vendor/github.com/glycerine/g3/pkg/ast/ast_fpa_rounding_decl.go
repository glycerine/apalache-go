package ast

// Source: src/ast/fpa_decl_plugin.cpp:230-253,710-724.

import (
	"math"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

// Public rounding-mode declaration kinds.
const (
	FPAOpRMNearestEven    int32 = astFPARMNearestEven
	FPAOpRMNearestAway    int32 = astFPARMNearestAway
	FPAOpRMTowardPositive int32 = astFPARMTowardPositive
	FPAOpRMTowardNegative int32 = astFPARMTowardNegative
	FPAOpRMTowardZero     int32 = astFPARMTowardZero
)

func (m *ASTManager) floatingPointRoundingDecl(k int32, ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	if len(ps) != 0 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("rounding mode constant does not have parameters")}
	}
	if len(domain) != 0 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("rounding mode is a constant")}
	}
	sort, e := m.floatingPointSort(1, nil)
	if e != nil {
		return astNodeRef{}, e
	}
	var name string
	switch k {
	case astFPARMNearestEven:
		name = "roundNearestTiesToEven"
	case astFPARMNearestAway:
		name = "roundNearestTiesToAway"
	case astFPARMTowardPositive:
		name = "roundTowardPositive"
	case astFPARMTowardNegative:
		name = "roundTowardNegative"
	case astFPARMTowardZero:
		name = "roundTowardZero"
	default:
		return astNodeRef{}, ErrASTDomain
	}
	symbol, e := m.Symbols.Intern(name)
	if e != nil {
		return astNodeRef{}, e
	}
	if e = m.beginFormatInfo(false, m.floatingPointPlugin.family, k, nil); e != nil {
		return astNodeRef{}, e
	}
	defer m.endFormatInfo()
	return m.mkFuncDecl(symbol, nil, m.external(sort), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
}
