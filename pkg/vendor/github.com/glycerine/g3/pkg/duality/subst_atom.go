package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// SubstAtom replaces a literal through the source Boolean skeleton. It recurs
// only through conjunctions/disjunctions, and through quantifier bodies only
// when the substituted atom is itself quantified. Other applications are leaves.
// Source: duality_rpfp.cpp:452-461,537-545,593-622.
func (c *Context) SubstAtom(formula, atom, value *Expr) (out *Expr, result error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	for _, term := range []*Expr{formula, atom, value} {
		if err := term.open(); err != nil {
			return nil, err
		}
		if term.value.object.context != c {
			return nil, ErrContextMismatch
		}
	}
	m := c.manager
	root, err := formula.Raw()
	if err != nil {
		return nil, err
	}
	target, err := atom.Raw()
	if err != nil {
		return nil, err
	}
	replacement, err := value.Raw()
	if err != nil {
		return nil, err
	}
	targetKind, err := m.GetKind(target)
	if err != nil {
		return nil, err
	}
	isNot := func(h ast.ASTHandle) (bool, ast.ASTHandle, error) {
		kind, err := m.GetKind(h)
		if err != nil || kind != ast.ASTApp {
			return false, ast.ASTHandle{}, err
		}
		op, err := m.GetDeclKind(h)
		if err != nil || op != ast.ASTOpNot {
			return false, ast.ASTHandle{}, err
		}
		child, err := m.GetArg(h, 0)
		return true, child, err
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
	own := func(h ast.ASTHandle, err error) (*Expr, error) {
		if err != nil {
			return nil, err
		}
		term, err := NewExpr(c, h)
		if err != nil {
			return nil, err
		}
		pins = append(pins, term)
		return term, nil
	}
	var visit func(ast.ASTHandle) (*Expr, error)
	visit = func(h ast.ASTHandle) (*Expr, error) {
		if term, found := memo[h]; found {
			return term, nil
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return nil, err
		}
		var next *Expr
		if kind == ast.ASTApp {
			op, err := m.GetDeclKind(h)
			if err != nil {
				return nil, err
			}
			if op == ast.ASTOpAnd || op == ast.ASTOpOr {
				n, err := m.GetNumArgs(h)
				if err != nil {
					return nil, err
				}
				args := make([]*Expr, n)
				for i := uint32(0); i < n; i++ {
					child, err := m.GetArg(h, i)
					if err != nil {
						return nil, err
					}
					args[i], err = visit(child)
					if err != nil {
						return nil, err
					}
				}
				next, err = c.ReallySimplifyAndOr(args, op == ast.ASTOpAnd)
				if err != nil {
					return nil, err
				}
				pins = append(pins, next)
			}
		} else if kind == ast.ASTQuantifier && targetKind == ast.ASTQuantifier {
			if h == target {
				next, err = own(replacement, nil)
			} else {
				body, err := m.GetQuantifierBody(h)
				if err != nil {
					return nil, err
				}
				changed, err := visit(body)
				if err != nil {
					return nil, err
				}
				raw, err := changed.Raw()
				if err != nil {
					return nil, err
				}
				next, err = own(m.UpdateQuantifierBody(h, raw))
				if err != nil {
					return nil, err
				}
			}
			if err != nil {
				return nil, err
			}
		}
		if next == nil {
			raw := h
			if h == target {
				raw = replacement
			} else {
				negated, child, err := isNot(h)
				if err != nil {
					return nil, err
				}
				if negated && child == target {
					// Source Negate removes one not or flips Boolean constants.
					neg, inner, err := isNot(replacement)
					if err != nil {
						return nil, err
					}
					switch {
					case neg:
						raw = inner
					case m.IsTrue(replacement):
						raw, err = m.MkFalse()
					case m.IsFalse(replacement):
						raw, err = m.MkTrue()
					default:
						raw, err = m.MkNot(replacement)
					}
					if err != nil {
						return nil, err
					}
				}
			}
			next, err = own(raw, nil)
			if err != nil {
				return nil, err
			}
		}
		memo[h] = next
		return next, nil
	}
	reduced, err := visit(root)
	if err != nil {
		return nil, err
	}
	return reduced.Clone()
}
