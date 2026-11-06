package ast

// Checked scalar replacements for borrowed ast.h field accessors.

import (
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (m *ASTManager) GetDecl(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTApp)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.external(m.appAt(r.slot).decl), nil
}
func (m *ASTManager) GetArity(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTFuncDecl)
	if e != nil {
		return 0, e
	}
	return m.declAt(r.slot).arity, nil
}
func (m *ASTManager) GetDomain(h ASTHandle, i uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTFuncDecl)
	if e != nil {
		return ASTHandle{}, e
	}
	v := m.declAt(r.slot)
	if i >= v.arity {
		return ASTHandle{}, ErrASTDomain
	}
	r, e = m.readInternal(v.domain, i)
	return m.external(r), e
}
func (m *ASTManager) GetRange(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTFuncDecl)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.external(m.declAt(r.slot).rangeSort), nil
}
func (m *ASTManager) GetNumArgs(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTApp)
	if e != nil {
		return 0, e
	}
	return m.appAt(r.slot).count, nil
}
func (m *ASTManager) IsGround(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	return m.nodeAt(r.slot).kind == ASTApp && m.appAt(r.slot).ground, nil
}
func (m *ASTManager) HasQuantifiers(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	kind := m.nodeAt(r.slot).kind
	if kind == ASTQuantifier {
		return true, nil
	}
	return kind == ASTApp && m.appAt(r.slot).quantifiers, nil
}
func (m *ASTManager) SortIsInfinite(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTSort)
	if e != nil {
		return false, e
	}
	p := m.sortAt(r.slot).info
	if p == (ASTSortInfo{}) {
		return true, nil
	}
	v, e := m.sortInfo(p)
	if e != nil {
		return false, e
	}
	return v.size.IsInfinite(), nil
}
func (m *ASTManager) SortIsVeryBig(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTSort)
	if e != nil {
		return false, e
	}
	p := m.sortAt(r.slot).info
	if p == (ASTSortInfo{}) {
		return true, nil
	}
	v, e := m.sortInfo(p)
	if e != nil {
		return false, e
	}
	return v.size.IsVeryBig(), nil
}
func (m *ASTManager) GetASTName(h ASTHandle) (usymbol.Symbol, error) {
	if e := m.lock(); e != nil {
		return usymbol.Symbol{}, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return usymbol.Symbol{}, e
	}
	switch m.nodeAt(r.slot).kind {
	case ASTSort:
		return m.sortAt(r.slot).name, nil
	case ASTFuncDecl:
		return m.declAt(r.slot).name, nil
	}
	return usymbol.Symbol{}, ErrASTDomain
}
func (m *ASTManager) GetQuantifierBody(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.external(m.quantifierAt(r.slot).body), nil
}
func (m *ASTManager) GetNumDeclarations(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return 0, e
	}
	return m.quantifierAt(r.slot).count, nil
}
func (m *ASTManager) GetDeclarationSort(h ASTHandle, i uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return ASTHandle{}, e
	}
	v := m.quantifierAt(r.slot)
	if i >= v.count {
		return ASTHandle{}, ErrASTDomain
	}
	r, e = m.readInternal(v.sorts, i)
	return m.external(r), e
}
func (m *ASTManager) GetDeclarationName(h ASTHandle, i uint32) (usymbol.Symbol, error) {
	if e := m.lock(); e != nil {
		return usymbol.Symbol{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return usymbol.Symbol{}, e
	}
	v := m.quantifierAt(r.slot)
	if i >= v.count {
		return usymbol.Symbol{}, ErrASTDomain
	}
	return m.readSymbol(v.names, i)
}
func (m *ASTManager) GetQuantifierWeight(h ASTHandle) (int32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return 0, e
	}
	return m.quantifierAt(r.slot).weight, nil
}
func (m *ASTManager) IsForall(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return false, e
	}
	return m.quantifierAt(r.slot).forall, nil
}
func (m *ASTManager) GetQuantifierIDs(h ASTHandle) (usymbol.Symbol, usymbol.Symbol, error) {
	if e := m.lock(); e != nil {
		return usymbol.Symbol{}, usymbol.Symbol{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return usymbol.Symbol{}, usymbol.Symbol{}, e
	}
	v := m.quantifierAt(r.slot)
	return v.qid, v.skolemID, nil
}
func (m *ASTManager) GetNumPatterns(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return 0, e
	}
	return m.quantifierAt(r.slot).patternCount, nil
}
func (m *ASTManager) GetPattern(h ASTHandle, i uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return ASTHandle{}, e
	}
	v := m.quantifierAt(r.slot)
	if i >= v.patternCount {
		return ASTHandle{}, ErrASTDomain
	}
	r, e = m.readInternal(v.patterns, i)
	return m.external(r), e
}
func (m *ASTManager) GetNumNoPatterns(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return 0, e
	}
	return m.quantifierAt(r.slot).noPatternCount, nil
}
func (m *ASTManager) GetNoPattern(h ASTHandle, i uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTQuantifier)
	if e != nil {
		return ASTHandle{}, e
	}
	v := m.quantifierAt(r.slot)
	if i >= v.noPatternCount {
		return ASTHandle{}, ErrASTDomain
	}
	r, e = m.readInternal(v.noPatterns, i)
	return m.external(r), e
}
