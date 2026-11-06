package ast

// Parameter records own their copied payloads; slice storage is Go-managed.
func (m *ASTManager) dropProofParameters(parameters []ASTParameter) {
	for _, p := range parameters {
		m.dropParameter(p)
	}
}

func (m *ASTManager) MkProofQuantInstantiation(f ASTHandle, bindings []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.proofMode == ASTProofDisabled {
		return ASTHandle{}, nil
	}
	var parameters []ASTParameter
	defer func() { m.dropProofParameters(parameters) }()
	for _, h := range bindings {
		p, e := m.newParameterAST(h)
		if e != nil {
			return ASTHandle{}, e
		}
		parameters = append(parameters, p)
		// The source assertion is after push, and parameter(AST) always has
		// this tag even when its borrowed AST pointer is null.
		if ASTSourceDebug && m.parameterAt(p.slot).Kind != ASTParameterAST {
			return ASTHandle{}, ErrASTDomain
		}
	}
	args := [1]ASTHandle{f}
	r, e := m.mkFamilyApp(0, ASTPrQuantInst, parameters, args[:])
	return m.external(r), e
}
func (m *ASTManager) MkProofTheoryLemma(tid FamilyID, f ASTHandle, ps []ASTHandle, params []ASTParameter) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.proofMode == ASTProofDisabled {
		return ASTHandle{}, nil
	}
	var args []ASTHandle
	var parameters []ASTParameter
	defer func() { m.dropProofParameters(parameters) }()
	s, e := m.families.GetName(tid)
	if e != nil {
		return ASTHandle{}, e
	}
	p, e := m.newParameterSymbol(s)
	if e != nil {
		return ASTHandle{}, e
	}
	parameters = append(parameters, p)
	for _, param := range params {
		p, e = m.copyParameter(param)
		if e != nil {
			return ASTHandle{}, e
		}
		parameters = append(parameters, p)
	}
	args = append(args, ps...)
	args = append(args, f)
	r, e := m.mkFamilyApp(0, ASTPrTheoryLemma, parameters, args)
	return m.external(r), e
}
