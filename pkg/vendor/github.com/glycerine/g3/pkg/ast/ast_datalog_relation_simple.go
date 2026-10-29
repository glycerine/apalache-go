package ast

// Source: src/ast/dl_decl_plugin.cpp:166-208,252-264,404-410,447-454,
// 472-492,494-604,746-754.

import "errors"

func (m *ASTManager) datalogRelationColumnsRef(sort astNodeRef) ([]astNodeRef, error) {
	if e := m.validNode(sort); e != nil || m.nodeAt(sort.slot).kind != ASTSort {
		return nil, ErrASTDomain
	}
	infoRef := m.sortAt(sort.slot).info
	if infoRef == (ASTSortInfo{}) {
		return nil, ErrASTDomain
	}
	info, e := m.sortInfo(infoRef)
	if e != nil {
		return nil, e
	}
	p := m.auxiliaryPlugin(astDatalogPlugin)
	if p == nil || info.family != p.family || info.kind != DatalogRelationSortKind {
		return nil, ErrASTDomain
	}
	columns := make([]astNodeRef, info.count)
	for i := range columns {
		parameter, e := m.infoParameter(info, uint32(i))
		if e != nil {
			return nil, e
		}
		value, e := m.parameter(parameter)
		if e != nil || value.Kind != ASTParameterAST || m.validNode(value.ast) != nil || m.nodeAt(value.ast.slot).kind != ASTSort {
			return nil, ErrASTDomain
		}
		columns[i] = value.ast
	}
	return columns, nil
}

func (m *ASTManager) datalogStoreSelectDecl(kind int32, ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	if len(ps) != 0 || len(domain) < 1 {
		return astNodeRef{}, ErrASTDomain
	}
	p := m.auxiliaryPlugin(astDatalogPlugin)
	if p == nil {
		return astNodeRef{}, ErrASTDomain
	}
	result, e := m.requireKind(domain[0], ASTSort)
	if e != nil {
		return astNodeRef{}, e
	}
	// The source assigns Bool to r before is_rel_sort for OP_RA_SELECT, so that
	// operation is rejected by its own relation-sort guard.
	if kind == DatalogOpSelect {
		return astNodeRef{}, ErrASTDomain
	}
	columns, e := m.datalogRelationColumnsRef(result)
	if e != nil || len(domain) != len(columns)+1 {
		return astNodeRef{}, ErrASTDomain
	}
	for i, column := range columns {
		actual, e := m.requireKind(domain[i+1], ASTSort)
		if e != nil || actual != column {
			return astNodeRef{}, ErrASTDomain
		}
	}
	return m.pluginDecl(p.symbols[DatalogOpStore], p.family, kind, nil, domain, result, ASTFuncFlags{}, false)
}

func (m *ASTManager) datalogEmptyDecl(ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	if len(ps) != 1 || len(domain) != 0 {
		return astNodeRef{}, ErrASTDomain
	}
	value, e := m.parameter(ps[0])
	if e != nil || value.Kind != ASTParameterAST {
		return astNodeRef{}, ErrASTDomain
	}
	if _, e = m.datalogRelationColumnsRef(value.ast); e != nil {
		return astNodeRef{}, e
	}
	p := m.auxiliaryPlugin(astDatalogPlugin)
	if p == nil {
		return astNodeRef{}, ErrASTDomain
	}
	return m.pluginDecl(p.symbols[DatalogOpEmpty], p.family, DatalogOpEmpty, ps, nil, value.ast, ASTFuncFlags{}, false)
}

