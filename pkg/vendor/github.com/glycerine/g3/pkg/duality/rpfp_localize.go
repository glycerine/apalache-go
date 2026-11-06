package duality

// Source: duality_rpfp.cpp:222-305 (LocalizeRec, Localize, ReducedDualEdge).
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (g *RPFP) Localize(edge *Edge, term *Expr) (out *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if edge == nil || edge.Owner != g || edge.closed || g.vocabulary.closed {
		return nil, ErrWrapperOwner
	}
	if err := term.open(); err != nil {
		return nil, err
	}
	if term.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	timer, err := startProfile(g.context, "Localize")
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	if len(edge.F.individualParams) > 0 && !edge.mapsReady {
		if err := g.SetEdgeMaps(edge); err != nil {
			return nil, err
		}
	}
	m := g.context.manager
	memo := make(map[ast.ASTHandle]*Expr)
	defer func() {
		for _, v := range memo {
			result = errors.Join(result, v.Close())
		}
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	var visit func(ast.ASTHandle) (*Expr, error)
	visit = func(h ast.ASTHandle) (*Expr, error) {
		if v, ok := memo[h]; ok {
			return v, nil
		}
		var v *Expr
		if ref, ok := edge.varMap[h]; ok {
			to, err := m.RefGet(ref)
			if err != nil {
				return nil, err
			}
			v, err = NewExpr(g.context, to)
			if err != nil {
				return nil, err
			}
		} else {
			kind, err := m.GetKind(h)
			if err != nil {
				return nil, err
			}
			switch kind {
			case ast.ASTApp:
				decl, err := m.GetDecl(h)
				if err != nil {
					return nil, err
				}
				count, err := m.GetNumArgs(h)
				if err != nil {
					return nil, err
				}
				args := make([]*Expr, count)
				raw := make([]ast.ASTHandle, count)
				for n := uint32(0); n < count; n++ {
					a, err := m.GetArg(h, n)
					if err != nil {
						return nil, err
					}
					args[n], err = visit(a)
					if err != nil {
						return nil, err
					}
					raw[n], err = args[n].Raw()
					if err != nil {
						return nil, err
					}
				}
				if idx, ok := edge.relMap[decl]; ok {
					v, err = g.RedDualRela(edge, args, idx)
				} else {
					family, e := m.GetFamilyID(h)
					if e != nil {
						return nil, e
					}
					_, background := g.vocabulary.constants[decl]
					if count == 0 && family == ast.NullFamilyID && !background {
						original, e := NewExpr(g.context, h)
						if e != nil {
							return nil, e
						}
						v, err = HideVariable(original, edge.Number)
						err = errors.Join(err, original.Close())
					} else {
						rebuilt, e := m.MkApp(decl, raw)
						err = e
						if err == nil {
							v, err = NewExpr(g.context, rebuilt)
						}
					}
				}
				if err != nil {
					if v != nil {
						err = errors.Join(err, v.Close())
					}
					return nil, err
				}
			case ast.ASTQuantifier:
				count, err := m.GetNumPatterns(h)
				if err != nil {
					return nil, err
				}
				patterns := make([]ast.ASTHandle, count)
				for n := uint32(0); n < count; n++ {
					p, err := m.GetPattern(h, n)
					if err != nil {
						return nil, err
					}
					localized, err := visit(p)
					if err != nil {
						return nil, err
					}
					patterns[n], err = localized.Raw()
					if err != nil {
						return nil, err
					}
				}
				body, err := m.GetQuantifierBody(h)
				if err != nil {
					return nil, err
				}
				localized, err := visit(body)
				if err != nil {
					return nil, err
				}
				body, err = localized.Raw()
				if err != nil {
					return nil, err
				}
				forall, err := m.IsForall(h)
				if err != nil {
					return nil, err
				}
				rebuilt, err := m.UpdateQuantifierKindPatterns(h, forall, patterns, body)
				if err != nil {
					return nil, err
				}
				v, err = NewExpr(g.context, rebuilt)
				if err != nil {
					return nil, err
				}
			default:
				v, err = NewExpr(g.context, h)
				if err != nil {
					return nil, err
				}
			}
		}
		memo[h] = v
		return v, nil
	}
	root, err := term.Raw()
	if err != nil {
		return nil, err
	}
	localized, err := visit(root)
	if err != nil {
		return nil, err
	}
	return localized.Clone()
}
func (g *RPFP) ReducedDualEdge(edge *Edge) (_ *Expr, result error) {
	if err := g.SetEdgeMaps(edge); err != nil {
		return nil, err
	}
	timer, err := startProfile(g.context, "RedVars")
	if err != nil {
		return nil, err
	}
	b, vars, err := g.RedVars(edge.Parent)
	err = errors.Join(err, timer.Close())
	if err != nil {
		if b != nil {
			err = errors.Join(err, b.Close())
		}
		for _, v := range vars {
			err = errors.Join(err, v.Close())
		}
		return nil, err
	}
	defer func() {
		result = errors.Join(result, b.Close())
		for _, v := range vars {
			result = errors.Join(result, v.Close())
		}
	}()
	formula, err := edge.F.Formula()
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	localized, err := g.Localize(edge, formula)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, localized.Close()) }()
	return b.Implies(localized)
}
