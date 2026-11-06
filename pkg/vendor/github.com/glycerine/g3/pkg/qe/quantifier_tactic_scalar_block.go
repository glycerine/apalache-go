package qe

// Source: src/qe/qe.cpp, same-polarity variable block elimination.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// eliminateQuantifierBitVectorBlock eliminates each declaration in one
// quantifier block using the existing bit-vector search.
func eliminateQuantifierBitVectorBlock(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (result ast.ASTRef, complete bool, err error) {
	return eliminateQuantifierScalarBlock(m, quantifier, config, qeScalarBitVector)
}

func eliminateQuantifierDatatypeBlock(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (result ast.ASTRef, complete bool, err error) {
	return eliminateQuantifierScalarBlock(m, quantifier, config, qeScalarDatatype)
}

func eliminateQuantifierDatalogBlock(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (result ast.ASTRef, complete bool, err error) {
	return eliminateQuantifierScalarBlock(m, quantifier, config, qeScalarDatalog)
}

func eliminateQuantifierScalarBlock(m *ast.ASTManager, quantifier ast.ASTHandle, config Config, searchKind qeScalarSearchKind) (result ast.ASTRef, complete bool, err error) {
	if m == nil || config.Cooperation == nil {
		return result, false, ast.ErrASTHandle
	}
	if searchKind != qeScalarBitVector && searchKind != qeScalarDatatype && searchKind != qeScalarDatalog {
		return result, false, ast.ErrASTDomain
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	kind, err := m.GetKind(quantifier)
	if err != nil || kind != ast.ASTQuantifier {
		return result, false, err
	}
	free, err := rewriter.NewExprFreeVars(m)
	if err != nil {
		return result, false, err
	}
	if err = free.Collect(quantifier); err != nil || !free.Empty() {
		return result, false, err
	}
	body, err := m.GetQuantifierBody(quantifier)
	if err != nil {
		return result, false, err
	}
	nested, err := m.HasQuantifiers(body)
	if err != nil || nested {
		return result, false, err
	}
	count, err := m.GetNumDeclarations(quantifier)
	if err != nil {
		return result, false, err
	}
	var scalarFamily ast.FamilyID
	if searchKind == qeScalarDatatype {
		scalarFamily, err = m.RegisterDatatypePlugin()
		if err != nil {
			return result, false, err
		}
	} else if searchKind == qeScalarDatalog {
		scalarFamily, err = m.RegisterDatalogPlugin()
		if err != nil {
			return result, false, err
		}
	}
	for i := uint32(0); i < count; i++ {
		sort, cause := m.GetDeclarationSort(quantifier, i)
		if cause != nil {
			return result, false, cause
		}
		if searchKind == qeScalarBitVector {
			isBV, cause := m.IsBitVectorSort(sort)
			if cause != nil || !isBV {
				return result, false, cause
			}
		} else {
			family, cause := m.GetFamilyID(sort)
			if cause != nil || family != scalarFamily {
				return result, false, cause
			}
		}
	}
	extracted, err := extractExprQuantifier(m, quantifier)
	if err != nil {
		return result, false, closeFailedExprQuantifierExtract(extracted, err)
	}
	defer func() { err = errors.Join(err, extracted.Close()) }()
	body, err = m.RefGet(extracted.body)
	if err != nil {
		return result, false, err
	}
	forall, err := m.IsForall(quantifier)
	if err != nil {
		return result, false, err
	}
	if forall {
		body, err = m.MkNot(body)
		if err != nil {
			return result, false, err
		}
	}
	formula, err := m.NewRef(body)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&formula)) }()
	for _, ref := range extracted.variables {
		variable, e := m.RefGet(ref)
		if e != nil {
			return result, false, e
		}
		body, e = m.RefGet(formula)
		if e != nil {
			return result, false, e
		}
		bound, e := bindExprQuantifierVariables(m, []ast.ASTHandle{variable}, body)
		if e != nil {
			return result, false, e
		}
		q, e := m.RefGet(bound)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		kind, e := m.GetKind(q)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		if kind != ast.ASTQuantifier {
			if e = m.DelRef(&bound); e != nil {
				return result, false, e
			}
			continue
		}
		part, done, e := eliminateQuantifierScalarSearch(m, q, config, searchKind)
		e = errors.Join(e, m.DelRef(&bound))
		if e != nil || !done {
			if part != (ast.ASTRef{}) {
				e = errors.Join(e, m.DelRef(&part))
			}
			return result, false, e
		}
		value, e := m.RefGet(part)
		if e == nil {
			e = m.SetRef(formula, value)
		}
		e = errors.Join(e, m.DelRef(&part))
		if e != nil {
			return result, false, e
		}
	}
	body, err = m.RefGet(formula)
	if err != nil {
		return result, false, err
	}
	if forall {
		body, err = m.MkNotSimplified(body)
		if err != nil {
			return result, false, err
		}
	}
	result, err = m.NewRef(body)
	return result, err == nil, err
}
