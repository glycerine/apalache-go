package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// RemoveRedundancy follows the source ordered sibling-literal substitutions,
// arithmetic numeral equalities, common factoring and quantified simplification.
// Every intermediate and memoized result owns its reference for the full pass.
// Source: duality_rpfp.cpp:624-693.
func (c *Context) RemoveRedundancy(formula *Expr) (out *Expr, result error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if err := formula.open(); err != nil {
		return nil, err
	}
	if formula.value.object.context != c {
		return nil, ErrContextMismatch
	}
	m := c.manager
	root, err := formula.Raw()
	if err != nil {
		return nil, err
	}
	memo := make(map[ast.ASTHandle]*Expr)
	var pins []*Expr
	defer func() {
		for _, pin := range pins {
			result = errors.Join(result, pin.Close())
		}
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	own := func(term *Expr, err error) (*Expr, error) {
		if term != nil {
			pins = append(pins, term)
		}
		return term, err
	}
	wrap := func(h ast.ASTHandle, err error) (*Expr, error) {
		if err != nil {
			return nil, err
		}
		return own(NewExpr(c, h))
	}
	raw := func(term *Expr) (ast.ASTHandle, error) { return term.Raw() }
	substitute := func(input *Expr, from, to ast.ASTHandle) (*Expr, error) {
		h, err := input.Raw()
		if err != nil {
			return nil, err
		}
		ref, err := substituteExpression(c, h, map[ast.ASTHandle]ast.ASTHandle{from: to})
		if err != nil {
			return nil, err
		}
		h, err = m.RefGet(ref)
		if err != nil {
			return nil, errors.Join(err, m.DelRef(&ref))
		}
		next, createErr := wrap(h, nil)
		return next, errors.Join(createErr, m.DelRef(&ref))
	}
	reduce := func(args []*Expr, and bool) error {
		for i := range args {
			atom, positive, err := IsLiteral(args[i])
			if err != nil {
				return err
			}
			if atom == nil {
				continue
			}
			pins = append(pins, atom)
			h, err := atom.Raw()
			if err != nil {
				return err
			}
			kind, err := m.GetKind(h)
			if err != nil {
				return err
			}
			if kind == ast.ASTApp && positive == and {
				family, err := m.GetFamilyID(h)
				if err != nil {
					return err
				}
				op, err := m.GetDeclKind(h)
				if err != nil {
					return err
				}
				if family == m.BasicFamilyID() && op == ast.ASTOpEq {
					lhs, err := m.GetArg(h, 0)
					if err != nil {
						return err
					}
					rhs, err := m.GetArg(h, 1)
					if err != nil {
						return err
					}
					numeral, err := sourceArithmeticNumeral(m, lhs)
					if err != nil {
						return err
					}
					if numeral {
						lhs, rhs = rhs, lhs
					}
					numeral, err = sourceArithmeticNumeral(m, rhs)
					if err != nil {
						return err
					}
					lhsKind, err := m.GetKind(lhs)
					if err != nil {
						return err
					}
					if numeral && lhsKind == ast.ASTApp {
						for j := range args {
							if j != i {
								args[j], err = substitute(args[j], lhs, rhs)
								if err != nil {
									return err
								}
							}
						}
					}
				}
			}
			value, err := own(c.BoolVal(positive))
			if err != nil {
				return err
			}
			if !and {
				rawValue, err := value.Raw()
				if err != nil {
					return err
				}
				value, err = wrap(m.MkNot(rawValue))
				if err != nil {
					return err
				}
			}

			for j := range args {
				if j != i {
					args[j], err = own(c.SubstAtom(args[j], atom, value))
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	var visit func(ast.ASTHandle) (*Expr, error)
	visit = func(h ast.ASTHandle) (*Expr, error) {
		if found, ok := memo[h]; ok {
			return found, nil
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return nil, err
		}
		var next *Expr
		switch kind {
		case ast.ASTApp:
			count, err := m.GetNumArgs(h)
			if err != nil {
				return nil, err
			}
			args := make([]*Expr, count)
			for i := uint32(0); i < count; i++ {
				child, err := m.GetArg(h, i)
				if err != nil {
					return nil, err
				}
				args[i], err = visit(child)
				if err != nil {
					return nil, err
				}
			}
			family, err := m.GetFamilyID(h)
			if err != nil {
				return nil, err
			}
			op, err := m.GetDeclKind(h)
			if err != nil {
				return nil, err
			}
			if family == m.BasicFamilyID() && (op == ast.ASTOpAnd || op == ast.ASTOpOr) {
				if err = reduce(args, op == ast.ASTOpAnd); err != nil {
					return nil, err
				}
				next, err = own(c.ReallySimplifyAndOr(args, op == ast.ASTOpAnd))
				if err != nil {
					return nil, err
				}
			} else {
				handles := make([]ast.ASTHandle, count)
				for i, arg := range args {
					handles[i], err = raw(arg)
					if err != nil {
						return nil, err
					}
				}
				if family == m.BasicFamilyID() && op == ast.ASTOpEq {
					left, err := m.GetASTID(handles[0])
					if err != nil {
						return nil, err
					}
					right, err := m.GetASTID(handles[1])
					if err != nil {
						return nil, err
					}
					if left > right {
						handles[0], handles[1] = handles[1], handles[0]
					}
				}
				decl, err := m.GetDecl(h)
				if err != nil {
					return nil, err
				}
				next, err = wrap(m.MkApp(decl, handles))
				if err != nil {
					return nil, err
				}
			}
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return nil, err
			}
			changed, err := visit(body)
			if err != nil {
				return nil, err
			}
			q, err := wrap(h, nil)
			if err != nil {
				return nil, err
			}
			next, err = own(c.CloneQuantAndSimp(q, changed))
			if err != nil {
				return nil, err
			}
		default:
			next, err = wrap(h, nil)
			if err != nil {
				return nil, err
			}
		}
		memo[h] = next
		return next, nil
	}
	simplified, err := visit(root)
	if err != nil {
		return nil, err
	}
	return simplified.Clone()
}
