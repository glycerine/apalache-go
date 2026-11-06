package ast

// Actual label, pattern, model-value, and user-sort decl plugins in ast.cpp.

import (
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (m *ASTManager) pluginDecl(name usymbol.Symbol, fid FamilyID, k int32, ps []ASTParameter, domain []ASTHandle, result astNodeRef, flags ASTFuncFlags, private bool) (astNodeRef, error) {
	s, g, e := m.newInfo(false, fid, k, ps, SortSize{}, flags, private)
	if e != nil {
		return astNodeRef{}, e
	}
	// Stack func_decl_info is destroyed after the factory expression, including
	// its owning rational parameter copies; unique registration has its own copy.
	defer m.dropInfoStorage(s, g)
	return m.mkFuncDecl(name, domain, m.external(result), ASTFuncDeclInfo{m.origin, s, g})
}
func (m *ASTManager) builtinDecl(fid FamilyID, k int32, ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astArithmeticPlugin {
		return m.arithmeticDecl(k, ps, domain)
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astBitVectorPlugin {
		return m.bitVectorDecl(k, ps, domain)
	}
	switch fid {
	case 1:
		if k == 0 {
			if len(domain) != 1 || len(ps) < 2 {
				return astNodeRef{}, ErrASTDomain
			}
			first, e := m.parameter(ps[0])
			if e != nil {
				return astNodeRef{}, e
			}
			if first.Kind != ASTParameterInt {
				return astNodeRef{}, ErrASTDomain
			}
			for _, p := range ps[1:] {
				v, e := m.parameter(p)
				if e != nil {
					return astNodeRef{}, e
				}
				if v.Kind != ASTParameterSymbol {
					return astNodeRef{}, ErrASTDomain
				}
			}
			r, e := m.requireKind(domain[0], ASTSort)
			if e != nil {
				return astNodeRef{}, e
			}
			if r != m.boolSort {
				return astNodeRef{}, ErrASTDomain
			}
			name := m.builtin.labelNeg
			if first.integer != 0 {
				name = m.builtin.labelPos
			}
			return m.pluginDecl(name, fid, 0, ps, domain, r, ASTFuncFlags{}, false)
		}
		if ASTSourceDebug && k != 1 {
			return astNodeRef{}, ErrASTDomain
		}
		if len(domain) != 0 {
			return astNodeRef{}, ErrASTDomain
		}
		for _, p := range ps {
			v, e := m.parameter(p)
			if e != nil {
				return astNodeRef{}, e
			}
			if v.Kind != ASTParameterSymbol {
				return astNodeRef{}, ErrASTDomain
			}
		}
		return m.pluginDecl(m.builtin.labelLit, fid, 1, ps, nil, m.boolSort, ASTFuncFlags{}, false)
	case 2:
		name, e := m.Symbols.Intern("pattern")
		if e != nil {
			return astNodeRef{}, e
		}
		return m.pluginDecl(name, fid, 0, nil, domain, m.boolSort, ASTFuncFlags{}, false)
	case 3:
		if ASTSourceDebug && k != 0 {
			return astNodeRef{}, ErrASTDomain
		}
		if len(domain) != 0 || len(ps) != 2 {
			return astNodeRef{}, ErrASTDomain
		}
		index, e := m.parameter(ps[0])
		if e != nil {
			return astNodeRef{}, e
		}
		value, e := m.parameter(ps[1])
		if e != nil {
			return astNodeRef{}, e
		}
		if index.Kind != ASTParameterInt || value.Kind != ASTParameterAST {
			return astNodeRef{}, ErrASTDomain
		}
		r := value.ast
		if e = m.validNode(r); e != nil {
			return astNodeRef{}, e
		}
		if m.nodeAt(r.slot).kind != ASTSort {
			return astNodeRef{}, ErrASTDomain
		}
		name, e := m.modelValueName(m.sortAt(r.slot).name, index.integer)
		if e != nil {
			return astNodeRef{}, e
		}
		return m.pluginDecl(name, fid, k, ps, nil, r, ASTFuncFlags{}, true)
	default:
		return astNodeRef{}, ErrASTDomain
	}
}
func (m *ASTManager) registerUserName(name usymbol.Symbol) (int32, error) {
	return astRegisterUserName(&m.builtin.userTable, &m.builtin.userNames, name)
}
func (m *ASTManager) userSort(k int32, ps []ASTParameter) (astNodeRef, error) {
	if k < 0 || uint64(k) >= uint64(len(m.builtin.userNames)) {
		return astNodeRef{}, ErrASTDomain
	}
	s, g, e := m.newInfo(true, 4, k, ps, InfiniteSortSize(), ASTFuncFlags{}, false)
	if e != nil {
		return astNodeRef{}, e
	}
	defer m.dropInfoStorage(s, g)
	return m.mkSort(m.builtin.userNames[k], ASTSortInfo{m.origin, s, g})
}
func (m *ASTManager) MkUninterpretedSort(name usymbol.Symbol, ps []ASTParameter) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	k, e := m.registerUserName(name)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e := m.userSort(k, ps)
	return m.external(r), e
}

// IsUninterpretedSort follows ast_manager::is_uninterp(sort*): both a
// declaration without a family and a user-sort declaration are uninterpreted.
func (m *ASTManager) IsUninterpretedSort(sort ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return false, err
	}
	family, _, err := m.declIdentity(r)
	return family == NullFamilyID || family == 4, err
}
