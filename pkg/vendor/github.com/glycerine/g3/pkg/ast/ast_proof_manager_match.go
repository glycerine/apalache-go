package ast

func (m *ASTManager) proofMatchUnary(h ASTHandle, k int32, out *ASTHandle) (bool, error) {
	yes, e := m.proofKind(h, k)
	if e != nil || !yes {
		return false, e
	}
	r, e := m.requireKind(h, ASTApp)
	if e != nil {
		return false, e
	}
	if m.appAt(r.slot).count != 1 {
		return false, nil
	}
	a, e := m.proofArg(h, 0)
	if e != nil {
		return false, e
	}
	if out == nil {
		return false, ErrASTHandle
	}
	*out = a
	return true, nil
}
func (m *ASTManager) MatchProofAsserted(h ASTHandle, out *ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.proofMatchUnary(h, ASTPrAsserted, out)
}
func (m *ASTManager) MatchProofHypothesis(h ASTHandle, out *ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.proofMatchUnary(h, ASTPrHypothesis, out)
}
func (m *ASTManager) MatchProofLemma(h ASTHandle, out *ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.proofMatchUnary(h, ASTPrLemma, out)
}
func (m *ASTManager) MatchProofRewrite(h ASTHandle, left, right *ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	yes, e := m.proofKind(h, ASTPrRewrite)
	if e != nil || !yes {
		return false, e
	}
	f, e := m.proofArg(h, 0)
	if e != nil {
		return false, e
	}
	eq, e := m.proofKind(f, ASTOpEq)
	if e != nil {
		return false, e
	}
	iff := false
	if !eq {
		iff, e = m.proofKind(f, ASTOpIff)
		if e != nil {
			return false, e
		}
	}
	if !eq && !iff {
		// Source VERIFY runs in release too. Its failure invokes
		// __builtin_unreachable in the selected release compiler; report that
		// undefined boundary explicitly. Diagnostic stream transport is open.
		return false, ErrASTDomain
	}
	r, e := m.requireKind(f, ASTApp)
	if e != nil {
		return false, e
	}
	if m.appAt(r.slot).count != 2 {
		return false, ErrASTDomain
	}
	a, e := m.proofArg(f, 0)
	if e != nil {
		return false, e
	}
	b, e := m.proofArg(f, 1)
	if e != nil {
		return false, e
	}
	if left == nil || right == nil {
		return false, ErrASTHandle
	}
	*left = a
	*right = b
	return true, nil
}
