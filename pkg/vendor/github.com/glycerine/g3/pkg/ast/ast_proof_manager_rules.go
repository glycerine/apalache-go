package ast

func (m *ASTManager) proofParentsFact(k int32, ps []ASTHandle, f ASTHandle) (ASTHandle, error) {
	var args []ASTHandle
	args = append(args, ps...)
	args = append(args, f)
	return m.proofMake(k, args...)
}
func (m *ASTManager) MkProofCommutativity(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTApp)
	if e != nil {
		return ASTHandle{}, e
	}
	v := m.appAt(r.slot)
	if ASTSourceDebug && v.count != 2 {
		return ASTHandle{}, ErrASTDomain
	}
	a, e := m.proofArg(h, 1)
	if e != nil {
		return ASTHandle{}, e
	}
	b, e := m.proofArg(h, 0)
	if e != nil {
		return ASTHandle{}, e
	}
	args := [2]ASTHandle{a, b}
	swapped, e := m.mkApp(v.decl, args[:])
	if e != nil {
		return ASTHandle{}, e
	}
	fact, e := m.proofEq(h, m.external(swapped), false)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(ASTPrCommutativity, fact)
}
func (m *ASTManager) MkProofIffTrue(p ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if p.IsNull() {
		return p, nil
	}
	f, e := m.proofFact(p)
	if e != nil {
		return ASTHandle{}, e
	}
	if e = m.proofCheckBool(f); e != nil {
		return ASTHandle{}, e
	}
	fact, e := m.proofMake(ASTOpIff, f, m.external(m.trueApp))
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(ASTPrIffTrue, p, fact)
}
func (m *ASTManager) MkProofIffFalse(p ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if p.IsNull() {
		return p, nil
	}
	f, e := m.proofFact(p)
	if e != nil {
		return ASTHandle{}, e
	}
	if ASTSourceDebug {
		yes, e := m.proofKind(f, ASTOpNot)
		if e != nil {
			return ASTHandle{}, e
		}
		if !yes {
			return ASTHandle{}, ErrASTDomain
		}
	}
	a, e := m.proofArg(f, 0)
	if e != nil {
		return ASTHandle{}, e
	}
	fact, e := m.proofMake(ASTOpIff, a, m.external(m.falseApp))
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(ASTPrIffFalse, p, fact)
}
func (m *ASTManager) MkProofLemma(p, f ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if p.IsNull() {
		return p, nil
	}
	if ASTSourceDebug || m.proofRuleTraceEnabled("mk_lemma") {
		fact, e := m.proofFact(p)
		if e != nil {
			return ASTHandle{}, e
		}
		if fact != m.external(m.falseApp) && m.proofRuleTraceEnabled("mk_lemma") {
			if e := m.traceProofLemma(p); e != nil {
				return ASTHandle{}, e
			}
		}
		if ASTSourceDebug && fact != m.external(m.falseApp) {
			return ASTHandle{}, ErrASTDomain
		}
	}
	return m.proofMake(ASTPrLemma, p, f)
}
func (m *ASTManager) MkProofDistributivity(a, b ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	f, e := m.proofEq(a, b, false)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(ASTPrDistributivity, f)
}
func (m *ASTManager) proofQuantIntro(a, b, p ASTHandle, oeq bool) (ASTHandle, error) {
	if p.IsNull() {
		return ASTHandle{}, nil
	}
	if ASTSourceDebug {
		r, e := m.requireKind(a, ASTQuantifier)
		if e != nil {
			return ASTHandle{}, e
		}
		s, e := m.requireKind(b, ASTQuantifier)
		if e != nil {
			return ASTHandle{}, e
		}
		if m.quantifierAt(r.slot).count != m.quantifierAt(s.slot).count {
			return ASTHandle{}, ErrASTDomain
		}
		f, e := m.proofFact(p)
		if e != nil {
			return ASTHandle{}, e
		}
		k := ASTOpIff
		if oeq {
			k = ASTOpOEq
		}
		yes, e := m.proofKind(f, k)
		if e != nil {
			return ASTHandle{}, e
		}
		if !yes {
			return ASTHandle{}, ErrASTDomain
		}
	}
	k := ASTOpIff
	if oeq {
		k = ASTOpOEq
	}
	fact, e := m.proofMake(k, a, b)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(ASTPrQuantIntro, p, fact)
}
func (m *ASTManager) MkProofQuantIntro(a, b, p ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofQuantIntro(a, b, p, false)
}
func (m *ASTManager) MkProofOEqQuantIntro(a, b, p ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofQuantIntro(a, b, p, true)
}
func (m *ASTManager) proofGuardedRelation(k int32, a, b ASTHandle, oeq bool) (ASTHandle, error) {
	if m.proofMode == ASTProofDisabled {
		return ASTHandle{}, nil
	}
	var f ASTHandle
	var e error
	if oeq {
		f, e = m.proofEq(a, b, true)
	} else {
		f, e = m.proofMake(ASTOpIff, a, b)
	}
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(k, f)
}
func (m *ASTManager) MkProofPullQuant(a, q ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofGuardedRelation(ASTPrPullQuant, a, q, false)
}
func (m *ASTManager) MkProofPushQuant(q, a ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofGuardedRelation(ASTPrPushQuant, q, a, false)
}
func (m *ASTManager) MkProofElimUnusedVars(q, a ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofGuardedRelation(ASTPrElimUnusedVars, q, a, false)
}
func (m *ASTManager) MkProofDER(q, a ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofGuardedRelation(ASTPrDER, q, a, false)
}
func (m *ASTManager) MkProofSkolemization(q, a ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.proofMode == ASTProofDisabled {
		return ASTHandle{}, nil
	}
	if e := m.proofCheckBool(q); e != nil {
		return ASTHandle{}, e
	}
	if e := m.proofCheckBool(a); e != nil {
		return ASTHandle{}, e
	}
	return m.proofGuardedRelation(ASTPrSkolemize, q, a, true)
}
func (m *ASTManager) MkProofDefAxiom(a ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofGuardedUnary(ASTPrDefAxiom, a, false)
}
func (m *ASTManager) MkProofRewriteStar(a, b ASTHandle, ps []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.proofMode == ASTProofDisabled {
		return ASTHandle{}, nil
	}
	var args []ASTHandle
	args = append(args, ps...)
	f, e := m.proofEq(a, b, false)
	if e != nil {
		return ASTHandle{}, e
	}
	args = append(args, f)
	return m.proofMake(ASTPrRewriteStar, args...)
}
func (m *ASTManager) MkProofApplyDefs(a, b ASTHandle, ps []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.proofMode == ASTProofDisabled {
		return ASTHandle{}, nil
	}
	var args []ASTHandle
	args = append(args, ps...)
	f, e := m.proofEq(a, b, true)
	if e != nil {
		return ASTHandle{}, e
	}
	args = append(args, f)
	return m.proofMake(ASTPrApplyDef, args...)
}
func (m *ASTManager) MkProofApplyDef(a, b, p ASTHandle) (ASTHandle, error) {
	ps := [1]ASTHandle{p}
	return m.MkProofApplyDefs(a, b, ps[:])
}
func (m *ASTManager) proofNNFParents(ps []ASTHandle) (bool, error) {
	for _, p := range ps {
		has, e := m.proofHasFact(p)
		if e != nil {
			return false, e
		}
		if !has {
			return false, nil
		}
		f, e := m.proofFact(p)
		if e != nil {
			return false, e
		}
		yes, e := m.proofKind(f, ASTOpOEq)
		if e != nil {
			return false, e
		}
		if !yes {
			return false, nil
		}
	}
	return true, nil
}
func (m *ASTManager) CheckNNFProofParents(ps []ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.proofNNFParents(ps)
}
func (m *ASTManager) proofNNF(a, b ASTHandle, ps []ASTHandle, negative bool) (ASTHandle, error) {
	if m.proofMode == ASTProofDisabled {
		return ASTHandle{}, nil
	}
	if _, e := m.proofNNFParents(ps); e != nil {
		return ASTHandle{}, e
	}
	var args []ASTHandle
	args = append(args, ps...)
	k := ASTPrNNFPos
	if negative {
		var e error
		a, e = m.proofMake(ASTOpNot, a)
		if e != nil {
			return ASTHandle{}, e
		}
		k = ASTPrNNFNeg
	}
	f, e := m.proofEq(a, b, true)
	if e != nil {
		return ASTHandle{}, e
	}
	args = append(args, f)
	return m.proofMake(k, args...)
}
func (m *ASTManager) MkProofNNFPos(a, b ASTHandle, ps []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofNNF(a, b, ps, false)
}
func (m *ASTManager) MkProofNNFNeg(a, b ASTHandle, ps []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofNNF(a, b, ps, true)
}
func (m *ASTManager) MkProofIffOEq(p ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if p.IsNull() {
		return p, nil
	}
	f, e := m.proofFact(p)
	if e != nil {
		return ASTHandle{}, e
	}
	oeq, e := m.proofKind(f, ASTOpOEq)
	if e != nil {
		return ASTHandle{}, e
	}
	if ASTSourceDebug {
		iff, e := m.proofKind(f, ASTOpIff)
		if e != nil {
			return ASTHandle{}, e
		}
		if !oeq && !iff {
			return ASTHandle{}, ErrASTDomain
		}
	}
	if oeq {
		return p, nil
	}
	a, e := m.proofArg(f, 0)
	if e != nil {
		return ASTHandle{}, e
	}
	b, e := m.proofArg(f, 1)
	if e != nil {
		return ASTHandle{}, e
	}
	fact, e := m.proofEq(a, b, true)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(ASTPrIffOEq, p, fact)
}
func (m *ASTManager) MkProofAndElim(p ASTHandle, i uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.proofMode == ASTProofDisabled {
		return ASTHandle{}, nil
	}
	f, e := m.proofFact(p)
	if e != nil {
		return ASTHandle{}, e
	}
	if ASTSourceDebug {
		yes, e := m.proofKind(f, ASTOpAnd)
		if e != nil {
			return ASTHandle{}, e
		}
		if !yes {
			return ASTHandle{}, ErrASTDomain
		}
	}
	if m.proofRuleTraceEnabled("mk_and_elim") {
		r, e := m.requireKind(f, ASTApp)
		if e != nil {
			return ASTHandle{}, e
		}
		if i >= m.appAt(r.slot).count {
			m.mu.Unlock()
			traceErr := m.traceProofAndElim(f, i)
			m.mu.Lock()
			if traceErr != nil {
				return ASTHandle{}, traceErr
			}
		}
	}
	a, e := m.proofArg(f, i)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(ASTPrAndElim, p, a)
}
func (m *ASTManager) MkProofNotOrElim(p ASTHandle, i uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.proofMode == ASTProofDisabled {
		return ASTHandle{}, nil
	}
	f, e := m.proofFact(p)
	if e != nil {
		return ASTHandle{}, e
	}
	if ASTSourceDebug {
		yes, e := m.proofKind(f, ASTOpNot)
		if e != nil {
			return ASTHandle{}, e
		}
		if !yes {
			return ASTHandle{}, ErrASTDomain
		}
	}
	or, e := m.proofArg(f, 0)
	if e != nil {
		return ASTHandle{}, e
	}
	if ASTSourceDebug {
		yes, e := m.proofKind(or, ASTOpOr)
		if e != nil {
			return ASTHandle{}, e
		}
		if !yes {
			return ASTHandle{}, ErrASTDomain
		}
	}
	a, e := m.proofArg(or, i)
	if e != nil {
		return ASTHandle{}, e
	}
	if yes, e := m.proofKind(a, ASTOpNot); e != nil {
		return ASTHandle{}, e
	} else if yes {
		a, e = m.proofArg(a, 0)
		if e != nil {
			return ASTHandle{}, e
		}
	} else {
		a, e = m.proofMake(ASTOpNot, a)
		if e != nil {
			return ASTHandle{}, e
		}
	}
	return m.proofMake(ASTPrNotOrElim, p, a)
}
