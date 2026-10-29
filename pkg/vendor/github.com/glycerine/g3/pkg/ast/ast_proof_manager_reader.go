package ast

// Source: src/ast/ast.cpp:2874-2887.

// MatchProofQuantInstantiation appends borrowed expression handles. The proof
// owns their lifetime; decoding does not acquire additional AST references.
func (m *ASTManager) MatchProofQuantInstantiation(p ASTHandle, out *ASTHandle, binding *[]ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	yes, e := m.proofKind(p, ASTPrQuantInst)
	if e != nil || !yes {
		return false, e
	}
	f, e := m.proofArg(p, 0)
	if e != nil {
		return false, e
	}
	if out == nil {
		return false, ErrASTHandle
	}
	*out = f
	r, e := m.requireKind(p, ASTApp)
	if e != nil {
		return false, e
	}
	d := m.declAt(m.appAt(r.slot).decl.slot)
	if binding == nil {
		return false, ErrASTHandle
	}
	if ASTSourceDebug && len(*binding) != 0 {
		return false, ErrASTDomain
	}
	if d.info.slot == 0 {
		return true, nil
	}
	info := m.infoAt(d.info.slot)
	for i := uint32(0); i < info.count; i++ {
		param, e := m.infoParameter(info, i)
		if e != nil {
			return false, e
		}
		record, e := m.parameter(param)
		if e != nil {
			return false, e
		}
		if record.Kind != ASTParameterAST {
			return false, ErrASTDomain
		}
		// get_ast/to_expr checks at the source's parameter extraction site;
		// no global prevalidation hides prior output or append prefixes.
		h := m.external(record.ast)
		if _, e = m.requireExpr(h); e != nil {
			return false, e
		}
		*binding = append(*binding, h)
	}
	return true, nil
}
