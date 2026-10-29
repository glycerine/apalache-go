package ast

// Source: src/ast/pb_decl_plugin.cpp:181-300; src/ast/pb_decl_plugin.h:81-128.

import (
	"strconv"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *ASTManager) pseudoBooleanDeclaration(h ASTHandle) (astNodeRef, int32, error) {
	r, e := m.internal(h)
	if e != nil {
		return astNodeRef{}, 0, e
	}
	if m.nodeAt(r.slot).kind == ASTApp {
		r = m.appAt(r.slot).decl
	}
	if m.nodeAt(r.slot).kind != ASTFuncDecl {
		return astNodeRef{}, 0, ErrASTDomain
	}
	family, kind, e := m.declIdentity(r)
	if e != nil {
		return astNodeRef{}, 0, e
	}
	p := m.auxiliaryPlugin(astPseudoBooleanPlugin)
	if p == nil || family != p.family {
		return r, kind, nil
	}
	return r, kind, nil
}

func (m *ASTManager) pseudoBooleanIs(h ASTHandle, expected int32) (bool, error) {
	d, kind, e := m.pseudoBooleanDeclaration(h)
	if e != nil {
		return false, e
	}
	family, _, e := m.declIdentity(d)
	p := m.auxiliaryPlugin(astPseudoBooleanPlugin)
	return e == nil && p != nil && family == p.family && kind == expected, e
}

func (m *ASTManager) IsPseudoBooleanAtMost(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.pseudoBooleanIs(h, ASTPBOpAtMost)
}
func (m *ASTManager) IsPseudoBooleanAtLeast(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.pseudoBooleanIs(h, ASTPBOpAtLeast)
}
func (m *ASTManager) IsPseudoBooleanLE(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.pseudoBooleanIs(h, ASTPBOpLE)
}
func (m *ASTManager) IsPseudoBooleanGE(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.pseudoBooleanIs(h, ASTPBOpGE)
}
func (m *ASTManager) IsPseudoBooleanEQ(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.pseudoBooleanIs(h, ASTPBOpEQ)
}
func (m *ASTManager) IsPseudoBooleanAux(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.pseudoBooleanIs(h, ASTPBOpAuxBool)
}

func (m *ASTManager) pseudoBooleanParameterValue(parameter ASTParameter) (result unumeral.OwnedRational, err error) {
	value, e := m.parameter(parameter)
	if e != nil {
		return result, e
	}
	result, e = m.numerals.NewValue()
	if e != nil {
		return result, e
	}
	switch value.Kind {
	case ASTParameterInt:
		e = m.numerals.SetInt64(&result, int64(value.integer))
	case ASTParameterRational:
		e = m.numerals.Copy(&result, &value.Rational)
	default:
		e = ErrASTDomain
	}
	if e != nil {
		err = e
		_ = m.numerals.Del(&result)
	}
	return result, err
}

func (m *ASTManager) pseudoBooleanParameter(h ASTHandle, index uint32) (unumeral.OwnedRational, error) {
	d, kind, e := m.pseudoBooleanDeclaration(h)
	if e != nil {
		return unumeral.OwnedRational{}, e
	}
	p := m.auxiliaryPlugin(astPseudoBooleanPlugin)
	family, _, e := m.declIdentity(d)
	if e != nil || p == nil || family != p.family || kind < ASTPBOpAtMost || kind > ASTPBOpEQ {
		return unumeral.OwnedRational{}, ErrASTDomain
	}
	info, e := m.funcInfo(m.declAt(d.slot).info)
	if e != nil || index >= info.count {
		return unumeral.OwnedRational{}, ErrASTDomain
	}
	parameter, e := m.infoParameter(info, index)
	if e != nil {
		return unumeral.OwnedRational{}, e
	}
	return m.pseudoBooleanParameterValue(parameter)
}

// PseudoBooleanBound returns an owning copy that the caller releases through
// the manager's rational service.
func (m *ASTManager) PseudoBooleanBound(h ASTHandle) (unumeral.OwnedRational, error) {
	if e := m.lock(); e != nil {
		return unumeral.OwnedRational{}, e
	}
	defer m.mu.Unlock()
	return m.pseudoBooleanParameter(h, 0)
}

// PseudoBooleanCoefficient returns one for cardinality constraints and an
// owning coefficient copy for general PB relations.
func (m *ASTManager) PseudoBooleanCoefficient(h ASTHandle, index uint32) (unumeral.OwnedRational, error) {
	if e := m.lock(); e != nil {
		return unumeral.OwnedRational{}, e
	}
	defer m.mu.Unlock()
	_, kind, e := m.pseudoBooleanDeclaration(h)
	if e != nil {
		return unumeral.OwnedRational{}, e
	}
	if kind == ASTPBOpAtMost || kind == ASTPBOpAtLeast {
		return m.numerals.One()
	}
	return m.pseudoBooleanParameter(h, index+1)
}

func (m *ASTManager) PseudoBooleanHasUnitCoefficients(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	d, kind, e := m.pseudoBooleanDeclaration(h)
	if e != nil {
		return false, e
	}
	if kind == ASTPBOpAtMost || kind == ASTPBOpAtLeast {
		return true, nil
	}
	if kind < ASTPBOpLE || kind > ASTPBOpEQ {
		return false, ErrASTDomain
	}
	info, e := m.funcInfo(m.declAt(d.slot).info)
	if e != nil {
		return false, e
	}
	for i := uint32(1); i < info.count; i++ {
		value, e := m.pseudoBooleanParameter(h, i)
		if e != nil {
			return false, e
		}
		one, check := m.numerals.IsOne(&value)
		cleanup := m.numerals.Del(&value)
		if check != nil {
			return false, check
		}
		if cleanup != nil {
			return false, cleanup
		}
		if !one {
			return false, nil
		}
	}
	return true, nil
}

func (m *ASTManager) MkFreshPseudoBooleanAux() (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	p := m.auxiliaryPlugin(astPseudoBooleanPlugin)
	if p == nil {
		return ASTHandle{}, ErrASTHandle
	}
	name, e := m.Symbols.Intern("pb!" + strconv.FormatUint(uint64(m.freshID), 10))
	if e != nil {
		return ASTHandle{}, e
	}
	m.freshID++
	d, e := m.pluginDecl(name, p.family, ASTPBOpAuxBool, nil, nil, m.basic.boolSort, ASTFuncFlags{}, false)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e := m.mkApp(d, nil)
	return m.external(r), e
}
