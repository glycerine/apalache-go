package ast

// Source: src/ast/dl_decl_plugin.cpp:98-114,453-469,655-685.

import (
	"errors"
	"strconv"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

const (
	DatalogRelationSortKind int32 = iota
	astDatalogFiniteSortKind
	DatalogRuleSortKind
)

const (
	DatalogOpStore int32 = iota
	DatalogOpEmpty
	DatalogOpIsEmpty
	DatalogOpJoin
	DatalogOpUnion
	DatalogOpWiden
	DatalogOpProject
	DatalogOpFilter
	DatalogOpNegationFilter
	DatalogOpRename
	DatalogOpComplement
	DatalogOpSelect
	DatalogOpClone
	astDatalogConstantKind
	DatalogOpLT
	DatalogOpRep
	DatalogOpAbs
	DatalogLastOp
)

// MkDatalogFiniteSort is dl_decl_util::mk_sort's retained finite-domain path.
func (m *ASTManager) MkDatalogFiniteSort(name usymbol.Symbol, size uint64) (result ASTHandle, err error) {
	if size == 0 {
		return ASTHandle{}, ErrASTDomain
	}
	family, err := m.RegisterDatalogPlugin()
	if err != nil {
		return ASTHandle{}, err
	}
	value, err := m.numerals.NewValue()
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, m.numerals.Del(&value)) }()
	if err = m.numerals.SetUint64(&value, size); err != nil {
		return ASTHandle{}, err
	}
	nameParameter, err := m.NewParameterSymbol(name)
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, m.DelParameter(&nameParameter)) }()
	sizeParameter, err := m.NewParameterRational(&value)
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, m.DelParameter(&sizeParameter)) }()
	return m.MkFamilySort(family, astDatalogFiniteSortKind, []ASTParameter{nameParameter, sizeParameter})
}

// MkDatalogNumeral implements dl_decl_util::mk_numeral for finite-domain,
// arithmetic, bit-vector and Boolean sorts.
func (m *ASTManager) MkDatalogNumeral(value uint64, sort ASTHandle) (result ASTHandle, err error) {
	family, err := m.datalogUtilityFamilyID()
	if err != nil {
		return ASTHandle{}, err
	}
	finite, err := m.IsDatalogFiniteSort(sort)
	if err != nil {
		return result, err
	}
	if !finite {
		boolSort, e := m.MkBoolSort()
		if e != nil {
			return result, e
		}
		if sort == boolSort {
			if value == 0 {
				return m.MkFalse()
			}
			if value == 1 {
				return m.MkTrue()
			}
			return result, ErrASTDomain
		}
		integer, arithmetic, e := m.datalogArithmeticSort(sort)
		if e != nil {
			return result, e
		}
		if arithmetic {
			numeral, e := m.numerals.NewValue()
			if e != nil {
				return result, e
			}
			defer func() { err = errors.Join(err, m.numerals.Del(&numeral)) }()
			if e = m.numerals.SetUint64(&numeral, value); e != nil {
				return result, e
			}
			return m.ArithmeticNumeral(&numeral, integer)
		}
		bitVector, e := m.IsBitVectorSort(sort)
		if e != nil {
			return result, e
		}
		if bitVector {
			width, e := m.BitVectorSize(sort)
			if e != nil {
				return result, e
			}
			return m.BitVectorNumeralUint64(value, width)
		}
		return result, ErrASTDomain
	}
	return m.mkDatalogFiniteNumeral(family, value, sort)
}

func (m *ASTManager) datalogUtilityFamilyID() (FamilyID, error) {
	// dl_decl_util::ast_plugin_registrator installs these dependencies before
	// constructing its arithmetic and bit-vector utility members.
	if _, e := m.RegisterArithmeticPlugin(); e != nil {
		return NullFamilyID, e
	}
	if _, e := m.RegisterBitVectorPlugin(); e != nil {
		return NullFamilyID, e
	}
	return m.RegisterDatalogPlugin()
}

