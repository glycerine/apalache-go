package ast

// Source: src/ast/arith_decl_plugin.cpp:400-449.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	// ASTArithmeticNumeralInvocation captures only the factory's reached lexical
	// parameter. Cache hits and rejected inputs never acquire this child. Cache
	// publication pins remain owned by the arithmetic plugin.
)

type ASTArithmeticNumeralInvocation struct {
	self                                 *ASTArithmeticNumeralInvocation
	manager                              *ASTManager
	frame                                *astArithNumeralFrame
	invocation                           uint64
	active, stopped, sourceDone, retired bool
	bodyCause, sourceCause               error
}

func (c *ASTArithmeticNumeralInvocation) prepare(m *ASTManager) error {
	if c == nil || c.self != nil && (c.self != c || c.manager != m) {
		return ErrASTHandle
	}
	if c.active || c.stopped || c.frame != nil && !c.retired {
		return ErrASTPending
	}
	*c = ASTArithmeticNumeralInvocation{self: c, manager: m, active: true}
	return nil
}
func (c *ASTArithmeticNumeralInvocation) valid(m *ASTManager) error {
	if c == nil || c.self != c || c.manager != m {
		return ErrASTHandle
	}
	if c.active {
		return ErrASTPending
	}
	return nil
}

func (m *ASTManager) ArithmeticNumeralWithInvocation(value *unumeral.OwnedRational, integer bool, c *ASTArithmeticNumeralInvocation) (h ASTHandle, err error) {
	if e := m.lock(); e != nil {
		return h, e
	}
	defer m.mu.Unlock()
	if err = c.prepare(m); err != nil {
		return
	}
	defer func() { c.active = false; c.stopped = err != nil }()
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	r, e := m.mkArithmeticNumeralCaptured(value, integer, false, c)
	return m.external(r), e
}

func (m *ASTManager) ArithmeticNumeralInvocationSourceReady(c *ASTArithmeticNumeralInvocation) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if c != nil && c.self == nil {
		return nil
	}
	if e := c.valid(m); e != nil {
		return e
	}
	if c.frame == nil || c.sourceDone {
		return c.sourceCause
	}
	if c.retired {
		return c.sourceCause
	}
	if c.frame.owner != m.arithmeticPlugin || c.invocation == 0 || c.frame.invocation != c.invocation {
		return ErrASTHandle
	}
	return c.sourceCause
}

func (m *ASTManager) CleanupArithmeticNumeralInvocation(c *ASTArithmeticNumeralInvocation) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if c != nil && c.self == nil {
		return nil
	}
	if e := c.valid(m); e != nil {
		return e
	}
	if c.frame == nil || c.retired {
		return nil
	}
	f := c.frame
	if f.owner != m.arithmeticPlugin || c.invocation == 0 || f.invocation != c.invocation {
		return ErrASTHandle
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	if e := f.destroy(m, true); e != nil {
		return e
	}
	c.retired = f.ownDone
	return nil
}
