package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

// preparedHornClause owns the canonical body/head and delayed outer-binder
// substitutions used by the active USE_QE_LITE FromClauses path.
type preparedHornClause struct {
	Parent       *Node
	Body, Head   *Expr
	Replacements map[uint32]*Expr
}

func (p *preparedHornClause) close() error {
	if p == nil {
		return nil
	}
	var result error
	if p.Body != nil {
		if err := p.Body.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			p.Body = nil
		}
	}
	if p.Head != nil {
		if err := p.Head.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			p.Head = nil
		}
	}
	for i, e := range p.Replacements {
		if err := e.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			delete(p.Replacements, i)
		}
	}
	return result
}

// prepareHornClause follows the first FromClauses pass: remove one outer
// forall, remember reversed de Bruijn substitutions, lift facts and turn
// noncanonical formulas into negated-body queries. Skolem constants are not
// substituted until after qe-lite in the subsequent edge-construction pass.
func prepareHornClause(input *Expr, simplifier *interp.Manager) (out *preparedHornClause, result error) {
	if err := input.open(); err != nil {
		return nil, err
	}
	c := input.value.object.context
	m := c.manager
	if simplifier == nil {
		return nil, ErrWrapperOwner
	}
	manager, err := simplifier.ASTManager()
	if err != nil {
		return nil, err
	}
	if manager != m {
		return nil, ErrContextMismatch
	}
	out = &preparedHornClause{Replacements: make(map[uint32]*Expr)}
	defer func() {
		if result != nil {
			result = errors.Join(result, out.close())
			out = nil
		}
	}()
	var refs []ast.ASTRef
	keep := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return h, nil
	}
	defer func() {
		for i := range refs {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}()
	h, err := input.Raw()
	if err != nil {
		return out, err
	}
	kind, err := m.GetKind(h)
	if err != nil {
		return out, err
	}
	if kind == ast.ASTQuantifier {
		forall, err := m.IsForall(h)
		if err != nil {
			return out, err
		}
		if forall {
			count, err := m.GetNumDeclarations(h)
			if err != nil {
				return out, err
			}
			for i := uint32(0); i < count; i++ {
				sort, err := m.GetDeclarationSort(h, i)
				if err != nil {
					return out, err
				}
				name, err := m.GetDeclarationName(h, i)
				if err != nil {
					return out, err
				}
				constant, err := m.MkConst(name, sort)
				if err != nil {
					return out, err
				}
				e, err := NewExpr(c, constant)
				if err != nil {
					return out, err
				}
				out.Replacements[count-1-i] = e
			}
			h, err = m.GetQuantifierBody(h)
			if err != nil {
				return out, err
			}
		}
	}
	kind, err = m.GetKind(h)
	if err != nil {
		return out, err
	}
	if kind == ast.ASTApp {
		family, err := m.GetFamilyID(h)
		if err != nil {
			return out, err
		}
		if family == ast.NullFamilyID {
			yes, err := keep(m.MkTrue())
			if err != nil {
				return out, err
			}
			h, err = keep(m.MkImplies(yes, h))
			if err != nil {
				return out, err
			}
		}
	}
	canonical := false
	kind, err = m.GetKind(h)
	if err != nil {
		return out, err
	}
	if kind == ast.ASTApp {
		family, err := m.GetFamilyID(h)
		if err != nil {
			return out, err
		}
		op, err := m.GetDeclKind(h)
		if err != nil {
			return out, err
		}
		if family == m.BasicFamilyID() && op == ast.ASTOpImplies {
			head, err := m.GetArg(h, 1)
			if err != nil {
				return out, err
			}
			kind, err := m.GetKind(head)
			if err != nil {
				return out, err
			}
			if kind == ast.ASTApp {
				family, err := m.GetFamilyID(head)
				if err != nil {
					return out, err
				}
				op, err := m.GetDeclKind(head)
				if err != nil {
					return out, err
				}
				canonical = family == ast.NullFamilyID || (family == m.BasicFamilyID() && op == ast.ASTOpFalse)
			}
		}
	}
	if !canonical {
		neg, err := keep(m.MkNot(h))
		if err != nil {
			return out, err
		}
		ref, err := simplifier.Simplify(neg)
		if err != nil {
			return out, err
		}
		refs = append(refs, ref)
		body, err := m.RefGet(ref)
		if err != nil {
			return out, err
		}
		no, err := keep(m.MkFalse())
		if err != nil {
			return out, err
		}
		h, err = keep(m.MkImplies(body, no))
		if err != nil {
			return out, err
		}
	}
	body, err := m.GetArg(h, 0)
	if err != nil {
		return out, err
	}
	out.Body, err = NewExpr(c, body)
	if err != nil {
		return out, err
	}
	head, err := m.GetArg(h, 1)
	if err != nil {
		return out, err
	}
	out.Head, err = NewExpr(c, head)
	return out, err
}
