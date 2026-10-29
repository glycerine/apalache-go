package ast

// Source: src/ast/ast_smt2_pp.cpp:397-448.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

type smt2SortParameter struct {
	kind ASTParameterKind
	ival uint32
	ast  ASTHandle
}

type smt2SortView struct {
	family                                                                                       FamilyID
	kind                                                                                         int32
	name                                                                                         usymbol.Symbol
	params                                                                                       []smt2SortParameter
	boolSort, intSort, realSort, bvSort, arraySort, floatSort, seqSort, stringSort, datatypeSort bool
}

func (m *ASTManager) smt2SortView(h ASTHandle) (smt2SortView, error) {
	if err := m.lock(); err != nil {
		return smt2SortView{}, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(h, ASTSort)
	if err != nil {
		return smt2SortView{}, err
	}
	s := m.sortAt(r.slot)
	v := smt2SortView{name: s.name, boolSort: r == m.basic.boolSort}
	v.family, v.kind, err = m.declIdentity(r)
	if err != nil {
		return v, err
	}
	if m.arithmeticPlugin != nil && v.family == m.arithmeticPlugin.family {
		v.realSort = v.kind == 0
		v.intSort = v.kind == 1
	}
	v.bvSort = m.bitVectorPlugin != nil && v.family == m.bitVectorPlugin.family && v.kind == 0
	v.arraySort = m.arrayPlugin != nil && v.family == m.arrayPlugin.family && v.kind == 0
	v.floatSort = m.floatingPointPlugin != nil && v.family == m.floatingPointPlugin.family && v.kind == 0
	v.seqSort = m.sequencePlugin != nil && v.family == m.sequencePlugin.family && (v.kind == 0 || v.kind == 1)
	v.stringSort = m.sequencePlugin != nil && r == m.sequencePlugin.stringSort
	v.datatypeSort = m.datatypePlugin != nil && v.family == m.datatypePlugin.family && v.kind == 0
	if s.info == (ASTSortInfo{}) {
		return v, nil
	}
	info, err := m.sortInfo(s.info)
	if err != nil {
		return v, err
	}
	v.params = make([]smt2SortParameter, 0, info.count)
	for i := uint32(0); i < info.count; i++ {
		p, e := m.infoParameter(info, i)
		if e != nil {
			return v, e
		}
		raw, e := m.parameter(p)
		if e != nil {
			return v, e
		}
		q := smt2SortParameter{kind: raw.Kind, ival: uint32(raw.integer)}
		if raw.Kind == ASTParameterAST {
			if e := m.validNode(raw.ast); e != nil {
				return v, e
			}
			q.ast = m.external(raw.ast)
		}
		v.params = append(v.params, q)
	}
	return v, nil
}

// SMT2PPSort builds the base SMT2 environment's sort document. The command
// context's separate parametric-sort override is a different source owner.
func (m *ASTManager) SMT2PPSort(sort ASTHandle) (ASTHandle, error) {
	v, err := m.smt2SortView(sort)
	if err != nil {
		return ASTHandle{}, err
	}
	switch {
	case v.boolSort:
		return m.FormatString("Bool")
	case v.intSort:
		return m.FormatString("Int")
	case v.realSort:
		return m.FormatString("Real")
	case v.bvSort:
		if len(v.params) != 1 || v.params[0].kind != ASTParameterInt {
			return ASTHandle{}, ErrASTDomain
		}
		name, e := m.FormatString("BitVec")
		if e != nil {
			return ASTHandle{}, e
		}
		width, e := m.FormatUnsigned(v.params[0].ival)
		if e != nil {
			return ASTHandle{}, e
		}
		return m.FormatSeq1("_", []ASTHandle{name, width})
	case v.arraySort:
		if len(v.params) < 2 {
			return ASTHandle{}, ErrASTDomain
		}
		args := make([]ASTHandle, 0, len(v.params))
		for _, p := range v.params {
			if p.kind != ASTParameterAST {
				return ASTHandle{}, ErrASTDomain
			}
			f, e := m.SMT2PPSort(p.ast)
			if e != nil {
				return ASTHandle{}, e
			}
			args = append(args, f)
		}
		return m.FormatSeq1("Array", args)
	case v.floatSort:
		if len(v.params) != 2 || v.params[0].kind != ASTParameterInt || v.params[1].kind != ASTParameterInt {
			return ASTHandle{}, ErrASTDomain
		}
		name, e := m.FormatString("FloatingPoint")
		if e != nil {
			return ASTHandle{}, e
		}
		ebits, e := m.FormatUnsigned(v.params[0].ival)
		if e != nil {
			return ASTHandle{}, e
		}
		sbits, e := m.FormatUnsigned(v.params[1].ival)
		if e != nil {
			return ASTHandle{}, e
		}
		return m.FormatSeq1("_", []ASTHandle{name, ebits, sbits})
	case v.seqSort && !v.stringSort:
		if len(v.params) < 1 || v.params[0].kind != ASTParameterAST {
			return ASTHandle{}, ErrASTDomain
		}
		inner, e := m.SMT2PPSort(v.params[0].ast)
		if e != nil {
			return ASTHandle{}, e
		}
		if v.kind == 0 {
			return m.FormatSeq1("Seq", []ASTHandle{inner})
		}
		return m.FormatSeq1("RegEx", []ASTHandle{inner})
	case (v.datatypeSort && len(v.params) > 1) || ((v.family == NullFamilyID || v.family == 4) && len(v.params) > 0):
		parameters := v.params
		if v.datatypeSort {
			parameters = parameters[1:]
		}
		args := make([]ASTHandle, 0, len(parameters))
		for _, p := range parameters {
			if p.kind != ASTParameterAST {
				return ASTHandle{}, ErrASTDomain
			}
			f, e := m.SMT2PPSort(p.ast)
			if e != nil {
				return ASTHandle{}, e
			}
			args = append(args, f)
		}
		name, e := m.Symbols.String(v.name)
		if e != nil {
			return ASTHandle{}, e
		}
		return m.FormatSeq1(name, args)
	default:
		name, e := m.Symbols.String(v.name)
		if e != nil {
			return ASTHandle{}, e
		}
		return m.FormatString(name)
	}
}
