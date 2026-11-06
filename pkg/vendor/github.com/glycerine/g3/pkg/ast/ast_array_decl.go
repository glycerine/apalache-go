package ast

// Source: src/ast/array_decl_plugin.h:41-61;
// src/ast/array_decl_plugin.cpp:120-137,235-306,446-465,498-512;
// src/ast/ast.cpp:2030-2036.

const (
	ASTArrayOpStore int32 = iota
	ASTArrayOpSelect
	ASTArrayOpConst
	ASTArrayOpExt
	ASTArrayOpDefault
	ASTArrayOpMap
	ASTArrayOpSetUnion
	ASTArrayOpSetIntersect
	ASTArrayOpSetDifference
	ASTArrayOpSetComplement
	ASTArrayOpSetSubset
	ASTArrayOpAsArray
	ASTArrayLastOp
)

func (m *ASTManager) arraySortParameter(info *astInfoRecord, i uint32) (astNodeRef, error) {
	p, e := m.infoParameter(info, i)
	if e != nil {
		return astNodeRef{}, e
	}
	record, e := m.parameter(p)
	if e != nil {
		return astNodeRef{}, e
	}
	if record.Kind != ASTParameterAST {
		return astNodeRef{}, ErrASTDomain
	}
	return m.requireKind(m.external(record.ast), ASTSort)
}
func (m *ASTManager) arrayCompatibleSorts(a, b astNodeRef) (bool, error) {
	if a == b {
		return true, nil
	}
	if !m.coercions || m.arithmeticPlugin == nil {
		return false, nil
	}
	af, _, e := m.declIdentity(a)
	if e != nil {
		return false, e
	}
	bf, _, e := m.declIdentity(b)
	if e != nil {
		return false, e
	}
	return af == m.arithmeticPlugin.family && bf == m.arithmeticPlugin.family, nil
}
func (m *ASTManager) arrayDecl(k int32, ps []ASTParameter, domain []ASTHandle, rangeSort ASTHandle) (astNodeRef, error) {
	p := m.arrayPlugin
	if p == nil {
		return astNodeRef{}, ErrASTDomain
	}
	if k == ASTArrayOpAsArray {
		return m.arrayAsArrayDecl(ps)
	}
	if k == ASTArrayOpMap {
		return m.arrayMapDecl(ps, domain)
	}
	if k == ASTArrayOpDefault {
		return m.arrayDefaultDecl(domain)
	}
	if k == ASTArrayOpExt {
		return m.arrayExtDecl(ps, domain)
	}
	if k >= ASTArrayOpSetUnion && k <= ASTArrayOpSetSubset {
		return m.arraySetDecl(k, domain)
	}
	if k == ASTArrayOpConst {
		var sort astNodeRef
		if len(ps) == 1 {
			param, e := m.parameter(ps[0])
			if e != nil {
				return astNodeRef{}, e
			}
			if param.Kind == ASTParameterAST {
				r, e := m.internal(m.external(param.ast))
				if e != nil {
					return astNodeRef{}, e
				}
				if m.nodeAt(r.slot).kind == ASTSort {
					sort = r
				}
			}
		}
		if sort.slot == 0 && !rangeSort.IsNull() {
			var e error
			sort, e = m.requireKind(rangeSort, ASTSort)
			if e != nil {
				return astNodeRef{}, e
			}
		}
		if len(domain) != 1 || sort.slot == 0 {
			return astNodeRef{}, ErrASTDomain
		}
		fid, kind, e := m.declIdentity(sort)
		if e != nil {
			return astNodeRef{}, e
		}
		if fid != p.family || kind != 0 {
			return astNodeRef{}, ErrASTDomain
		}
		info, e := m.sortInfo(m.sortAt(sort.slot).info)
		if e != nil {
			return astNodeRef{}, e
		}
		result, e := m.arraySortParameter(info, info.count-1)
		if e != nil {
			return astNodeRef{}, e
		}
		given, e := m.requireKind(domain[0], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		compatible, e := m.arrayCompatibleSorts(result, given)
		if e != nil {
			return astNodeRef{}, e
		}
		if !compatible {
			return astNodeRef{}, ErrASTDomain
		}
		parameter, e := m.allocParameter()
		if e != nil {
			return astNodeRef{}, e
		}
		m.parameterAt(parameter.slot).Kind = ASTParameterAST
		m.parameterAt(parameter.slot).ast = sort
		defer m.dropParameter(parameter)
		return m.pluginDecl(p.names[2], p.family, k, []ASTParameter{parameter}, domain, sort, ASTFuncFlags{}, true)
	}
	if k != ASTArrayOpSelect && k != ASTArrayOpStore {
		return astNodeRef{}, ErrASTDomain
	}
	if len(domain) < 2 || k == ASTArrayOpStore && len(domain) < 3 {
		return astNodeRef{}, ErrASTDomain
	}
	sort, e := m.requireKind(domain[0], ASTSort)
	if e != nil {
		return astNodeRef{}, e
	}
	if k == ASTArrayOpStore {
		fid, kind, e := m.declIdentity(sort)
		if e != nil {
			return astNodeRef{}, e
		}
		if fid != p.family || kind != 0 {
			return astNodeRef{}, ErrASTDomain
		}
	}
	info, e := m.sortInfo(m.sortAt(sort.slot).info)
	if e != nil {
		return astNodeRef{}, e
	}
	expected := uint64(info.count)
	if k == ASTArrayOpStore {
		expected++
	}
	if uint64(len(domain)) != expected {
		return astNodeRef{}, ErrASTDomain
	}
	normalized := []ASTHandle{domain[0]}
	count := info.count
	if k == ASTArrayOpSelect {
		count--
	}
	for i := uint32(0); i < count; i++ {
		expected, e := m.arraySortParameter(info, i)
		if e != nil {
			return astNodeRef{}, e
		}
		actual, e := m.requireKind(domain[i+1], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		compatible, e := m.arrayCompatibleSorts(expected, actual)
		if e != nil {
			return astNodeRef{}, e
		}
		if !compatible {
			return astNodeRef{}, ErrASTDomain
		}
		normalized = append(normalized, m.external(expected))
	}
	result := sort
	name := p.names[0]
	if k == ASTArrayOpSelect {
		result, e = m.arraySortParameter(info, info.count-1)
		if e != nil {
			return astNodeRef{}, e
		}
		name = p.names[1]
	}
	return m.pluginDecl(name, p.family, k, nil, normalized, result, ASTFuncFlags{}, false)
}
