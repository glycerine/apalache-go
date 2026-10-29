package ast

// Source: src/ast/bv_decl_plugin.h:302,390-394.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (m *ASTManager) IsBitVector(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	s, e := m.sortOf(r)
	if e != nil {
		return false, e
	}
	return m.bitVectorSortRecognized(s)
}

// BitVectorSize accepts the source sort and expression overloads. The result
// borrows metadata; it acquires no reference to the sort or its parameter.
func (m *ASTManager) BitVectorSize(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return 0, e
	}
	if m.nodeAt(r.slot).kind != ASTSort {
		r, e = m.requireExpr(h)
		if e != nil {
			return 0, e
		}
		r, e = m.sortOf(r)
		if e != nil {
			return 0, e
		}
	}
	return m.bitVectorSortSize(r, nil)
}

func (m *ASTManager) bitVectorSortSize(s astNodeRef, g *uparams.GlobalParamsService) (uint32, error) {
	enabled, e := astBVAssertionEnabled(g)
	if e != nil {
		return 0, e
	}
	if enabled {
		isBV, e := m.bitVectorSortRecognized(s)
		if e != nil {
			return 0, e
		}
		if !isBV {
			if e = uparams.NamespaceAssertionFailureOptional(g, "src/ast/bv_decl_plugin.h", 391, "is_bv_sort(s)"); e != nil {
				return 0, e
			}
		}
	}
	info, e := m.sortInfo(m.sortAt(s.slot).info)
	if e != nil {
		return 0, e
	}
	p, e := m.infoParameter(info, 0)
	if e != nil {
		return 0, e
	}
	v, e := m.parameter(p)
	if e != nil {
		return 0, e
	}
	if v.Kind != ASTParameterInt {
		return 0, ErrASTDomain
	}
	return uint32(v.integer), nil
}
