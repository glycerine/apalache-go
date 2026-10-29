package ast

// Source: src/ast/bv_decl_plugin.cpp:214-248,395-413,470-477,523-529,584-589.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"math"
)

func (m *ASTManager) int2BVSize(ps []ASTParameter) (width int32, found bool, result error) {
	if len(ps) != 1 {
		return 0, false, bvDeclarationError("int2bv expects one parameter")
	}
	value, e := m.parameter(ps[0])
	if e != nil {
		return 0, false, e
	}
	// Source parameter p(parameters[0]) copies an owned rational even though
	// that kind will subsequently be rejected. Other kinds are scalar copies.
	if value.Kind == ASTParameterRational {
		if e := m.copyInt2BVParameterRational(&value.Rational); e != nil {
			return 0, false, e
		}
	}

	if value.Kind == ASTParameterInt {
		return value.integer, true, nil
	}
	if value.Kind != ASTParameterAST {
		return 0, false, bvDeclarationError("int2bv expects one integer parameter")
	}
	if e = m.validNode(value.ast); e != nil {
		return 0, false, e
	}
	switch m.nodeAt(value.ast.slot).kind {
	case ASTApp, ASTVar, ASTQuantifier:
	default:
		return 0, false, bvDeclarationError("int2bv expects one integer parameter")
	}
	sort, e := m.sortOf(value.ast)
	if e != nil {
		return 0, false, e
	}
	return m.bitVectorSortWidth(m.bitVectorPlugin.family, m.external(sort))
}

func (p *astBitVectorPluginOwner) conversionDecl(k int32, width uint32, ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	m := p.owner
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	if k == 57 && width == 0 {
		return astNodeRef{}, bvDeclarationError("bit-vector size must be greater than zero")
	}
	if width == math.MaxUint32 {
		return astNodeRef{}, ErrASTDomain
	} // source width+1/index overflow
	if p.fixedPending.slot != 0 {
		return astNodeRef{}, ErrASTPending
	}
	v := &p.fixed[k]
	if uint64(len(*v)) < uint64(width+1) {
		*v = append(*v, make([]astNodeRef, int(width+1)-len(*v))...)
	}
	if len(domain) != 1 {
		message := "expecting one argument to int2bv"
		if k == 58 {
			message = "expecting one argument to bv2int"
		}
		return astNodeRef{}, bvDeclarationError(message)
	}
	r := (*v)[width]
	if r.slot != 0 {
		return r, nil
	}
	var e error
	result := p.fields[4]
	name := "bv2int"
	if k == 57 {
		name = "int2bv"
		result, e = p.getSort(width)
		if e != nil {
			return astNodeRef{}, e
		}
	} else {
		ps = nil
	}
	symbol, e := m.Symbols.Intern(name)
	if e != nil {
		return astNodeRef{}, e
	}
	r, e = m.pluginDecl(symbol, p.family, k, ps, domain, result, ASTFuncFlags{}, false)
	if e != nil {
		return astNodeRef{}, e
	}
	return r, p.publishFixed(k, width, r)
}

// A successful parameter copy/destruction ends before the parser's type error.
// Its numerical owner remains pending only if that actual copy/cleanup fails.
func (m *ASTManager) copyInt2BVParameterRational(value *unumeral.OwnedRational) (result error) {
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return e
	}
	defer f.finishNumeral(&result, nil)
	return f.copyConstruct(0, value)
}
