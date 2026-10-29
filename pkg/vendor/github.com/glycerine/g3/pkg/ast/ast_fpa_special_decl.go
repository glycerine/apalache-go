package ast

// Source: src/ast/fpa_decl_plugin.cpp:256-290,710-719.

import "github.com/glycerine/g3/pkg/util/z3exception"

// The source accepts a float sort parameter first, two integer widths second,
// and a float range third. It ignores arity and domain for these five kinds.
func (m *ASTManager) floatingPointSpecialDecl(k int32, ps []ASTParameter, rangeSort ASTHandle) (out astNodeRef, result error) {
	p := m.floatingPointPlugin
	if p == nil || !p.published || p.sourceEnded {
		return out, ErrASTHandle
	}
	floatSort := func(r astNodeRef) bool {
		if r.slot == 0 || m.validNode(r) != nil || m.nodeAt(r.slot).kind != ASTSort {
			return false
		}
		info, e := m.sortInfo(m.sortAt(r.slot).info)
		return e == nil && info.family == p.family && info.kind == 0
	}
	var sort astNodeRef
	if len(ps) == 1 {
		parameter, e := m.parameter(ps[0])
		if e != nil {
			return out, e
		}
		if parameter.Kind == ASTParameterAST && floatSort(parameter.ast) {
			sort = parameter.ast
		}
	}
	if sort.slot == 0 && len(ps) == 2 {
		e, first := m.parameter(ps[0])
		s, second := m.parameter(ps[1])
		if first != nil {
			return out, first
		}
		if second != nil {
			return out, second
		}
		if e.Kind == ASTParameterInt && s.Kind == ASTParameterInt {
			sort, result = m.mkFloatSort(uint32(e.integer), uint32(s.integer))
			if result != nil {
				return out, result
			}
		}
	}
	if sort.slot == 0 && rangeSort != (ASTHandle{}) {
		r, e := m.internal(rangeSort)
		if e != nil {
			return out, e
		}
		if floatSort(r) {
			sort = r
		}
	}
	if sort.slot == 0 {
		return out, ASTException{z3exception.NewDefaultException("sort of floating point constant was not specified")}
	}
	info, e := m.sortInfo(m.sortAt(sort.slot).info)
	if e != nil {
		return out, e
	}
	eParam, e := m.infoParameter(info, 0)
	if e != nil {
		return out, e
	}
	eWidth, e := m.parameterOfKind(eParam, ASTParameterInt)
	if e != nil {
		return out, e
	}
	sParam, e := m.infoParameter(info, 1)
	if e != nil {
		return out, e
	}
	sWidth, e := m.parameterOfKind(sParam, ASTParameterInt)
	if e != nil {
		return out, e
	}
	f, e := p.beginNumeralProbe()
	if e != nil {
		return out, e
	}
	defer p.finishNumeralConstruction(f, &result, &out)
	ebits, sbits := uint32(eWidth.integer), uint32(sWidth.integer)
	switch k {
	case astFPANaN:
		e = p.numeric.MkNaN(ebits, sbits, &f.value, &f.copy)
		if e == nil && ASTSourceDebug {
			var enabled bool
			enabled, e = p.numeralAssertionsEnabled()
			if e == nil && enabled {
				var nan bool
				nan, e = p.numeric.IsNaN(&f.value)
				if e == nil && !nan {
					e = p.numeralAssertionFailure(281, "m_fm.is_nan(val)")
				}
			}
		}
	case astFPAMinusInf:
		e = p.numeric.MkInf(ebits, sbits, true, &f.value)
	case astFPAPlusInf:
		e = p.numeric.MkInf(ebits, sbits, false, &f.value)
	case astFPAMinusZero:
		e = p.numeric.MkZero(ebits, sbits, true, &f.value)
	case astFPAPlusZero:
		e = p.numeric.MkZero(ebits, sbits, false, &f.value)
	default:
		return out, ErrASTDomain
	}
	if e != nil {
		return out, e
	}
	return p.mkNumeralDecl(&f.value, nil)
}
