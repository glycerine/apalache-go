package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// ElimIte expands equalities with conditional right operands into guarded
// equalities. Other conditionals become fresh constants with defining
// constraints prepended to the result. Shared conditionals share one constant.
// Quantifier annotations are unchanged; source body traversal is preserved.
// Source: duality_rpfp.cpp, ElimIteRec/ElimIte, live UnhoistLoop consumer.
func ElimIte(term *Expr) (out *Expr, result error) {
	if err := term.open(); err != nil {
		return nil, err
	}
	c := term.value.object.context
	m := c.manager
	root, err := term.Raw()
	if err != nil {
		return nil, err
	}
	var pins []ast.ASTRef
	keep := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pins = append(pins, ref)
		return h, nil
	}
	defer func() {
		for i := range pins {
			result = errors.Join(result, m.DelRef(&pins[i]))
		}
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	isOp := func(h ast.ASTHandle, op int32) (bool, error) {
		kind, err := m.GetKind(h)
		if err != nil || kind != ast.ASTApp {
			return false, err
		}
		family, err := m.GetFamilyID(h)
		if err != nil || family != m.BasicFamilyID() {
			return false, err
		}
		k, err := m.GetDeclKind(h)
		return k == op, err
	}
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	var constraints []ast.ASTHandle
	var visit func(ast.ASTHandle) (ast.ASTHandle, error)
	visit = func(h ast.ASTHandle) (ast.ASTHandle, error) {
		if v, ok := memo[h]; ok {
			return v, nil
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		changed := h
		switch kind {
		case ast.ASTApp:
			equality, err := isOp(h, ast.ASTOpEq)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if equality {
				rhs, err := m.GetArg(h, 1)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				conditional, err := isOp(rhs, ast.ASTOpIte)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if conditional {
					lhs, err := m.GetArg(h, 0)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					lhs, err = visit(lhs)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					var args [3]ast.ASTHandle
					for i := range args {
						child, err := m.GetArg(rhs, uint32(i))
						if err != nil {
							return ast.ASTHandle{}, err
						}
						args[i], err = visit(child)
						if err != nil {
							return ast.ASTHandle{}, err
						}
					}
					yes, err := keep(m.MkEq(lhs, args[1]))
					if err != nil {
						return ast.ASTHandle{}, err
					}
					no, err := keep(m.MkEq(lhs, args[2]))
					if err != nil {
						return ast.ASTHandle{}, err
					}
					neg, err := keep(m.MkNot(args[0]))
					if err != nil {
						return ast.ASTHandle{}, err
					}
					yes, err = keep(m.MkAnd(args[0], yes))
					if err != nil {
						return ast.ASTHandle{}, err
					}
					no, err = keep(m.MkAnd(neg, no))
					if err != nil {
						return ast.ASTHandle{}, err
					}
					changed, err = keep(m.MkOr(yes, no))
					if err != nil {
						return ast.ASTHandle{}, err
					}
					memo[h] = changed
					return changed, nil
				}
			}
			conditional, err := isOp(h, ast.ASTOpIte)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if conditional {
				sort, err := m.GetSort(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				changed, err = keep(m.MkFreshConst("@ite", sort))
				if err != nil {
					return ast.ASTHandle{}, err
				}
				// Publish before visiting the definition, matching the source memo slot.
				memo[h] = changed
				definition, err := keep(m.MkEq(changed, h))
				if err != nil {
					return ast.ASTHandle{}, err
				}
				definition, err = visit(definition)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				constraints = append(constraints, definition)
			} else {
				n, err := m.GetNumArgs(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				args := make([]ast.ASTHandle, n)
				for i := range args {
					child, err := m.GetArg(h, uint32(i))
					if err != nil {
						return ast.ASTHandle{}, err
					}
					args[i], err = visit(child)
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
				decl, err := m.GetDecl(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				changed, err = keep(m.MkApp(decl, args))
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			body, err = visit(body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			changed, err = keep(m.UpdateQuantifierBody(h, body))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		memo[h] = changed
		return changed, nil
	}
	h, err := visit(root)
	if err != nil {
		return nil, err
	}
	if len(constraints) > 0 {
		constraints = append(constraints, h)
		h, err = keep(m.MkAndArgs(constraints))
		if err != nil {
			return nil, err
		}
	}
	return NewExpr(c, h)
}
