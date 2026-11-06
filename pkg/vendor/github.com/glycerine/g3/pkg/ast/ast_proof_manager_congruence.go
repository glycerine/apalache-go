package ast

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (m *ASTManager) MkProofTransitivityArray(ps []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if len(ps) == 0 {
		if ASTSourceDebug {
			return ASTHandle{}, ErrASTDomain
		}
		return ASTHandle{}, arena.ErrBounds
	}
	r := ps[0]
	for _, p := range ps[1:] {
		var e error
		r, e = m.proofTransitivity(r, p)
		if e != nil {
			return ASTHandle{}, e
		}
	}
	return r, nil
}
func (m *ASTManager) MkProofTransitivityStar(ps []ASTHandle, a, b ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if len(ps) == 0 {
		return ASTHandle{}, nil
	}
	if len(ps) == 1 {
		return ps[0], nil
	}
	if ASTSourceDebug {
		for _, p := range ps {
			if p.IsNull() {
				return ASTHandle{}, ErrASTDomain
			}
			yes, e := m.proofKind(p, ASTPrReflexivity)
			if e != nil {
				return ASTHandle{}, e
			}
			if yes {
				return ASTHandle{}, ErrASTDomain
			}
		}
	}
	var args []ASTHandle
	args = append(args, ps...)
	f, e := m.proofEq(a, b, false)
	if e != nil {
		return ASTHandle{}, e
	}
	args = append(args, f)
	return m.proofMake(ASTPrTransitivityStar, args...)
}
func (m *ASTManager) proofMonotonicity(d astNodeRef, a, b ASTHandle, ps []ASTHandle) (ASTHandle, error) {
	// Source only inspects f1/f2 here in SASSERT expressions.
	if ASTSourceDebug {
		x, e := m.requireKind(a, ASTApp)
		if e != nil {
			return ASTHandle{}, e
		}
		y, e := m.requireKind(b, ASTApp)
		if e != nil {
			return ASTHandle{}, e
		}
		p, q := m.appAt(x.slot), m.appAt(y.slot)
		if p.count != q.count || p.decl != q.decl {
			return ASTHandle{}, ErrASTDomain
		}
	}
	var args []ASTHandle
	args = append(args, ps...)
	pair := [2]ASTHandle{a, b}
	f, e := m.mkApp(d, pair[:])
	if e != nil {
		return ASTHandle{}, e
	}
	args = append(args, m.external(f))
	return m.proofMake(ASTPrMonotonicity, args...)
}
func (m *ASTManager) MkProofMonotonicity(relation, a, b ASTHandle, ps []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	d, e := m.requireKind(relation, ASTFuncDecl)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMonotonicity(d, a, b, ps)
}
func (m *ASTManager) proofCongruence(a, b ASTHandle, ps []ASTHandle, oeq bool) (ASTHandle, error) {
	x, e := m.requireKind(a, ASTApp)
	if e != nil {
		return ASTHandle{}, e
	}
	s, e := m.sortOf(x)
	if e != nil {
		return ASTHandle{}, e
	}
	if ASTSourceDebug {
		y, e := m.requireKind(b, ASTApp)
		if e != nil {
			return ASTHandle{}, e
		}
		t, e := m.sortOf(y)
		if e != nil {
			return ASTHandle{}, e
		}
		if s != t {
			return ASTHandle{}, ErrASTDomain
		}
	}
	k := ASTOpOEq
	if !oeq {
		k = ASTOpEq
		if s == m.boolSort {
			k = ASTOpIff
		}
	}
	domain := [2]ASTHandle{m.external(s), m.external(s)}
	d, e := m.basicDecl(k, nil, domain[:])
	if e != nil {
		return ASTHandle{}, e
	}
	return m.proofMonotonicity(d, a, b, ps)
}
func (m *ASTManager) MkProofCongruence(a, b ASTHandle, ps []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofCongruence(a, b, ps, false)
}
func (m *ASTManager) MkProofOEqCongruence(a, b ASTHandle, ps []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofCongruence(a, b, ps, true)
}