func (m *ASTManager) mkDatalogFiniteNumeral(family FamilyID, value uint64, sort ASTHandle) (result ASTHandle, err error) {
	size, err := m.GetSortSize(sort)
	if err != nil || !size.IsFinite() {
		if err == nil {
			err = ErrASTDomain
		}
		return ASTHandle{}, err
	}
	bound, err := size.Size()
	if err != nil {
		return ASTHandle{}, err
	}
	if value >= bound {
		return ASTHandle{}, ErrASTDomain
	}
	numeral, err := m.numerals.NewValue()
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, m.numerals.Del(&numeral)) }()
	if err = m.numerals.SetUint64(&numeral, value); err != nil {
		return ASTHandle{}, err
	}
	valueParameter, err := m.NewParameterRational(&numeral)
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, m.DelParameter(&valueParameter)) }()
	sortParameter, err := m.NewParameterAST(sort)
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, m.DelParameter(&sortParameter)) }()
	return m.MkFamilyApp(family, astDatalogConstantKind, []ASTParameter{valueParameter, sortParameter}, nil)
}

func (m *ASTManager) MkDatalogLT(left, right ASTHandle) (ASTHandle, error) {
	family, err := m.RegisterDatalogPlugin()
	if err != nil {
		return ASTHandle{}, err
	}
	return m.MkFamilyApp(family, DatalogOpLT, nil, []ASTHandle{left, right})
}

// MkDatalogLE preserves dl_decl_util::mk_le: !(right < left).
func (m *ASTManager) MkDatalogLE(left, right ASTHandle) (ASTHandle, error) {
	less, e := m.MkDatalogLT(right, left)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.MkNot(less)
}

func (m *ASTManager) datalogSort(fid FamilyID, kind int32, ps []ASTParameter) (astNodeRef, error) {
	if kind == DatalogRelationSortKind {
		return m.datalogRelationSort(fid, ps)
	}
	if kind == DatalogRuleSortKind {
		if len(ps) != 0 {
			return astNodeRef{}, ErrASTDomain
		}
		p := m.auxiliaryPlugin(astDatalogPlugin)
		if p == nil || p.family != fid {
			return astNodeRef{}, ErrASTDomain
		}
		slot, generation, e := m.newInfo(true, fid, kind, nil, InfiniteSortSize(), ASTFuncFlags{}, false)
		if e != nil {
			return astNodeRef{}, e
		}
		defer m.dropInfoStorage(slot, generation)
		return m.mkSort(p.symbols[16], ASTSortInfo{m.origin, slot, generation})
	}
	if kind != astDatalogFiniteSortKind || len(ps) != 2 {
		return astNodeRef{}, ErrASTDomain
	}
	name, err := m.parameter(ps[0])
	if err != nil {
		return astNodeRef{}, err
	}
	if name.Kind != ASTParameterSymbol {
		return astNodeRef{}, ErrASTDomain
	}
	sym := name.symbol
	value, err := m.parameter(ps[1])
	if err != nil {
		return astNodeRef{}, err
	}
	if value.Kind != ASTParameterRational {
		return astNodeRef{}, ErrASTDomain
	}
	valid, err := m.numerals.IsUint64(&value.Rational)
	if err != nil {
		return astNodeRef{}, err
	}
	if !valid {
		return astNodeRef{}, ErrASTDomain
	}
	size, err := m.numerals.Uint64(&value.Rational)
	if err != nil {
		return astNodeRef{}, err
	}
	slot, generation, err := m.newInfo(true, fid, kind, ps, FiniteSortSize(size), ASTFuncFlags{}, false)
	if err != nil {
		return astNodeRef{}, err
	}
	defer m.dropInfoStorage(slot, generation)
	return m.mkSort(sym, ASTSortInfo{m.origin, slot, generation})
}

