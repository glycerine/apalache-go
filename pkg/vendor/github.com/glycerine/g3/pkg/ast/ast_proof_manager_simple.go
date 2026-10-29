package ast

func (m *ASTManager) MkProofTrue() (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	args := [1]ASTHandle{m.external(m.trueApp)}
	r, e := m.proofCore(0, ASTPrTrue, args[:])
	return m.external(r), e
}
func (m *ASTManager) proofGuardedUnary(k int32, h ASTHandle, checkBool bool) (ASTHandle, error) {
	if checkBool {
		if e := m.proofCheckBool(h); e != nil {
			return ASTHandle{}, e
		}
	}
	args := [1]ASTHandle{h}
	r, e := m.proofCore(0, k, args[:])
	return m.external(r), e
}
func (m *ASTManager) MkProofAsserted(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.proofAssertedTraceEnabled() {
		isBool, e := m.proofBool(h)
		if e != nil {
			return ASTHandle{}, e
		}
		if !isBool {
			// Source CTRACE precedes the debug-only Boolean assertion. The
			// SMT2 printer takes this manager's lock itself.
			m.mu.Unlock()
			traceErr := m.traceProofAssertedNonBool(h)
			m.mu.Lock()
			if traceErr != nil {
				return ASTHandle{}, traceErr
			}
		}
	}
	return m.proofGuardedUnary(ASTPrAsserted, h, true)
}
func (m *ASTManager) MkProofGoal(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofGuardedUnary(ASTPrGoal, h, true)
}
func (m *ASTManager) MkProofDefIntro(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofGuardedUnary(ASTPrDefIntro, h, true)
}
func (m *ASTManager) MkProofHypothesis(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofMake(ASTPrHypothesis, h)
}
func (m *ASTManager) proofReflexivity(h ASTHandle, oeq bool) (ASTHandle, error) {
	fact, e := m.proofEq(h, h, oeq)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(ASTPrReflexivity, fact)
}
func (m *ASTManager) MkProofReflexivity(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofReflexivity(h, false)
}
func (m *ASTManager) MkProofOEqReflexivity(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofReflexivity(h, true)
}
func (m *ASTManager) proofRewrite(a, b ASTHandle, oeq bool) (ASTHandle, error) {
	if m.proofMode == ASTProofDisabled {
		return ASTHandle{}, nil
	}
	fact, e := m.proofEq(a, b, oeq)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(ASTPrRewrite, fact)
}
func (m *ASTManager) MkProofRewrite(a, b ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofRewrite(a, b, false)
}
func (m *ASTManager) MkProofOEqRewrite(a, b ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofRewrite(a, b, true)
}
func (m *ASTManager) proofSymmetry(p ASTHandle) (ASTHandle, error) {
	if p.IsNull() {
		return p, nil
	}
	if yes, e := m.proofKind(p, ASTPrReflexivity); e != nil {
		return ASTHandle{}, e
	} else if yes {
		return p, nil
	}
	if yes, e := m.proofKind(p, ASTPrSymmetry); e != nil {
		return ASTHandle{}, e
	} else if yes {
		return m.proofArg(p, 0)
	}
	f, e := m.proofFact(p)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e := m.requireKind(f, ASTApp)
	if e != nil {
		return ASTHandle{}, e
	}
	a := m.appAt(r.slot)
	if ASTSourceDebug && a.count != 2 {
		return ASTHandle{}, ErrASTDomain
	}
	x, e := m.proofArg(f, 1)
	if e != nil {
		return ASTHandle{}, e
	}
	y, e := m.proofArg(f, 0)
	if e != nil {
		return ASTHandle{}, e
	}
	args := [2]ASTHandle{x, y}
	out, e := m.mkApp(a.decl, args[:])
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(ASTPrSymmetry, p, m.external(out))
}
func (m *ASTManager) MkProofSymmetry(p ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofSymmetry(p)
}
func (m *ASTManager) proofRelation(f ASTHandle) (astNodeRef, ASTHandle, ASTHandle, error) {
	r, e := m.requireKind(f, ASTApp)
	if e != nil {
		return astNodeRef{}, ASTHandle{}, ASTHandle{}, e
	}
	v := m.appAt(r.slot)
	if ASTSourceDebug && v.count != 2 {
		return astNodeRef{}, ASTHandle{}, ASTHandle{}, ErrASTDomain
	}
	a, e := m.proofArg(f, 0)
	if e != nil {
		return astNodeRef{}, ASTHandle{}, ASTHandle{}, e
	}
	b, e := m.proofArg(f, 1)
	return v.decl, a, b, e
}
func (m *ASTManager) proofTransitivity(p, q ASTHandle) (ASTHandle, error) {
	if p.IsNull() {
		return q, nil
	}
	if q.IsNull() {
		return p, nil
	}
	traceEnabled := m.proofTransitivityTraceEnabled()
	if ASTSourceDebug || traceEnabled {
		f, e := m.proofFact(p)
		if e != nil {
			return ASTHandle{}, e
		}
		g, e := m.proofFact(q)
		if e != nil {
			return ASTHandle{}, e
		}
		d, _, b, e := m.proofRelation(f)
		if e != nil {
			return ASTHandle{}, e
		}
		other, c, _, e := m.proofRelation(g)
		if e != nil {
			return ASTHandle{}, e
		}
		if d != other && traceEnabled {
			firstDecl, secondDecl := m.external(d), m.external(other)
			m.mu.Unlock()
			traceErr := m.traceProofTransitivityDeclarations(f, g, firstDecl, secondDecl)
			m.mu.Lock()
			if traceErr != nil {
				return ASTHandle{}, traceErr
			}
		}
		compatible := d == other
		if ASTSourceDebug && !compatible {
			fIff, e := m.proofKind(f, ASTOpIff)
			if e != nil {
				return ASTHandle{}, e
			}
			fEq, e := m.proofKind(f, ASTOpEq)
			if e != nil {
				return ASTHandle{}, e
			}
			fOEq, e := m.proofKind(f, ASTOpOEq)
			if e != nil {
				return ASTHandle{}, e
			}
			gIff, e := m.proofKind(g, ASTOpIff)
			if e != nil {
				return ASTHandle{}, e
			}
			gEq, e := m.proofKind(g, ASTOpEq)
			if e != nil {
				return ASTHandle{}, e
			}
			gOEq, e := m.proofKind(g, ASTOpOEq)
			if e != nil {
				return ASTHandle{}, e
			}
			compatible = (fIff || fEq) && (gIff || gEq) || (fEq || fOEq) && (gEq || gOEq)
		}
		if ASTSourceDebug && !compatible {
			return ASTHandle{}, ErrASTDomain
		}
		if b != c && traceEnabled {
			m.mu.Unlock()
			traceErr := m.traceProofTransitivityMiddle(f, g, p, q)
			m.mu.Lock()
			if traceErr != nil {
				return ASTHandle{}, traceErr
			}
		}
		if ASTSourceDebug && b != c {
			return ASTHandle{}, ErrASTDomain
		}
	}
	// Source reflexivity branches precede the release body's fact extraction.
	if yes, e := m.proofKind(p, ASTPrReflexivity); e != nil {
		return ASTHandle{}, e
	} else if yes {
		return q, nil
	}
	if yes, e := m.proofKind(q, ASTPrReflexivity); e != nil {
		return ASTHandle{}, e
	} else if yes {
		return p, nil
	}
	f, e := m.proofFact(p)
	if e != nil {
		return ASTHandle{}, e
	}
	g, e := m.proofFact(q)
	if e != nil {
		return ASTHandle{}, e
	}
	fr, e := m.requireKind(f, ASTApp)
	if e != nil {
		return ASTHandle{}, e
	}
	d := m.appAt(fr.slot).decl
	if yes, e := m.proofKind(g, ASTOpOEq); e != nil {
		return ASTHandle{}, e
	} else if yes {
		gr, e := m.requireKind(g, ASTApp)
		if e != nil {
			return ASTHandle{}, e
		}
		d = m.appAt(gr.slot).decl
	}
	first, e := m.proofArg(f, 0)
	if e != nil {
		return ASTHandle{}, e
	}
	last, e := m.proofArg(g, 1)
	if e != nil {
		return ASTHandle{}, e
	}
	args := [2]ASTHandle{first, last}
	fact, e := m.mkApp(d, args[:])
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMake(ASTPrTransitivity, p, q, m.external(fact))
}
func (m *ASTManager) MkProofTransitivity(p, q ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofTransitivity(p, q)
}
func (m *ASTManager) MkProofTransitivity3(p, q, r ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	a, e := m.proofTransitivity(p, q)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofTransitivity(a, r)
}
func (m *ASTManager) MkProofTransitivity4(p, q, r, s ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	a, e := m.proofTransitivity(p, q)
	if e != nil {
		return ASTHandle{}, e
	}
	a, e = m.proofTransitivity(a, r)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofTransitivity(a, s)
}
func (m *ASTManager) MkProofModusPonens(p, q ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if p.IsNull() || q.IsNull() {
		return ASTHandle{}, nil
	}
	traceEnabled := m.proofModusPonensTraceEnabled()
	var f, g ASTHandle
	var e error
	if ASTSourceDebug || traceEnabled {
		f, e = m.proofFact(p)
		if e != nil {
			return ASTHandle{}, e
		}
		g, e = m.proofFact(q)
		if e != nil {
			return ASTHandle{}, e
		}
	}
	if ASTSourceDebug || traceEnabled {
		valid := false
		for _, k := range [...]int32{ASTOpImplies, ASTOpIff, ASTOpOEq} {
			yes, e := m.proofKind(g, k)
			if e != nil {
				return ASTHandle{}, e
			}
			valid = valid || yes
		}
		if !valid && traceEnabled {
			if e = m.traceProofModusPonensBadFact(p, q); e != nil {
				return ASTHandle{}, e
			}
		}
		if ASTSourceDebug && !valid {
			return ASTHandle{}, ErrASTDomain
		}
	}
	if ASTSourceDebug || traceEnabled {
		a, e := m.proofArg(g, 0)
		if e != nil {
			return ASTHandle{}, e
		}
		if a != f && traceEnabled {
			m.mu.Unlock()
			traceErr := m.traceProofModusPonensSMT2(2645, "", f, g)
			m.mu.Lock()
			if traceErr != nil {
				return ASTHandle{}, traceErr
			}
		}
		if ASTSourceDebug && a != f {
			return ASTHandle{}, ErrASTDomain
		}
	}
	if traceEnabled {
		for _, site := range [...]struct {
			proof ASTHandle
			line  int
		}{{q, 2648}, {p, 2649}} {
			nonground, err := m.proofModusPonensNonground(site.proof)
			if err != nil {
				return ASTHandle{}, err
			}
			if nonground {
				m.mu.Unlock()
				traceErr := m.traceProofModusPonensSMT2(site.line, "Non-ground: ", site.proof)
				m.mu.Lock()
				if traceErr != nil {
					return ASTHandle{}, traceErr
				}
			}
		}
	}
	if yes, e := m.proofKind(q, ASTPrReflexivity); e != nil {
		return ASTHandle{}, e
	} else if yes {
		return p, nil
	}
	if !ASTSourceDebug {
		g, e = m.proofFact(q)
		if e != nil {
			return ASTHandle{}, e
		}
	}
	fact, e := m.proofArg(g, 1)
	if e != nil {
		return ASTHandle{}, e
	}
	k := ASTPrModusPonens
	if yes, e := m.proofKind(g, ASTOpOEq); e != nil {
		return ASTHandle{}, e
	} else if yes {
		k = ASTPrModusPonensOEq
	}
	return m.proofMake(k, p, q, fact)
}
