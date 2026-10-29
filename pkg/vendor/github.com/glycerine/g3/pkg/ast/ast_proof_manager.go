package ast

// Concrete ast_manager proof construction/inspection from ast.h and ast.cpp.

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (m *ASTManager) ProofMode() (ASTProofMode, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	return m.proofMode, nil
}
func (m *ASTManager) ProofsEnabled() (bool, error) {
	v, e := m.ProofMode()
	return v != ASTProofDisabled, e
}
func (m *ASTManager) ProofsDisabled() (bool, error) {
	v, e := m.ProofMode()
	return v == ASTProofDisabled, e
}
func (m *ASTManager) ToggleProofMode(v ASTProofMode) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if v > ASTProofEnabled {
		return ErrASTDomain
	}
	m.proofMode = v
	return nil
}
func (m *ASTManager) UndefProof() (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.external(m.undefProof), nil
}
func (m *ASTManager) proofCore(fid FamilyID, k int32, args []ASTHandle) (astNodeRef, error) {
	if m.proofMode == ASTProofDisabled {
		return astNodeRef{}, nil
	}
	return m.mkFamilyApp(fid, k, nil, args)
}
func (m *ASTManager) proofThree(fid FamilyID, k int32, a, b, c ASTHandle) (astNodeRef, error) {
	args := [3]ASTHandle{a, b, c}
	return m.proofCore(fid, k, args[:2])
}
func (m *ASTManager) proofKind(h ASTHandle, k int32) (bool, error) {
	if h.IsNull() {
		return false, nil
	}
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	fid, kind, e := m.declIdentity(m.appAt(r.slot).decl)
	return fid == 0 && kind == k, e
}
func (m *ASTManager) proofIs(h ASTHandle) (bool, error) {
	if h.IsNull() {
		return false, nil
	}
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	return m.declAt(m.appAt(r.slot).decl.slot).rangeSort == m.proofSort, nil
}
func (m *ASTManager) proofAppNode(h ASTHandle) (astNodeRef, error) {
	r, e := m.requireKind(h, ASTApp)
	if e != nil {
		return r, e
	}
	if ASTSourceDebug {
		yes, e := m.proofIs(h)
		if e != nil {
			return r, e
		}
		if !yes {
			return r, ErrASTDomain
		}
	}
	return r, nil
}
func (m *ASTManager) proofHasFact(h ASTHandle) (bool, error) {
	r, e := m.proofAppNode(h)
	if e != nil {
		return false, e
	}
	a := m.appAt(r.slot)
	if a.count == 0 {
		return false, nil
	}
	last, e := m.readInternal(a.args, a.count-1)
	if e != nil {
		return false, e
	}
	s, e := m.sortOf(last)
	return s != m.proofSort, e
}
func (m *ASTManager) proofFact(h ASTHandle) (ASTHandle, error) {
	r, e := m.proofAppNode(h)
	if e != nil {
		return ASTHandle{}, e
	}
	if ASTSourceDebug {
		yes, e := m.proofHasFact(h)
		if e != nil {
			return ASTHandle{}, e
		}
		if !yes {
			return ASTHandle{}, ErrASTDomain
		}
	}
	a := m.appAt(r.slot)
	if a.count == 0 {
		return ASTHandle{}, arena.ErrBounds
	}
	last, e := m.readInternal(a.args, a.count-1)
	return m.external(last), e
}
func (m *ASTManager) proofArg(h ASTHandle, i uint32) (ASTHandle, error) {
	r, e := m.requireKind(h, ASTApp)
	if e != nil {
		return ASTHandle{}, e
	}
	a := m.appAt(r.slot)
	if i >= a.count {
		return ASTHandle{}, arena.ErrBounds
	}
	x, e := m.readInternal(a.args, i)
	return m.external(x), e
}
func (m *ASTManager) proofBool(h ASTHandle) (bool, error) {
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	s, e := m.sortOf(r)
	return s == m.boolSort, e
}
func (m *ASTManager) proofCheckBool(h ASTHandle) error {
	if !ASTSourceDebug {
		return nil
	}
	yes, e := m.proofBool(h)
	if e != nil {
		return e
	}
	if !yes {
		return ErrASTDomain
	}
	return nil
}
func (m *ASTManager) proofEq(a, b ASTHandle, oeq bool) (ASTHandle, error) {
	k := ASTOpOEq
	if !oeq {
		yes, e := m.proofBool(a)
		if e != nil {
			return ASTHandle{}, e
		}
		k = ASTOpEq
		if yes {
			k = ASTOpIff
		}
	}
	args := [2]ASTHandle{a, b}
	r, e := m.mkFamilyApp(0, k, nil, args[:])
	return m.external(r), e
}
func (m *ASTManager) proofMake(k int32, args ...ASTHandle) (ASTHandle, error) {
	r, e := m.mkFamilyApp(0, k, nil, args)
	return m.external(r), e
}
func (m *ASTManager) IsProof(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.proofIs(h)
}
func (m *ASTManager) IsProofKind(h ASTHandle, k int32) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.proofKind(h, k)
}
func (m *ASTManager) IsUndefProof(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return h == m.external(m.undefProof), nil
}
func (m *ASTManager) ProofHasFact(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.proofHasFact(h)
}
func (m *ASTManager) GetProofFact(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.proofFact(h)
}
func (m *ASTManager) GetProofNumParents(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.proofAppNode(h)
	if e != nil {
		return 0, e
	}
	yes, e := m.proofHasFact(h)
	n := m.appAt(r.slot).count
	if yes {
		n--
	}
	return n, e
}
func (m *ASTManager) GetProofParent(h ASTHandle, i uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if _, e := m.proofAppNode(h); e != nil {
		return ASTHandle{}, e
	}
	a, e := m.proofArg(h, i)
	if e != nil {
		return ASTHandle{}, e
	}
	if _, e = m.requireKind(a, ASTApp); e != nil {
		return ASTHandle{}, e
	}
	return a, nil
}
