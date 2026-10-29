package ast

// Source: src/ast/arith_decl_plugin.cpp:632-639.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	// ArithmeticNumeralValueWithInvocation follows the caller's arith_util family
	// identity, copies the stored value first, then reads the integer/real flag.
	// Nonmatches leave both outputs and the invocation unread.
)

func (m *ASTManager) ArithmeticNumeralValueWithInvocation(family FamilyID, h ASTHandle, out *unumeral.OwnedRational, integer *bool, c *unumeral.RationalServiceInvocation) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	return m.readArithmeticNumeralWithInvocation(family, r, out, integer, c)
}

func (m *ASTManager) readArithmeticNumeralWithInvocation(family FamilyID, r astNodeRef, out *unumeral.OwnedRational, integer *bool, c *unumeral.RationalServiceInvocation) (bool, error) {
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	fid, kind, e := m.declIdentity(m.appAt(r.slot).decl)
	if e != nil || fid != family || kind != 0 {
		return false, e
	}
	info, e := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
	if e != nil {
		return false, e
	}
	p, e := m.infoParameter(info, 0)
	if e != nil {
		return false, e
	}
	value, e := m.parameter(p)
	if e != nil {
		return false, e
	}
	if value.Kind != ASTParameterRational {
		return false, ErrASTDomain
	}
	if c == nil {
		return false, ErrASTHandle
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	if e = m.numerals.CopyWithInvocation(out, &value.Rational, c); e != nil {
		return false, e
	}
	p, e = m.infoParameter(info, 1)
	if e != nil {
		return false, e
	}
	flag, e := m.parameter(p)
	if e != nil {
		return false, e
	}
	if flag.Kind != ASTParameterInt {
		return false, ErrASTDomain
	}
	if integer == nil {
		return false, ErrASTHandle
	}
	*integer = flag.integer != 0
	return true, nil
}
