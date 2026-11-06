package tab

// Source: src/muz/tab/tab_context.cpp, tb::matcher.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// SemanticMatcher borrows an AST manager. Match extends the caller's concrete
// substitution, which must be reserved for both variable banks, and appends
// owning equality-condition references.
type SemanticMatcher struct{ manager *ast.ASTManager }

func NewSemanticMatcher(manager *ast.ASTManager) (*SemanticMatcher, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &SemanticMatcher{manager: manager}, nil
}

func (m *SemanticMatcher) equal(left, right ast.ASTHandle) (lbool.LBool, error) {
	if left == right {
		return lbool.True, nil
	}
	leftKind, err := m.manager.GetKind(left)
	if err != nil {
		return lbool.Undef, err
	}
	rightKind, err := m.manager.GetKind(right)
	if err != nil {
		return lbool.Undef, err
	}
	if leftKind != ast.ASTApp || rightKind != ast.ASTApp {
		return lbool.Undef, nil
	}
	leftValue, err := m.manager.IsValue(left)
	if err != nil {
		return lbool.Undef, err
	}
	rightValue, err := m.manager.IsValue(right)
	if err != nil {
		return lbool.Undef, err
	}
	if leftValue && rightValue {
		return lbool.False, nil
	}
	leftDecl, leftConstructor, err := m.manager.DatatypeConstructorApp(left)
	if err != nil {
		return lbool.Undef, err
	}
	rightDecl, rightConstructor, err := m.manager.DatatypeConstructorApp(right)
	if err != nil {
		return lbool.Undef, err
	}
	if !leftConstructor || !rightConstructor {
		return lbool.Undef, nil
	}
	if leftDecl != rightDecl {
		return lbool.False, nil
	}
	count, err := m.manager.GetNumArgs(left)
	if err != nil {
		return lbool.Undef, err
	}
	result := lbool.True
	for i := uint32(0); i < count; i++ {
		a, err := m.manager.GetArg(left, i)
		if err != nil {
			return lbool.Undef, err
		}
		b, err := m.manager.GetArg(right, i)
		if err != nil {
			return lbool.Undef, err
		}
		state, err := m.equal(a, b)
		if err != nil || state == lbool.False {
			return state, err
		}
		if state == lbool.Undef {
			result = lbool.Undef
		}
	}
	return result, nil
}

// Match compares predicate applications with the same declaration. The
// substitution and conditions remain extended on a non-match, as in C++.
func (m *SemanticMatcher) Match(pattern, term ast.ASTHandle, bindings *substitution.Substitution, conditions *[]ast.ASTRef) (bool, error) {
	if m == nil || m.manager == nil || bindings == nil || conditions == nil {
		return false, ast.ErrASTHandle
	}
	pk, err := m.manager.GetKind(pattern)
	if err != nil {
		return false, err
	}
	tk, err := m.manager.GetKind(term)
	if err != nil {
		return false, err
	}
	if pk != ast.ASTApp || tk != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	pdecl, err := m.manager.GetDecl(pattern)
	if err != nil {
		return false, err
	}
	tdecl, err := m.manager.GetDecl(term)
	if err != nil {
		return false, err
	}
	if pdecl != tdecl {
		return false, nil
	}
	count, err := m.manager.GetNumArgs(pattern)
	if err != nil {
		return false, err
	}
	tcount, err := m.manager.GetNumArgs(term)
	if err != nil {
		return false, err
	}
	if count != tcount {
		return false, nil
	}
	type pair struct{ pattern, term ast.ASTHandle }
	todo := make([]pair, 0, count)
	for i := uint32(0); i < count; i++ {
		a, err := m.manager.GetArg(pattern, i)
		if err != nil {
			return false, err
		}
		b, err := m.manager.GetArg(term, i)
		if err != nil {
			return false, err
		}
		todo = append(todo, pair{a, b})
	}
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		kind, err := m.manager.GetKind(current.term)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		kind, err = m.manager.GetKind(current.pattern)
		if err != nil {
			return false, err
		}
		left := current.pattern
		if kind == ast.ASTVar {
			index, err := m.manager.GetVarIndex(left)
			if err != nil {
				return false, err
			}
			binding, found, err := bindings.Find(index, 0)
			if err != nil {
				return false, err
			}
			if !found {
				if err = bindings.Insert(index, 0, substitution.NewExprOffset(current.term, 1)); err != nil {
					return false, err
				}
				continue
			}
			left = binding.Expr()
		} else if kind != ast.ASTApp {
			return false, nil
		}
		state, err := m.equal(left, current.term)
		if err != nil {
			return false, err
		}
		if state == lbool.False {
			return false, nil
		}
		if state == lbool.Undef {
			// The caller owns partial conditions on failure. Reserve that
			// ownership before constructing the source side-condition equality.
			ref, err := m.manager.NewRef(ast.ASTHandle{})
			if err != nil {
				return false, err
			}
			*conditions = append(*conditions, ref)
			equality, err := m.manager.MkEq(left, current.term)
			if !equality.IsNull() {
				err = errors.Join(err, m.manager.SetRef(ref, equality))
			}
			if err != nil {
				return false, err
			}
		}
	}
	return true, nil
}