func (m *ASTManager) datalogDecl(kind int32, ps []ASTParameter, domain []ASTHandle, rangeSort ASTHandle) (astNodeRef, error) {
	switch kind {
	case DatalogOpStore, DatalogOpSelect:
		return m.datalogStoreSelectDecl(kind, ps, domain)
	case DatalogOpEmpty:
		return m.datalogEmptyDecl(ps, domain)
	case DatalogOpIsEmpty, DatalogOpUnion, DatalogOpWiden, DatalogOpComplement, DatalogOpClone:
		return m.datalogSimpleRelationDecl(kind, ps, domain)
	case DatalogOpProject, DatalogOpRename, DatalogOpJoin, DatalogOpNegationFilter:
		return m.datalogIndexedRelationDecl(kind, ps, domain)
	case DatalogOpFilter:
		return m.datalogFilterDecl(ps, domain)
	case DatalogOpRep, DatalogOpAbs:
		return m.datalogConversionDecl(kind, ps, domain, rangeSort)
	case DatalogOpLT:
		if len(ps) != 0 || len(domain) != 2 || domain[0] != domain[1] {
			return astNodeRef{}, ErrASTDomain
		}
		s, err := m.requireKind(domain[0], ASTSort)
		if err != nil || !m.isDatalogFiniteSort(s) {
			return astNodeRef{}, ErrASTDomain
		}
		p := m.auxiliaryPlugin(astDatalogPlugin)
		if p == nil {
			return astNodeRef{}, ErrASTDomain
		}
		return m.pluginDecl(p.symbols[DatalogOpLT], p.family, kind, nil, domain, m.boolSort, ASTFuncFlags{}, false)
	}
	if kind != astDatalogConstantKind || len(ps) != 2 || len(domain) != 0 {
		return astNodeRef{}, ErrASTDomain
	}
	value, err := m.parameter(ps[0])
	if err != nil {
		return astNodeRef{}, err
	}
	if value.Kind != ASTParameterRational {
		return astNodeRef{}, ErrASTDomain
	}
	valid, err := m.numerals.IsUint64(&value.Rational)
	if err != nil {
		return astNodeRef{}, err
	}
	if !valid {
		return astNodeRef{}, ErrASTDomain
	}
	sp, err := m.parameter(ps[1])
	if err != nil {
		return astNodeRef{}, err
	}
	if sp.Kind != ASTParameterAST {
		return astNodeRef{}, ErrASTDomain
	}
	s := sp.ast
	if err = m.validNode(s); err != nil {
		return astNodeRef{}, err
	}
	if m.nodeAt(s.slot).kind != ASTSort || !m.isDatalogFiniteSort(s) {
		return astNodeRef{}, ErrASTDomain
	}
	p := m.auxiliaryPlugin(astDatalogPlugin)
	if p == nil {
		return astNodeRef{}, ErrASTDomain
	}
	return m.pluginDecl(p.symbols[13], p.family, kind, ps, nil, s, ASTFuncFlags{}, false)
}

func (m *ASTManager) isDatalogFiniteSort(s astNodeRef) bool {
	info := m.sortAt(s.slot).info
	if info == (ASTSortInfo{}) {
		return false
	}
	i, err := m.sortInfo(info)
	if err != nil || i.kind != astDatalogFiniteSortKind {
		return false
	}
	p := m.auxiliaryPlugin(astDatalogPlugin)
	return p != nil && i.family == p.family
}

// DatalogNumeralValue is dl_decl_util::is_numeral's concrete uint64 reader.
func (m *ASTManager) DatalogNumeralValue(h ASTHandle) (value uint64, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return
	}
	p := m.auxiliaryPlugin(astDatalogPlugin)
	if p == nil {
		return
	}
	decl := m.appAt(r.slot).decl
	family, kind, err := m.declIdentity(decl)
	if err != nil || family != p.family || kind != astDatalogConstantKind {
		return
	}
	info, err := m.funcInfo(m.declAt(decl.slot).info)
	if err != nil {
		return
	}
	first, err := m.infoParameter(info, 0)
	if err != nil {
		return
	}
	param, err := m.parameter(first)
	if err != nil {
		return
	}
	if param.Kind != ASTParameterRational {
		err = ErrASTDomain
		return
	}
	valid, err := m.numerals.IsUint64(&param.Rational)
	if err != nil {
		return
	}
	if !valid {
		err = ErrASTDomain
		return
	}
	value, err = m.numerals.Uint64(&param.Rational)
	if err != nil {
		return
	}
	return value, true, nil
}

// DatalogNumeralText is the finite-domain probe in Z3_get_numeral_rational.
// The source converts the stored uint64 to a rational before printing it.
func (m *ASTManager) DatalogNumeralText(h ASTHandle) (value string, matched bool, err error) {
	n, matched, err := m.DatalogNumeralValue(h)
	if err != nil || !matched {
		return "", matched, err
	}
	return strconv.FormatUint(n, 10), true, nil
}
