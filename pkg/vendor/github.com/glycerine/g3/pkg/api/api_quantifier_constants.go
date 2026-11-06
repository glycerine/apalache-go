package api

// Source: src/api/api_quant.cpp:143-283.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ConstantQuantifierSpec describes the source convenience constructor that
// abstracts nullary uninterpreted constants into de Bruijn variables before
// creating a quantifier.
type ConstantQuantifierSpec struct {
	Forall               bool
	Bound                []ast.ASTHandle
	Body                 ast.ASTHandle
	Weight               int32
	QID, SkolemID        string
	Patterns, NoPatterns []ast.ASTHandle
}

func (c *SMTContext) ConstantQuantifier(spec ConstantQuantifierSpec) (ast.ASTHandle, error) {
	if len(spec.Bound) == 0 || (len(spec.Patterns) != 0 && len(spec.NoPatterns) != 0) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	m, err := c.quantifierManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	qid, err := internQuantifierSymbol(m, spec.QID)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	skid, err := internQuantifierSymbol(m, spec.SkolemID)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.constantQuantifierSymbols(spec, qid, skid)
}

func (c *SMTContext) constantQuantifierSymbols(spec ConstantQuantifierSpec, qid, skid usymbol.Symbol) (result ast.ASTHandle, err error) {
	if len(spec.Bound) == 0 || (len(spec.Patterns) != 0 && len(spec.NoPatterns) != 0) {
		return result, ast.ErrASTDomain
	}
	m, err := c.quantifierManager()
	if err != nil {
		return result, err
	}
	sorts := make([]ast.ASTHandle, len(spec.Bound))
	names := make([]usymbol.Symbol, len(spec.Bound))
	for i, bound := range spec.Bound {
		kind, e := m.GetKind(bound)
		if e != nil || kind != ast.ASTApp {
			if e == nil {
				e = ast.ErrASTDomain
			}
			return result, e
		}
		if count, e := m.GetNumArgs(bound); e != nil || count != 0 {
			if e == nil {
				e = ast.ErrASTDomain
			}
			return result, e
		}
		decl, e := m.GetDecl(bound)
		if e != nil {
			return result, e
		}
		family, e := m.GetFamilyID(decl)
		if e != nil || family != ast.NullFamilyID {
			if e == nil {
				e = ast.ErrASTDomain
			}
			return result, e
		}
		sorts[i], e = m.GetSort(bound)
		if e != nil {
			return result, e
		}
		name, e := m.GetASTName(decl)
		if e != nil {
			return result, e
		}
		names[i] = name
	}
	abstractor, err := ast.NewExprAbstractor(m)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, abstractor.Close()) }()
	refs := make([]ast.ASTRef, 0, len(spec.Patterns)+len(spec.NoPatterns)+1)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&refs[i]))
		}
	}()
	abstract := func(root ast.ASTHandle) (ast.ASTHandle, error) {
		ref, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return ast.ASTHandle{}, e
		}
		refs = append(refs, ref)
		if e = abstractor.Abstract(0, spec.Bound, root, refs[len(refs)-1]); e != nil {
			return ast.ASTHandle{}, e
		}
		return m.RefGet(refs[len(refs)-1])
	}
	patterns := make([]ast.ASTHandle, len(spec.Patterns))
	for i, pattern := range spec.Patterns {
		yes, e := m.IsPattern(pattern)
		if e != nil || !yes {
			if e == nil {
				e = ast.ErrASTDomain
			}
			return result, e
		}
		patterns[i], e = abstract(pattern)
		if e != nil {
			return result, e
		}
	}
	noPatterns := make([]ast.ASTHandle, len(spec.NoPatterns))
	for i, pattern := range spec.NoPatterns {
		kind, e := m.GetKind(pattern)
		if e != nil || kind != ast.ASTApp {
			if e == nil {
				e = ast.ErrASTDomain
			}
			return result, e
		}
		noPatterns[i], e = abstract(pattern)
		if e != nil {
			return result, e
		}
	}
	body, err := abstract(spec.Body)
	if err != nil {
		return result, err
	}
	bodySort, err := m.GetSort(body)
	if err != nil {
		return result, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return result, err
	}
	if bodySort != boolean {
		return result, ast.ErrASTDomain
	}
	if err = validateQuantifierPatterns(m, qid, uint32(len(sorts)), patterns); err != nil {
		return result, err
	}
	return m.MkQuantifier(ast.ASTQuantifierSpec{Forall: spec.Forall, Sorts: sorts, Names: names, Body: body, Weight: spec.Weight, QID: qid, SkolemID: skid, Patterns: patterns, NoPatterns: noPatterns})
}

func (c *SMTContext) ForallConstants(bound []ast.ASTHandle, body ast.ASTHandle, patterns ...ast.ASTHandle) (ast.ASTHandle, error) {
	return c.ConstantQuantifier(ConstantQuantifierSpec{Forall: true, Bound: bound, Body: body, Patterns: patterns})
}
func (c *SMTContext) ExistsConstants(bound []ast.ASTHandle, body ast.ASTHandle, patterns ...ast.ASTHandle) (ast.ASTHandle, error) {
	return c.ConstantQuantifier(ConstantQuantifierSpec{Bound: bound, Body: body, Patterns: patterns})
}
