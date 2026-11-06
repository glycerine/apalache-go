package ast

// Source: src/ast/fpa_decl_plugin.cpp:64-95; src/ast/ast.cpp:76-85.

import (
	"math"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// Source: src/ast/ast.h:70-73; src/ast/ast.cpp:1515-1517.
// Immutable Go exception text follows the existing default_exception adapter.
type ASTException struct{ z3exception.DefaultException }

const (
	astFPARMNearestEven    int32 = 0
	astFPARMNearestAway    int32 = 1
	astFPARMTowardPositive int32 = 2
	astFPARMTowardNegative int32 = 3
	astFPARMTowardZero     int32 = 4
	astFPANum              int32 = 5
	astFPAPlusInf          int32 = 6
	astFPAMinusInf         int32 = 7
	astFPANaN              int32 = 8
	astFPAPlusZero         int32 = 9
	astFPAMinusZero        int32 = 10
	astFPAAdd              int32 = 11
	astFPASub              int32 = 12
	astFPANeg              int32 = 13
	astFPAMul              int32 = 14
	astFPADiv              int32 = 15
	astFPARem              int32 = 16
	astFPAAbs              int32 = 17
	astFPAMin              int32 = 18
	astFPAMax              int32 = 19
	astFPAFMA              int32 = 20
	astFPASqrt             int32 = 21
	astFPARoundToIntegral  int32 = 22
	astFPAEq               int32 = 23
	astFPALt               int32 = 24
	astFPAGt               int32 = 25
	astFPALe               int32 = 26
	astFPAGe               int32 = 27
	astFPAIsNaN            int32 = 28
	astFPAIsInf            int32 = 29
	astFPAIsZero           int32 = 30
	astFPAIsNormal         int32 = 31
	astFPAIsSubnormal      int32 = 32
	astFPAIsNegative       int32 = 33
	astFPAIsPositive       int32 = 34
	astFPAFP               int32 = 35
	astFPAToFP             int32 = 36
	astFPAToFPUnsigned     int32 = 37
	astFPAToUBV            int32 = 38
	astFPAToSBV            int32 = 39
	astFPAToReal           int32 = 40
	astFPAToIEEEBV         int32 = 41
	astFPABVWrap           int32 = 42
	astFPABV2RM            int32 = 43
)

// Public numeral and special-value declaration kinds used by the FPA-to-BV
// converter and its traversal rewriter.
const (
	FPAOpNumeral      int32 = astFPANum
	FPAOpPositiveInf  int32 = astFPAPlusInf
	FPAOpNegativeInf  int32 = astFPAMinusInf
	FPAOpNaN          int32 = astFPANaN
	FPAOpPositiveZero int32 = astFPAPlusZero
	FPAOpNegativeZero int32 = astFPAMinusZero
)

func (p *astFloatingPointPluginOwner) numeralAssertionsEnabled() (bool, error) {
	if !ASTSourceDebug {
		return false, nil
	}
	if p.assertions == nil {
		return true, nil
	}
	return p.assertions.NamespaceAssertionsEnabled()
}
func (p *astFloatingPointPluginOwner) numeralAssertionFailure(line int32, condition string) error {
	if p.assertions == nil {
		return uparams.ErrGlobalParamsAssertion
	}
	return p.assertions.NamespaceAssertionFailure("src/ast/fpa_decl_plugin.cpp", line, condition)
}
func (p *astFloatingPointPluginOwner) mkNumeralDecl(value *unumeral.ArbitraryFloat, g *uparams.GlobalParamsService) (astNodeRef, error) {
	if p == nil || p.self != p || p.owner == nil || p.owner.floatingPointPlugin != p || !p.published || p.sourceEnded || value == nil {
		return astNodeRef{}, ErrASTHandle
	}
	if g != nil && (g.CheckBudget(p.owner.BudgetField) != nil) {
		return astNodeRef{}, uparams.ErrGlobalParamsOwner
	}
	if p.assertions != nil && p.assertions != g {
		return astNodeRef{}, ErrASTHandle
	}
	p.assertions = g
	m := p.owner
	sort, e := m.mkFloatSort(value.EBits(), value.SBits())
	if e != nil {
		return astNodeRef{}, e
	}
	kind, name := astFPANum, "fp.numeral"
	nan, e := p.numeric.IsNaN(value)
	if e != nil {
		return astNodeRef{}, e
	}
	if nan {
		kind, name = astFPANaN, "NaN"
	} else {
		positive, e := p.numeric.IsPositiveInf(value)
		if e != nil {
			return astNodeRef{}, e
		}
		if positive {
			kind, name = astFPAPlusInf, "+oo"
		} else {
			negative, e := p.numeric.IsNegativeInf(value)
			if e != nil {
				return astNodeRef{}, e
			}
			if negative {
				kind, name = astFPAMinusInf, "-oo"
			} else {
				positive, e = p.numeric.IsPositiveZero(value)
				if e != nil {
					return astNodeRef{}, e
				}
				if positive {
					kind, name = astFPAPlusZero, "+zero"
				} else {
					negative, e = p.numeric.IsNegativeZero(value)
					if e != nil {
						return astNodeRef{}, e
					}
					if negative {
						kind, name = astFPAMinusZero, "-zero"
					}
				}
			}
		}
	}
	var parameter *astParameterRecord
	var local astParameterRecord
	if kind == astFPANum {
		enabled, e := p.numeralAssertionsEnabled()
		if e != nil {
			return astNodeRef{}, e
		}
		if enabled {
			regular := value.SBits() == 0
			if !regular {
				regular, e = p.numeric.IsNormal(value)
				if e != nil {
					return astNodeRef{}, e
				}
			}
			if !regular {
				regular, e = p.numeric.IsDenormal(value)
				if e != nil {
					return astNodeRef{}, e
				}
			}
			if !regular {
				if e = p.numeralAssertionFailure(85, "m_fm.is_regular(v)"); e != nil {
					return astNodeRef{}, e
				}
			}
		}
		id, e := p.mkValueID(value)
		if e != nil {
			return astNodeRef{}, e
		}
		local = astParameterRecord{Kind: ASTParameterExternal, external: id}
		parameter = &local
		enabled, e = p.numeralAssertionsEnabled()
		if e != nil {
			return astNodeRef{}, e
		}
		if enabled && local.Kind != ASTParameterExternal {
			if e = p.numeralAssertionFailure(87, "p.is_external()"); e != nil {
				return astNodeRef{}, e
			}
		}
		sort, e = m.mkFloatSort(value.EBits(), value.SBits())
		if e != nil {
			return astNodeRef{}, e
		}
	}
	symbol, e := m.Symbols.Intern(name)
	if e != nil {
		return astNodeRef{}, e
	}
	if e = m.beginFormatInfo(false, p.family, kind, parameter); e != nil {
		return astNodeRef{}, e
	}
	defer m.endFormatInfo()
	return m.mkFuncDecl(symbol, nil, m.external(sort), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
}

func (p *astFloatingPointPluginOwner) recycledValueID(id uint32) error {
	enabled, e := p.numeralAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled {
		_, index, e := p.findValueID(id)
		if e != nil {
			return e
		}
		if index < 0 {
			if e = p.numeralAssertionFailure(65, "m_value_table.contains(id)"); e != nil {
				return e
			}
		}
	}
	if e = p.eraseValueID(id); e != nil {
		return e
	}
	if !p.outOfMemory() {
		p.freeIDs = append(p.freeIDs, id)
	}
	return p.values.del(id)
}

// Source: src/ast/fpa_decl_plugin.cpp:94-95. This reaches the same actual
// declaration registration and application constructor, without another clone.
func (p *astFloatingPointPluginOwner) mkNumeral(value *unumeral.ArbitraryFloat, g *uparams.GlobalParamsService) (astNodeRef, error) {
	decl, e := p.mkNumeralDecl(value, g)
	if e != nil {
		return astNodeRef{}, e
	}
	return p.owner.mkApp(decl, nil)
}

// Source: src/ast/fpa_decl_plugin.cpp:98-139. The source NUM branch copies
// the actual indexed vector value; special branches read both sort parameters
// only after their matching application test. Other live kinds leave out and
// the optional receipt unread/unmodified.
func (p *astFloatingPointPluginOwner) isNumeral(n astNodeRef, out *unumeral.ArbitraryFloat, receipt *unumeral.ArbitraryFloatValueCopyReceipt) (bool, error) {
	if p == nil || p.self != p || p.owner == nil || p.owner.floatingPointPlugin != p || !p.published || p.sourceEnded {
		return false, ErrASTHandle
	}
	m := p.owner
	if e := m.validNode(n); e != nil {
		return false, e
	}
	if m.nodeAt(n.slot).kind != ASTApp {
		return false, nil
	}
	decl := m.appAt(n.slot).decl
	fid, kind, e := m.declIdentity(decl)
	if e != nil {
		return false, e
	}
	if fid != p.family {
		return false, nil
	}
	if kind == astFPANum {
		info, e := m.funcInfo(m.declAt(decl.slot).info)
		if e != nil {
			return false, e
		}
		parameter, e := m.infoParameter(info, 0)
		if e != nil {
			return false, e
		}
		value, e := m.parameterOfKind(parameter, ASTParameterExternal)
		if e != nil {
			return false, e
		}
		source, e := p.values.at(value.external)
		if e != nil {
			return false, e
		}
		e = p.numeric.CopyWithReceipt(out, &source, receipt)
		return e == nil, e
	}
	if kind != astFPAMinusInf && kind != astFPAPlusInf && kind != astFPANaN && kind != astFPAPlusZero && kind != astFPAMinusZero {
		return false, nil
	}
	sort := m.declAt(decl.slot).rangeSort
	info, e := m.sortInfo(m.sortAt(sort.slot).info)
	if e != nil {
		return false, e
	}
	ebits, e := m.infoParameter(info, 0)
	if e != nil {
		return false, e
	}
	a, e := m.parameterOfKind(ebits, ASTParameterInt)
	if e != nil {
		return false, e
	}
	exponentBits := uint32(a.integer)
	sbits, e := m.infoParameter(info, 1)
	if e != nil {
		return false, e
	}
	b, e := m.parameterOfKind(sbits, ASTParameterInt)
	if e != nil {
		return false, e
	}
	significandBits := uint32(b.integer)
	switch kind {
	case astFPAMinusInf:
		e = p.numeric.MkInf(exponentBits, significandBits, true, out)
	case astFPAPlusInf:
		e = p.numeric.MkInf(exponentBits, significandBits, false, out)
	case astFPANaN:
		e = p.numeric.MkNaN(exponentBits, significandBits, out, receipt)
	case astFPAPlusZero:
		e = p.numeric.MkZero(exponentBits, significandBits, false, out)
	case astFPAMinusZero:
		e = p.numeric.MkZero(exponentBits, significandBits, true, out)
	}
	return e == nil, e
}
