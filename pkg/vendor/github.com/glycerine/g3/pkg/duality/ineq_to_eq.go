package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// IneqToEq recovers equalities from adjacent opposite non-strict arithmetic
// comparisons with identical ordered operands. It preserves conjunction shape
// by replacing the second comparison with true, without further simplification.
// Source: duality_rpfp.cpp, IneqToEqRec/IneqToEq; live UnhoistLoop consumer.
func IneqToEq(term *Expr) (out *Expr, result error) {
	if err := term.open(); err != nil {
		return nil, err
	}
	c := term.value.object.context
	m := c.manager
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	root, err := term.Raw()
	if err != nil {
		return nil, err
	}
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	var pins []ast.ASTRef
	pin := func(h ast.ASTHandle) error {
		r, err := m.NewRef(h)
		if err == nil {
			pins = append(pins, r)
		}
		return err
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
	comparison := func(h ast.ASTHandle) (int32, error) {
		kind, err := m.GetKind(h)
		if err != nil || kind != ast.ASTApp {
			return -1, err
		}
		family, err := m.GetFamilyID(h)
		if err != nil || family != arithmetic {
			return -1, err
		}
		return m.GetDeclKind(h)
	}
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
			decl, err := m.GetDecl(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
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
			family, err := m.GetFamilyID(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			op, err := m.GetDeclKind(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if family == m.BasicFamilyID() && op == ast.ASTOpAnd {
				for i := 0; i+1 < len(args); i++ {
					a, err := comparison(args[i])
					if err != nil {
						return ast.ASTHandle{}, err
					}
					b, err := comparison(args[i+1])
					if err != nil {
						return ast.ASTHandle{}, err
					}
					// Arithmetic declaration kinds: OP_LE=2, OP_GE=3.
					if !((a == 2 && b == 3) || (a == 3 && b == 2)) {
						continue
					}
					x, err := m.GetArg(args[i], 0)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					y, err := m.GetArg(args[i], 1)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					u, err := m.GetArg(args[i+1], 0)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					v, err := m.GetArg(args[i+1], 1)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					if x == u && y == v {
						args[i], err = m.MkEq(x, y)
						if err != nil {
							return ast.ASTHandle{}, err
						}
						if err := pin(args[i]); err != nil {
							return ast.ASTHandle{}, err
						}
						args[i+1], err = m.MkTrue()
						if err != nil {
							return ast.ASTHandle{}, err
						}
						if err := pin(args[i+1]); err != nil {
							return ast.ASTHandle{}, err
						}
					}
				}
			}
			changed, err = m.MkApp(decl, args)
			if err != nil {
				return ast.ASTHandle{}, err
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
			changed, err = m.UpdateQuantifierBody(h, body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		if err := pin(changed); err != nil {
			return ast.ASTHandle{}, err
		}
		memo[h] = changed
		return changed, nil
	}
	h, err := visit(root)
	if err != nil {
		return nil, err
	}
	return NewExpr(c, h)
}
