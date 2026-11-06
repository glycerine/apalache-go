package duality

// Source: src/muz/duality/duality_dl_interface.cpp, adding clauses for pinned
// predicates that do not occur as a rule head.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// MissingHeadClauses returns owned clauses for declarations absent from the
// supplied clauses' heads. Predicates must already be filtered by the caller
// to the pinned declarations registered in the fixedpoint context.
func MissingHeadClauses(m *ast.ASTManager, clauses, predicates []ast.ASTHandle) (result []ast.ASTRef, resultErr error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	defer func() {
		if resultErr != nil {
			for i := range result {
				resultErr = errors.Join(resultErr, m.DelRef(&result[i]))
			}
			result = nil
		}
	}()
	heads := make(map[ast.ASTHandle]bool)
	for _, clause := range clauses {
		h := clause
		for {
			kind, err := m.GetKind(h)
			if err != nil {
				return nil, err
			}
			if kind == ast.ASTQuantifier {
				h, err = m.GetQuantifierBody(h)
				if err != nil {
					return nil, err
				}
				continue
			}
			if kind != ast.ASTApp {
				break
			}
			family, err := m.GetFamilyID(h)
			if err != nil {
				return nil, err
			}
			if family == m.BasicFamilyID() {
				op, e := m.GetDeclKind(h)
				if e != nil {
					return nil, e
				}
				if op == ast.ASTOpImplies {
					h, e = m.GetArg(h, 1)
					if e != nil {
						return nil, e
					}
					continue
				}
			}
			decl, err := m.GetDecl(h)
			if err != nil {
				return nil, err
			}
			heads[decl] = true
			break
		}
	}
	for _, predicate := range predicates {
		if heads[predicate] {
			continue
		}
		ref, err := missingHeadClause(m, predicate)
		if err != nil {
			return result, err
		}
		result = append(result, ref)
	}
	return result, nil
}

func missingHeadClause(m *ast.ASTManager, predicate ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	arity, err := m.GetArity(predicate)
	if err != nil {
		return result, err
	}
	refs := make([]ast.ASTRef, 0, arity+1)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
		}
	}()
	args := make([]ast.ASTHandle, 0, arity)
	for i := uint32(0); i < arity; i++ {
		sort, e := m.GetDomain(predicate, i)
		if e != nil {
			return result, e
		}
		arg, e := m.MkFreshConst("X", sort)
		if e != nil {
			return result, e
		}
		ref, e := m.NewRef(arg)
		if e != nil {
			return result, e
		}
		refs = append(refs, ref)
		args = append(args, arg)
	}
	head, err := m.MkApp(predicate, args)
	if err != nil {
		return result, err
	}
	falsity, err := m.MkFalse()
	if err != nil {
		return result, err
	}
	body, err := m.MkImplies(falsity, head)
	if err != nil {
		return result, err
	}
	bodyRef, err := m.NewRef(body)
	if err != nil {
		return result, err
	}
	refs = append(refs, bodyRef)
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	result = out
	abstract, err := ast.NewExprAbstractor(m)
	if err != nil {
		return result, err
	}
	err = abstract.MkForall(args, body, out)
	return result, errors.Join(err, abstract.Close())
}
