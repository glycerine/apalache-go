package ast

// Source: src/ast/ast.cpp:2265-2309; src/util/buffer.h.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

// MkLabel builds the retained label specialization with native parameter slices.
func (m *ASTManager) MkLabel(pos bool, names []usymbol.Symbol, body ASTHandle) (out ASTHandle, err error) {
	if err = m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	if len(names) == 0 {
		return ASTHandle{}, ErrASTDomain
	}
	if ASTSourceDebug {
		bodyRef, e := m.internal(body)
		if e != nil {
			return ASTHandle{}, e
		}
		sort, e := m.sortOf(bodyRef)
		if e != nil {
			return ASTHandle{}, e
		}
		if sort != m.boolSort {
			return ASTHandle{}, ErrASTDomain
		}
	}
	ps := make([]ASTParameter, 0, len(names)+1)
	defer func() {
		for i := len(ps); i > 0; i-- {
			if e := m.dropParameter(ps[i-1]); err == nil {
				err = e
			}
		}
	}()
	n := int32(0)
	if pos {
		n = 1
	}
	p, e := m.newParameterInt(n)
	if e != nil {
		return ASTHandle{}, e
	}
	ps = append(ps, p)
	for _, name := range names {
		p, e = m.newParameterSymbol(name)
		if e != nil {
			return ASTHandle{}, e
		}
		ps = append(ps, p)
	}
	r, e := m.mkFamilyApp(1, 0, ps, []ASTHandle{body})
	return m.external(r), e
}

// MkLabelLit builds the zero-argument label literal from its symbol names.
func (m *ASTManager) MkLabelLit(names []usymbol.Symbol) (out ASTHandle, err error) {
	if err = m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	if len(names) == 0 && ASTSourceDebug {
		return ASTHandle{}, ErrASTDomain
	}
	ps := make([]ASTParameter, 0, len(names))
	defer func() {
		for i := len(ps); i > 0; i-- {
			if e := m.dropParameter(ps[i-1]); err == nil {
				err = e
			}
		}
	}()
	for _, name := range names {
		p, e := m.newParameterSymbol(name)
		if e != nil {
			return ASTHandle{}, e
		}
		ps = append(ps, p)
	}
	r, e := m.mkFamilyApp(1, 1, ps, nil)
	return m.external(r), e
}

func (m *ASTManager) readLabel(h ASTHandle, kind int32, names *[]usymbol.Symbol) (bool, bool, error) {
	if names == nil {
		return false, false, ErrASTDomain
	}
	r, e := m.internal(h)
	if e != nil {
		return false, false, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, false, nil
	}
	decl := m.appAt(r.slot).decl
	fid, k, e := m.declIdentity(decl)
	if e != nil || fid != 1 || k != kind {
		return false, false, e
	}
	info, e := m.funcInfo(m.declAt(decl.slot).info)
	if e != nil {
		return false, false, e
	}
	start := uint32(0)
	pos := false
	if kind == 0 {
		p, e := m.infoParameter(info, 0)
		if e != nil {
			return false, false, e
		}
		v, e := m.parameter(p)
		if e != nil {
			return false, false, e
		}
		pos = v.integer != 0
		start = 1
	}
	for i := start; i < info.count; i++ {
		p, e := m.infoParameter(info, i)
		if e != nil {
			return false, false, e
		}
		v, e := m.parameter(p)
		if e != nil {
			return false, false, e
		}
		*names = append(*names, v.symbol)
	}
	return true, pos, nil
}

// IsLabel appends names on a match, preserving both outputs on a miss.
func (m *ASTManager) IsLabel(h ASTHandle, pos *bool, names *[]usymbol.Symbol) (bool, error) {
	if pos == nil {
		return false, ErrASTDomain
	}
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	yes, value, e := m.readLabel(h, 0, names)
	if yes && e == nil {
		*pos = value
	}
	return yes, e
}

// IsLabelLit appends names on a match and leaves the caller slice on a miss.
func (m *ASTManager) IsLabelLit(h ASTHandle, names *[]usymbol.Symbol) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	yes, _, e := m.readLabel(h, 1, names)
	return yes, e
}
