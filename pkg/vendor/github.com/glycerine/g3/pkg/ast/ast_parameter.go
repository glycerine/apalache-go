package ast

// Source: src/ast/ast.cpp:36-130; src/ast/ast.h:156-242.
// Source parameter constructors, copy/move, destruction, equality and hash.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (m *ASTManager) parameter(p ASTParameter) (*astParameterRecord, error) {
	if p.origin == m.origin && p.slot == math.MaxUint32-2 {
		f := &m.bitVectorUtilityParameters
		if !f.active || p.generation < f.epoch*2 || p.generation-f.epoch*2 >= f.count {
			return nil, ErrASTHandle
		}
		return &f.values[p.generation-f.epoch*2], nil
	}
	if p.origin == m.origin && p.slot == math.MaxUint32-1 {
		if !m.formatLexical.active || p.generation != m.formatLexical.epoch*2 && (!m.formatLexical.localPair || p.generation != m.formatLexical.epoch*2+1) {
			return nil, ErrASTHandle
		}
		if m.formatLexical.localPair && p.generation == m.formatLexical.epoch*2+1 {
			return &m.formatLexical.secondParameter, nil
		}
		return &m.formatLexical.parameter, nil
	}
	if p.origin != m.origin || p.slot == 0 || p.slot >= m.nextParameter {
		return nil, ErrASTHandle
	}
	r := m.parameterAt(p.slot)
	if !r.Live || r.generation != p.generation {
		return nil, ErrASTHandle
	}
	return r, nil
}
func (m *ASTManager) allocParameter() (ASTParameter, error) {
	for m.freeParameter != 0 {
		slot := m.freeParameter
		r := m.parameterAt(slot)
		m.freeParameter = r.nextFree
		if r.generation == math.MaxUint32 {
			continue
		}
		gen := r.generation + 1
		*r = astParameterRecord{generation: gen, Live: true}
		m.liveParameters++
		return ASTParameter{m.origin, slot, gen}, nil
	}
	if m.nextParameter >= math.MaxUint32-2 {
		return ASTParameter{}, arena.ErrBounds
	}
	slot := m.nextParameter
	if e := m.ensureParameterPage(slot); e != nil {
		return ASTParameter{}, e
	}
	m.nextParameter++
	*m.parameterAt(slot) = astParameterRecord{generation: 1, Live: true}
	m.liveParameters++
	return ASTParameter{m.origin, slot, 1}, nil
}
func (m *ASTManager) dropParameter(p ASTParameter) error {
	r, e := m.parameter(p)
	if e != nil {
		return e
	}
	if r.Kind == ASTParameterRational && r.Rational != (unumeral.OwnedRational{}) {
		if e = m.numerals.Del(&r.Rational); e != nil {
			return e
		}
		m.traceParameterRationalDeallocated(r, 38)
	}
	r.Live = false
	r.nextFree = m.freeParameter
	m.freeParameter = p.slot
	m.liveParameters--
	return nil
}
func (m *ASTManager) newParameterInt(value int32) (ASTParameter, error) {
	p, e := m.allocParameter()
	if e == nil {
		m.parameterAt(p.slot).integer = value
	}
	return p, e
}
func (m *ASTManager) NewParameterInt(value int32) (ASTParameter, error) {
	if e := m.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer m.mu.Unlock()
	return m.newParameterInt(value)
}
func (m *ASTManager) NewParameterUint(value uint32) (ASTParameter, error) {
	return m.NewParameterInt(int32(value))
}
func (m *ASTManager) newParameterAST(value ASTHandle) (ASTParameter, error) {
	var r astNodeRef
	var e error
	if !value.IsNull() {
		r, e = m.internal(value)
		if e != nil {
			return ASTParameter{}, e
		}
	}
	p, e := m.allocParameter()
	if e == nil {
		v := m.parameterAt(p.slot)
		v.Kind = ASTParameterAST
		v.ast = r
	}
	return p, e
}
func (m *ASTManager) NewParameterAST(value ASTHandle) (ASTParameter, error) {
	if e := m.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer m.mu.Unlock()
	return m.newParameterAST(value)
}
func (m *ASTManager) newParameterSymbol(value usymbol.Symbol) (ASTParameter, error) {
	if _, e := m.Symbols.Hash(value); e != nil {
		return ASTParameter{}, e
	}
	p, e := m.allocParameter()
	if e == nil {
		r := m.parameterAt(p.slot)
		r.Kind = ASTParameterSymbol
		r.symbol = value
	}
	return p, e
}
func (m *ASTManager) NewParameterSymbol(value usymbol.Symbol) (ASTParameter, error) {
	if e := m.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer m.mu.Unlock()
	return m.newParameterSymbol(value)
}
func (m *ASTManager) NewParameterString(value string) (ASTParameter, error) {
	if e := m.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer m.mu.Unlock()
	s, e := m.Symbols.Intern(value)
	if e != nil {
		return ASTParameter{}, e
	}
	return m.newParameterSymbol(s)
}
func (m *ASTManager) newParameterRational(value *unumeral.OwnedRational) (ASTParameter, error) {
	p, e := m.allocParameter()
	if e != nil {
		return p, e
	}
	r := m.parameterAt(p.slot)
	r.Kind = ASTParameterRational
	r.Rational, e = m.numerals.NewValue()
	if e == nil {
		m.traceParameterRationalAllocated(r, "src/ast/ast.h", 124)
		e = m.numerals.Copy(&r.Rational, value)
	}
	if e != nil {
		m.dropParameter(p)
		return ASTParameter{}, e
	}
	return p, nil
}
func (m *ASTManager) NewParameterRational(value *unumeral.OwnedRational) (ASTParameter, error) {
	if e := m.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer m.mu.Unlock()
	return m.newParameterRational(value)
}
func (m *ASTManager) NewParameterDouble(value float64) (ASTParameter, error) {
	if e := m.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer m.mu.Unlock()
	p, e := m.allocParameter()
	if e == nil {
		r := m.parameterAt(p.slot)
		r.Kind = ASTParameterDouble
		r.double = value
	}
	return p, e
}
func (m *ASTManager) NewParameterExternal(value uint32) (ASTParameter, error) {
	if e := m.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer m.mu.Unlock()
	p, e := m.allocParameter()
	if e == nil {
		r := m.parameterAt(p.slot)
		r.Kind = ASTParameterExternal
		r.external = value
	}
	return p, e
}
func (m *ASTManager) assignParameter(dst, src ASTParameter) error {
	a, e := m.parameter(dst)
	if e != nil {
		return e
	}
	b, e := m.parameter(src)
	if e != nil {
		return e
	}
	if dst == src {
		return nil
	}
	if a.Kind == ASTParameterRational && a.Rational != (unumeral.OwnedRational{}) {
		if e = m.numerals.Del(&a.Rational); e != nil {
			return e
		}
		m.traceParameterRationalDeallocated(a, 53)
	}
	// Source deletes old owned rational before publishing the new kind/copy.
	a.Kind = b.Kind
	switch b.Kind {
	case ASTParameterInt:
		a.integer = b.integer
	case ASTParameterAST:
		a.ast = b.ast
	case ASTParameterSymbol:
		a.symbol = b.symbol
	case ASTParameterRational:
		a.Rational, e = m.numerals.NewValue()
		if e == nil {
			m.traceParameterRationalAllocated(a, "src/ast/ast.cpp", 60)
			e = m.numerals.Copy(&a.Rational, &b.Rational)
		}
		// A failed new object/copy leaves a destructible zero rational adapter,
		// preserving destruction of the old value rather than restoring it.
	case ASTParameterDouble:
		a.double = b.double
	case ASTParameterExternal:
		a.external = b.external
	default:
		return ErrASTDomain
	}
	return e
}
func (m *ASTManager) copyParameter(src ASTParameter) (ASTParameter, error) {
	if _, e := m.parameter(src); e != nil {
		return ASTParameter{}, e
	}
	p, e := m.allocParameter()
	if e != nil {
		return p, e
	}
	if e = m.assignParameter(p, src); e != nil {
		m.dropParameter(p)
		return ASTParameter{}, e
	}
	return p, nil
}
func (m *ASTManager) CopyParameter(src ASTParameter) (ASTParameter, error) {
	if e := m.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer m.mu.Unlock()
	return m.copyParameter(src)
}
func (m *ASTManager) AssignParameter(dst, src ASTParameter) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	return m.assignParameter(dst, src)
}
func (m *ASTManager) MoveParameter(src ASTParameter) (ASTParameter, error) {
	if e := m.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer m.mu.Unlock()
	r, e := m.parameter(src)
	if e != nil {
		return ASTParameter{}, e
	}
	saved := *r
	p, e := m.allocParameter()
	if e != nil {
		return p, e
	}
	dst := m.parameterAt(p.slot)
	gen := dst.generation
	*dst = saved
	dst.generation = gen
	dst.nextFree = 0
	if saved.Kind == ASTParameterRational {
		m.parameterAt(src.slot).Rational = unumeral.OwnedRational{}
		m.parameterAt(src.slot).rationalTraceID = ""
	}
	return p, nil
}
func (m *ASTManager) DelParameter(p *ASTParameter) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if p == nil {
		return ErrASTHandle
	}
	if e := m.dropParameter(*p); e != nil {
		return e
	}
	*p = ASTParameter{}
	return nil
}
func (m *ASTManager) ParameterKind(p ASTParameter) (ASTParameterKind, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.parameter(p)
	if e != nil {
		return 0, e
	}
	return r.Kind, nil
}
func (m *ASTManager) parameterHash(p ASTParameter) (uint32, error) {
	r, e := m.parameter(p)
	if e != nil {
		return 0, e
	}
	var v uint32
	switch r.Kind {
	case ASTParameterInt:
		v = uint32(r.integer)
	case ASTParameterAST:
		if e = m.validNode(r.ast); e != nil {
			return 0, e
		}
		v = m.nodeAt(r.ast.slot).hash
	case ASTParameterSymbol:
		v, e = m.Symbols.Hash(r.symbol)
	case ASTParameterRational:
		v, e = m.numerals.Hash(&r.Rational)
	case ASTParameterDouble:
		if math.IsNaN(r.double) || math.IsInf(r.double, 0) || r.double <= -1 || r.double >= 4294967296 {
			return 0, ErrASTDomain
		}
		v = uint32(r.double)
	case ASTParameterExternal:
		v = r.external
	default:
		return 0, ErrASTDomain
	}
	return (v << 2) | uint32(r.Kind), e
}
func (m *ASTManager) ParameterHash(p ASTParameter) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	return m.parameterHash(p)
}
func (m *ASTManager) parameterEqual(a, b ASTParameter) (bool, error) {
	x, e := m.parameter(a)
	if e != nil {
		return false, e
	}
	y, e := m.parameter(b)
	if e != nil {
		return false, e
	}
	if x.Kind != y.Kind {
		return false, nil
	}
	switch x.Kind {
	case ASTParameterInt:
		return x.integer == y.integer, nil
	case ASTParameterAST:
		return x.ast == y.ast, nil
	case ASTParameterSymbol:
		return x.symbol == y.symbol, nil
	case ASTParameterRational:
		return m.numerals.Equal(&x.Rational, &y.Rational)
	case ASTParameterDouble:
		return x.double == y.double, nil
	case ASTParameterExternal:
		return x.external == y.external, nil
	}
	return false, ErrASTDomain
}
func (m *ASTManager) ParameterEqual(a, b ASTParameter) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.parameterEqual(a, b)
}
