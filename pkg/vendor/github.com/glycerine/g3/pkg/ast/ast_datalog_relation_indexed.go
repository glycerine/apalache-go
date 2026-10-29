package ast

// Source: src/ast/dl_decl_plugin.cpp:210-250,322-402,412-445,506-583.

import (
	"errors"
	"math"
)

func (m *ASTManager) datalogIntegerIndices(ps []ASTParameter) ([]uint32, error) {
	indices := make([]uint32, len(ps))
	for i, parameter := range ps {
		value, e := m.parameter(parameter)
		if e != nil {
			return nil, e
		}
		if value.Kind != ASTParameterInt || value.integer < 0 {
			return nil, ErrASTDomain
		}
		indices[i] = uint32(value.integer)
	}
	return indices, nil
}

func (m *ASTManager) datalogRelationSortFromRefs(columns []astNodeRef) (result astNodeRef, err error) {
	p := m.auxiliaryPlugin(astDatalogPlugin)
	if p == nil {
		return result, ErrASTDomain
	}
	parameters := make([]ASTParameter, 0, len(columns))
	defer func() {
		for _, parameter := range parameters {
			err = errors.Join(err, m.dropParameter(parameter))
		}
	}()
	for _, column := range columns {
		parameter, e := m.newParameterAST(m.external(column))
		if e != nil {
			return result, e
		}
		parameters = append(parameters, parameter)
	}
	return m.datalogRelationSort(p.family, parameters)
}

func (m *ASTManager) datalogIndexedRelationDecl(kind int32, ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	p := m.auxiliaryPlugin(astDatalogPlugin)
	if p == nil {
		return astNodeRef{}, ErrASTDomain
	}
	indices, e := m.datalogIntegerIndices(ps)
	if e != nil {
		return astNodeRef{}, e
	}
	switch kind {
	case DatalogOpProject:
		if len(indices) < 1 || len(domain) != 1 {
			return astNodeRef{}, ErrASTDomain
		}
		relation, e := m.requireKind(domain[0], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		columns, e := m.datalogRelationColumnsRef(relation)
		if e != nil || len(indices) > len(columns) {
			return astNodeRef{}, ErrASTDomain
		}
		resultColumns := make([]astNodeRef, 0, len(columns)-len(indices))
		j := uint32(0)
		for _, index := range indices {
			if index >= uint32(len(columns)) || j > index {
				return astNodeRef{}, ErrASTDomain
			}
			for j < index {
				resultColumns = append(resultColumns, columns[j])
				j++
			}
			j++
		}
		resultColumns = append(resultColumns, columns[j:]...)
		result, e := m.datalogRelationSortFromRefs(resultColumns)
		if e != nil {
			return astNodeRef{}, e
		}
		return m.pluginDecl(p.symbols[kind], p.family, kind, ps, domain, result, ASTFuncFlags{}, false)
	case DatalogOpRename:
		if len(indices) < 2 || len(domain) != 1 {
			return astNodeRef{}, ErrASTDomain
		}
		relation, e := m.requireKind(domain[0], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		columns, e := m.datalogRelationColumnsRef(relation)
		if e != nil {
			return astNodeRef{}, e
		}
		for _, index := range indices {
			if index >= uint32(len(columns)) {
				return astNodeRef{}, ErrASTDomain
			}
		}
		first := indices[0]
		last := columns[first]
		for _, index := range indices[1:] {
			last, columns[index] = columns[index], last
		}
		columns[first] = last
		result, e := m.datalogRelationSortFromRefs(columns)
		if e != nil {
			return astNodeRef{}, e
		}
		return m.pluginDecl(p.symbols[kind], p.family, kind, ps, domain, result, ASTFuncFlags{}, false)
	case DatalogOpJoin, DatalogOpNegationFilter:
		minimum := 0
		if kind == DatalogOpNegationFilter {
			minimum = 1
		}
		if len(indices) < minimum || len(indices)%2 != 0 || len(domain) != 2 {
			return astNodeRef{}, ErrASTDomain
		}
		left, e := m.requireKind(domain[0], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		right, e := m.requireKind(domain[1], ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		leftColumns, e := m.datalogRelationColumnsRef(left)
		if e != nil {
			return astNodeRef{}, e
		}
		rightColumns, e := m.datalogRelationColumnsRef(right)
		if e != nil {
			return astNodeRef{}, e
		}
		for i := 0; i < len(indices); i += 2 {
			a, b := indices[i], indices[i+1]
			if a >= uint32(len(leftColumns)) || b >= uint32(len(rightColumns)) || leftColumns[a] != rightColumns[b] {
				return astNodeRef{}, ErrASTDomain
			}
		}
		result := left
		if kind == DatalogOpJoin {
			columns := append(append(make([]astNodeRef, 0, len(leftColumns)+len(rightColumns)), leftColumns...), rightColumns...)
			result, e = m.datalogRelationSortFromRefs(columns)
			if e != nil {
				return astNodeRef{}, e
			}
		}
		return m.pluginDecl(p.symbols[kind], p.family, kind, ps, domain, result, ASTFuncFlags{}, false)
	default:
		return astNodeRef{}, ErrASTDomain
	}
}

func (m *ASTManager) mkDatalogIndexedApp(kind int32, indices []uint32, args []ASTHandle) (result ASTHandle, err error) {
	family, e := m.RegisterDatalogPlugin()
	if e != nil {
		return result, e
	}
	parameters := make([]ASTParameter, 0, len(indices))
	defer func() {
		for i := range parameters {
			err = errors.Join(err, m.DelParameter(&parameters[i]))
		}
	}()
	for _, index := range indices {
		if index > math.MaxInt32 {
			return result, ErrASTDomain
		}
		parameter, e := m.NewParameterInt(int32(index))
		if e != nil {
			return result, e
		}
		parameters = append(parameters, parameter)
	}
	return m.MkFamilyApp(family, kind, parameters, args)
}

func (m *ASTManager) MkDatalogProject(relation ASTHandle, removed []uint32) (ASTHandle, error) {
	return m.mkDatalogIndexedApp(DatalogOpProject, removed, []ASTHandle{relation})
}

func (m *ASTManager) MkDatalogRename(relation ASTHandle, cycle []uint32) (ASTHandle, error) {
	return m.mkDatalogIndexedApp(DatalogOpRename, cycle, []ASTHandle{relation})
}

func (m *ASTManager) MkDatalogJoin(left, right ASTHandle, equalColumns []uint32) (ASTHandle, error) {
	return m.mkDatalogIndexedApp(DatalogOpJoin, equalColumns, []ASTHandle{left, right})
}

func (m *ASTManager) MkDatalogNegationFilter(left, right ASTHandle, equalColumns []uint32) (ASTHandle, error) {
	return m.mkDatalogIndexedApp(DatalogOpNegationFilter, equalColumns, []ASTHandle{left, right})
}
