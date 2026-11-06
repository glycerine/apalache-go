package ast

// Source: src/ast/dl_decl_plugin.cpp:69-164,727-744,756-768;
// src/ast/dl_decl_plugin.h:141-216.

import (
	"errors"
	"math"
)

// MkDatalogRelationSort constructs the source Table sort whose parameters are
// the relation's ordered column sorts.
func (m *ASTManager) MkDatalogRelationSort(columns []ASTHandle) (result ASTHandle, err error) {
	family, err := m.RegisterDatalogPlugin()
	if err != nil {
		return result, err
	}
	parameters := make([]ASTParameter, 0, len(columns))
	defer func() {
		for i := range parameters {
			err = errors.Join(err, m.DelParameter(&parameters[i]))
		}
	}()
	for _, column := range columns {
		parameter, e := m.NewParameterAST(column)
		if e != nil {
			return result, e
		}
		parameters = append(parameters, parameter)
	}
	return m.MkFamilySort(family, DatalogRelationSortKind, parameters)
}

func (m *ASTManager) MkDatalogRuleSort() (ASTHandle, error) {
	family, e := m.RegisterDatalogPlugin()
	if e != nil {
		return ASTHandle{}, e
	}
	return m.MkFamilySort(family, DatalogRuleSortKind, nil)
}

func (m *ASTManager) datalogRelationSort(fid FamilyID, ps []ASTParameter) (astNodeRef, error) {
	size := uint64(1)
	finite := true
	for _, parameter := range ps {
		value, e := m.parameter(parameter)
		if e != nil {
			return astNodeRef{}, e
		}
		if value.Kind != ASTParameterAST || m.validNode(value.ast) != nil || m.nodeAt(value.ast.slot).kind != ASTSort {
			return astNodeRef{}, ErrASTDomain
		}
		info := m.sortAt(value.ast.slot).info
		columnSize := InfiniteSortSize()
		if info != (ASTSortInfo{}) {
			record, e := m.sortInfo(info)
			if e != nil {
				return astNodeRef{}, e
			}
			columnSize = record.size
		}
		if !columnSize.IsFinite() {
			finite = false
			break
		}
		n, e := columnSize.Size()
		if e != nil {
			return astNodeRef{}, e
		}
		if n != 0 && size > math.MaxUint64/n {
			finite = false
			break
		}
		size *= n
	}
	cardinality := VeryBigSortSize()
	if finite {
		cardinality = FiniteSortSize(size)
	}
	name, e := m.Symbols.Intern("Table")
	if e != nil {
		return astNodeRef{}, e
	}
	slot, generation, e := m.newInfo(true, fid, DatalogRelationSortKind, ps, cardinality, ASTFuncFlags{}, false)
	if e != nil {
		return astNodeRef{}, e
	}
	defer m.dropInfoStorage(slot, generation)
	return m.mkSort(name, ASTSortInfo{m.origin, slot, generation})
}

func (m *ASTManager) datalogSortKind(h ASTHandle) (astNodeRef, int32, bool, error) {
	r, e := m.requireKind(h, ASTSort)
	if e != nil {
		return astNodeRef{}, 0, false, e
	}
	if m.arithmeticPlugin != nil && (r == m.arithmeticPlugin.fields[0] || r == m.arithmeticPlugin.fields[1]) {
		return r, 0, false, nil
	}
	info := m.sortAt(r.slot).info
	if info == (ASTSortInfo{}) {
		return r, 0, false, nil
	}
	record, e := m.sortInfo(info)
	if e != nil {
		return astNodeRef{}, 0, false, e
	}
	p := m.auxiliaryPlugin(astDatalogPlugin)
	return r, record.kind, p != nil && record.family == p.family, nil
}

func (m *ASTManager) IsDatalogRelationSort(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	_, kind, family, e := m.datalogSortKind(h)
	return e == nil && family && kind == DatalogRelationSortKind, e
}

func (m *ASTManager) IsDatalogFiniteSort(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	_, kind, family, e := m.datalogSortKind(h)
	return e == nil && family && kind == astDatalogFiniteSortKind, e
}

func (m *ASTManager) IsDatalogRuleSort(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	_, kind, family, e := m.datalogSortKind(h)
	return e == nil && family && kind == DatalogRuleSortKind, e
}

// DatalogRelationColumns returns borrowed column-sort handles in source order.
func (m *ASTManager) DatalogRelationColumns(h ASTHandle) ([]ASTHandle, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	r, kind, family, e := m.datalogSortKind(h)
	if e != nil || !family || kind != DatalogRelationSortKind {
		return nil, ErrASTDomain
	}
	info, e := m.sortInfo(m.sortAt(r.slot).info)
	if e != nil {
		return nil, e
	}
	result := make([]ASTHandle, info.count)
	for i := range result {
		parameter, e := m.infoParameter(info, uint32(i))
		if e != nil {
			return nil, e
		}
		value, e := m.parameter(parameter)
		if e != nil || value.Kind != ASTParameterAST || m.validNode(value.ast) != nil || m.nodeAt(value.ast.slot).kind != ASTSort {
			return nil, ErrASTDomain
		}
		result[i] = m.external(value.ast)
	}
	return result, nil
}

// DatalogFiniteSortSize implements dl_decl_util::try_get_size.
func (m *ASTManager) DatalogFiniteSortSize(h ASTHandle) (uint64, bool, error) {
	if e := m.lock(); e != nil {
		return 0, false, e
	}
	defer m.mu.Unlock()
	r, kind, family, e := m.datalogSortKind(h)
	if e != nil || !family || kind != astDatalogFiniteSortKind {
		return 0, false, e
	}
	info, e := m.sortInfo(m.sortAt(r.slot).info)
	if e != nil {
		return 0, false, e
	}
	parameter, e := m.infoParameter(info, 1)
	if e != nil {
		return 0, false, e
	}
	value, e := m.parameter(parameter)
	if e != nil || value.Kind != ASTParameterRational {
		return 0, false, ErrASTDomain
	}
	valid, e := m.numerals.IsUint64(&value.Rational)
	if e != nil || !valid {
		return 0, false, e
	}
	size, e := m.numerals.Uint64(&value.Rational)
	return size, e == nil, e
}