func (m *ASTManager) datalogSimpleRelationDecl(kind int32, ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	if len(ps) != 0 {
		return astNodeRef{}, ErrASTDomain
	}
	p := m.auxiliaryPlugin(astDatalogPlugin)
	if p == nil {
		return astNodeRef{}, ErrASTDomain
	}
	var result astNodeRef
	switch kind {
	case DatalogOpUnion, DatalogOpWiden:
		if len(domain) != 2 || domain[0] != domain[1] {
			return astNodeRef{}, ErrASTDomain
		}
		var e error
		result, e = m.requireKind(domain[0], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		if _, e := m.datalogRelationColumnsRef(result); e != nil {
			return astNodeRef{}, e
		}
		// mk_unionw uses m_union_sym for both operation kinds.
		return m.pluginDecl(p.symbols[DatalogOpUnion], p.family, kind, nil, domain, result, ASTFuncFlags{}, false)
	case DatalogOpIsEmpty:
		if len(domain) != 1 {
			return astNodeRef{}, ErrASTDomain
		}
		var e error
		result, e = m.requireKind(domain[0], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		if _, e := m.datalogRelationColumnsRef(result); e != nil {
			return astNodeRef{}, e
		}
		return m.pluginDecl(p.symbols[kind], p.family, kind, nil, domain, m.basic.boolSort, ASTFuncFlags{}, false)
	case DatalogOpComplement, DatalogOpClone:
		if len(domain) != 1 {
			return astNodeRef{}, ErrASTDomain
		}
		var e error
		result, e = m.requireKind(domain[0], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		if _, e := m.datalogRelationColumnsRef(result); e != nil {
			return astNodeRef{}, e
		}
		return m.pluginDecl(p.symbols[kind], p.family, kind, nil, domain, result, ASTFuncFlags{}, false)
	default:
		return astNodeRef{}, ErrASTDomain
	}
}

func (m *ASTManager) MkDatalogEmpty(relation ASTHandle) (result ASTHandle, err error) {
	family, e := m.RegisterDatalogPlugin()
	if e != nil {
		return result, e
	}
	parameter, e := m.NewParameterAST(relation)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.DelParameter(&parameter)) }()
	return m.MkFamilyApp(family, DatalogOpEmpty, []ASTParameter{parameter}, nil)
}

func (m *ASTManager) mkDatalogRelationApp(kind int32, args []ASTHandle) (ASTHandle, error) {
	family, e := m.RegisterDatalogPlugin()
	if e != nil {
		return ASTHandle{}, e
	}
	return m.MkFamilyApp(family, kind, nil, args)
}

func (m *ASTManager) MkDatalogStore(relation ASTHandle, values ...ASTHandle) (ASTHandle, error) {
	args := make([]ASTHandle, 1, len(values)+1)
	args[0] = relation
	args = append(args, values...)
	return m.mkDatalogRelationApp(DatalogOpStore, args)
}

func (m *ASTManager) MkDatalogSelect(relation ASTHandle, values ...ASTHandle) (ASTHandle, error) {
	args := make([]ASTHandle, 1, len(values)+1)
	args[0] = relation
	args = append(args, values...)
	return m.mkDatalogRelationApp(DatalogOpSelect, args)
}

func (m *ASTManager) MkDatalogIsEmpty(relation ASTHandle) (ASTHandle, error) {
	return m.mkDatalogRelationApp(DatalogOpIsEmpty, []ASTHandle{relation})
}

func (m *ASTManager) MkDatalogUnion(left, right ASTHandle) (ASTHandle, error) {
	return m.mkDatalogRelationApp(DatalogOpUnion, []ASTHandle{left, right})
}

func (m *ASTManager) MkDatalogWiden(left, right ASTHandle) (ASTHandle, error) {
	return m.mkDatalogRelationApp(DatalogOpWiden, []ASTHandle{left, right})
}

func (m *ASTManager) MkDatalogComplement(relation ASTHandle) (ASTHandle, error) {
	return m.mkDatalogRelationApp(DatalogOpComplement, []ASTHandle{relation})
}

func (m *ASTManager) MkDatalogClone(relation ASTHandle) (ASTHandle, error) {
	return m.mkDatalogRelationApp(DatalogOpClone, []ASTHandle{relation})
}
