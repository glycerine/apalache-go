package ast

// Source: src/ast/ast_smt2_pp.cpp:101-160.

import "math"

type smt2DeclParameter struct {
	kind ASTParameterKind
	ival int32
	text string
	decl ASTHandle
}

func (m *ASTManager) smt2DeclParameters(decl ASTHandle) ([]smt2DeclParameter, error) {
	if err := m.lock(); err != nil {
		return nil, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return nil, err
	}
	infoToken := m.declAt(r.slot).info
	if infoToken == (ASTFuncDeclInfo{}) {
		return nil, nil
	}
	info, err := m.funcInfo(infoToken)
	if err != nil {
		return nil, err
	}
	params := make([]smt2DeclParameter, 0, info.count)
	for i := uint32(0); i < info.count; i++ {
		p, e := m.infoParameter(info, i)
		if e != nil {
			return nil, e
		}
		v, e := m.parameter(p)
		if e != nil {
			return nil, e
		}
		q := smt2DeclParameter{kind: v.Kind, ival: v.integer}
		switch v.Kind {
		case ASTParameterInt:
		case ASTParameterRational:
			q.text, e = m.numerals.String(&v.Rational)
			if e != nil {
				return nil, e
			}
		case ASTParameterAST:
			if e := m.validNode(v.ast); e != nil {
				return nil, e
			}
			if m.nodeAt(v.ast.slot).kind != ASTFuncDecl {
				return nil, ErrASTDomain
			}
			q.decl = m.external(v.ast)
		default:
			return nil, ErrASTDomain
		}
		params = append(params, q)
	}
	return params, nil
}

// SMT2PPAs wraps a declaration name with the source sort-qualified "as" form.
func (m *ASTManager) SMT2PPAs(fname, sort ASTHandle) (ASTHandle, error) {
	printedSort, err := m.SMT2PPSort(sort)
	if err != nil {
		return ASTHandle{}, err
	}
	return m.FormatSeq1("as", []ASTHandle{fname, printedSort})
}

// SMT2PPFdeclParams prints the source indexed declaration prefix. The
// declaration parameter walk was classified before source entry.
func (m *ASTManager) SMT2PPFdeclParams(fname, decl ASTHandle) (ASTHandle, error) {
	indexed, err := m.SMT2IsIndexedFdecl(decl)
	if err != nil {
		return ASTHandle{}, err
	}
	if !indexed {
		return ASTHandle{}, ErrASTDomain
	}
	params, err := m.smt2DeclParameters(decl)
	if err != nil {
		return ASTHandle{}, err
	}
	parts := make([]ASTHandle, 0, 1+len(params))
	parts = append(parts, fname)
	for _, p := range params {
		var part ASTHandle
		switch p.kind {
		case ASTParameterInt:
			part, err = m.FormatInt(p.ival)
		case ASTParameterRational:
			part, err = m.FormatString(p.text)
		case ASTParameterAST:
			part, err = m.SMT2FdeclRef(p.decl)
		default:
			return ASTHandle{}, ErrASTDomain
		}
		if err != nil {
			return ASTHandle{}, err
		}
		parts = append(parts, part)
	}
	return m.FormatSeq1("_", parts)
}

// SMT2Fdecl selects the plain, sort-qualified or indexed source declaration
// spelling. The length sentinel marks a compound format, as in C++.
func (m *ASTManager) SMT2Fdecl(decl ASTHandle) (ASTHandle, uint32, error) {
	fname, length, err := m.SMT2FdeclNameDecl(decl)
	if err != nil {
		return ASTHandle{}, 0, err
	}
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return ASTHandle{}, 0, err
	}
	if family == NullFamilyID {
		return fname, length, nil
	}
	as, err := m.SMT2IsSortParam(decl)
	if err != nil {
		return ASTHandle{}, 0, err
	}
	if as {
		sort, e := m.GetRange(decl)
		if e != nil {
			return ASTHandle{}, 0, e
		}
		format, e := m.SMT2PPAs(fname, sort)
		return format, math.MaxUint32, e
	}
	indexed, err := m.SMT2IsIndexedFdecl(decl)
	if err != nil {
		return ASTHandle{}, 0, err
	}
	if indexed {
		format, e := m.SMT2PPFdeclParams(fname, decl)
		return format, math.MaxUint32, e
	}
	return fname, length, nil
}

// SMT2PPSignature prints a declaration reference's domain and range. Indexed
// parameters decorate the name before the signature, as in the source.
func (m *ASTManager) SMT2PPSignature(fname, decl ASTHandle) (ASTHandle, error) {
	indexed, err := m.SMT2IsIndexedFdecl(decl)
	if err != nil {
		return ASTHandle{}, err
	}
	if indexed {
		fname, err = m.SMT2PPFdeclParams(fname, decl)
		if err != nil {
			return ASTHandle{}, err
		}
	}
	arity, err := m.GetArity(decl)
	if err != nil {
		return ASTHandle{}, err
	}
	domain := make([]ASTHandle, 0, arity)
	for i := uint32(0); i < arity; i++ {
		sort, e := m.GetDomain(decl, i)
		if e != nil {
			return ASTHandle{}, e
		}
		format, e := m.SMT2PPSort(sort)
		if e != nil {
			return ASTHandle{}, e
		}
		domain = append(domain, format)
	}
	printedDomain, err := m.FormatSeq5(domain)
	if err != nil {
		return ASTHandle{}, err
	}
	rangeSort, err := m.GetRange(decl)
	if err != nil {
		return ASTHandle{}, err
	}
	printedRange, err := m.SMT2PPSort(rangeSort)
	if err != nil {
		return ASTHandle{}, err
	}
	return m.FormatSeq5([]ASTHandle{fname, printedDomain, printedRange})
}

// SMT2FdeclRef prints a null-family name or a built-in declaration signature.
func (m *ASTManager) SMT2FdeclRef(decl ASTHandle) (ASTHandle, error) {
	fname, _, err := m.SMT2FdeclNameDecl(decl)
	if err != nil {
		return ASTHandle{}, err
	}
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return ASTHandle{}, err
	}
	if family == NullFamilyID {
		return fname, nil
	}
	return m.SMT2PPSignature(fname, decl)
}
