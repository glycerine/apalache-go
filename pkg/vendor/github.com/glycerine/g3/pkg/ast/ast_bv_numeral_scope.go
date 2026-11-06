package ast

// Source: src/ast/rewriter/bv_trailing.cpp:149-171.

import "errors"

// ASTBitVectorNumeralScope owns the caller's copied c_val across subsequent
// AST calls. Keep its address stable until source exit and owning cleanup.
// The allocation-free default rational is erased for a nonnumeral input.
type ASTBitVectorNumeralScope struct {
	self                    *ASTBitVectorNumeralScope
	manager                 *ASTManager
	frame                   *astBVNumeralFrame
	invocation              uint64
	ended, cleaned, retired bool
	constructorUsed         bool
	readCause               error
}

func (s *ASTBitVectorNumeralScope) valid(m *ASTManager) error {
	if s == nil || s.self != s || s.manager != m {
		return ErrASTHandle
	}
	if !s.retired && s.frame != nil && (s.frame.self != s.frame || s.frame.invocation != s.invocation || s.frame.owner.owner != m) {
		return ErrASTHandle
	}
	return nil
}

func (m *ASTManager) ReadBitVectorNumeralScope(arg ASTHandle, s *ASTBitVectorNumeralScope) (matched, one bool, width uint32, result error) {
	if e := m.lock(); e != nil {
		return false, false, 0, e
	}
	defer m.mu.Unlock()
	if s == nil || s.self != nil {
		return false, false, 0, ErrASTHandle
	}
	*s = ASTBitVectorNumeralScope{manager: m}
	s.self = s
	defer func() { s.readCause = result }()
	r, e := m.requireExpr(arg)
	if e != nil {
		return false, false, 0, e
	}
	matched, e = m.bitVectorAppKind(r, 0)
	if e != nil || !matched {
		return matched, false, 0, e
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return true, false, 0, e
	}
	s.frame, s.invocation = f, f.invocation
	f.values[0], e = m.numerals.NewValue()
	if e != nil {
		return true, false, 0, e
	}
	f.placed[0], f.complete[0] = true, true
	_, e = m.readBitVectorNumeralWithInvocation(r, &f.values[0], &width, &f.children[2])
	if e != nil {
		return true, false, width, e
	}
	one, result = m.numerals.IsOne(&f.values[0])
	return true, one, width, result
}

// BitVectorNumeralUnsignedInScope captures the source's nested mk_numeral
// invocation before entry. Its stopped child joins before older c_val DT.
func (m *ASTManager) BitVectorNumeralUnsignedInScope(value, bits uint32, s *ASTBitVectorNumeralScope) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if e := s.valid(m); e != nil {
		return ASTHandle{}, e
	}
	if s.ended || s.cleaned || s.constructorUsed || s.readCause != nil {
		return ASTHandle{}, ErrASTPending
	}
	if s.frame == nil {
		return ASTHandle{}, ErrASTHandle
	}
	s.constructorUsed = true
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	r, e := m.bitVectorNumeralUnsignedCaptured(value, bits, &s.frame.normalizationChild)
	return m.external(r), e
}

// EndBitVectorNumeralScope returns the body error joined with reached source
// DT. Call it once even after a body refusal; cleanup is a separate operation.
func (m *ASTManager) EndBitVectorNumeralScope(s *ASTBitVectorNumeralScope, body error) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if e := s.valid(m); e != nil {
		return e
	}
	if s.ended || s.cleaned {
		return ErrASTPending
	}
	s.ended = true
	body = errors.Join(body, s.readCause)
	if s.frame == nil {
		s.retired = true
		return body
	}
	// Join only the exact captured constructor. An uncaptured stopped
	// younger invocation cannot grant older DT even when body has an error.
	if s.frame.owner.numeralFrames != s.frame && s.frame.normalizationChild.frame == nil {
		return errors.Join(body, ErrASTPending)
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	s.frame.finishNumeral(&body, nil)
	s.retired = s.frame.retired
	return body
}

func (m *ASTManager) CleanupBitVectorNumeralScope(s *ASTBitVectorNumeralScope) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if e := s.valid(m); e != nil {
		return e
	}
	if s.cleaned {
		return nil
	}
	if !s.ended {
		return ErrASTPending
	}
	if !s.retired && s.frame != nil {
		previous := m.formatExecuting
		m.formatExecuting = true
		defer func() { m.formatExecuting = previous }()
		if e := s.frame.destroy(true); e != nil {
			return e
		}
		// Retained frame headers/contexts remain on the plugin's actual list
		// until its owning Close; no ambient younger frame is adopted here.
	}
	s.cleaned = true
	return nil
}

// BitVectorNumeralScopeSourceReady separates source DT completion from the
// body error, so the caller can decide whether older vector/ref DT is reached.
// Owning cleanup never grants permission to resume a stopped source footer.
func (m *ASTManager) BitVectorNumeralScopeSourceReady(s *ASTBitVectorNumeralScope) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if e := s.valid(m); e != nil {
		return e
	}
	if !s.ended {
		return ErrASTPending
	}
	if s.retired || s.frame == nil {
		return nil
	}
	if s.frame.sourceCause != nil {
		return s.frame.sourceCause
	}
	if !s.frame.sourceDone {
		return ErrASTPending
	}
	return nil
}
