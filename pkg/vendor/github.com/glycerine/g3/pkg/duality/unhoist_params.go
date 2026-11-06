package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// AddParams appends independently owned individual parameters without
// substituting the formula (AddParamsToTransformer in duality_rpfp.cpp).
func (t *Transformer) AddParams(params []*Expr) (result error) {
	if err := t.open(); err != nil {
		return err
	}
	for _, p := range params {
		if err := p.open(); err != nil {
			return err
		}
		if p.value.object.context != t.context {
			return ErrContextMismatch
		}
	}
	var refs []ast.ASTRef
	defer func() {
		for i := range refs {
			result = errors.Join(result, t.context.manager.DelRef(&refs[i]))
		}
	}()
	for _, p := range params {
		ref, err := t.context.manager.CopyRef(p.value.ref)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
	}
	t.individualParams = append(t.individualParams, refs...)
	refs = nil
	return nil
}

// AddParamsToApp applies a replacement declaration to the old arguments
// followed by params. The caller owns the result.
// Source: duality_rpfp.cpp, AddParamsToApp, live UnhoistLoop consumer.
func AddParamsToApp(app *Expr, decl *FuncDecl, params []*Expr) (*Expr, error) {
	if err := app.open(); err != nil {
		return nil, err
	}
	if err := decl.open(); err != nil {
		return nil, err
	}
	c := app.value.object.context
	if decl.value.object.context != c {
		return nil, ErrContextMismatch
	}
	m := c.manager
	root, err := app.Raw()
	if err != nil {
		return nil, err
	}
	n, err := m.GetNumArgs(root)
	if err != nil {
		return nil, err
	}
	args := make([]ast.ASTHandle, 0, int(n)+len(params))
	for i := uint32(0); i < n; i++ {
		h, err := m.GetArg(root, i)
		if err != nil {
			return nil, err
		}
		args = append(args, h)
	}
	for _, p := range params {
		if err := p.open(); err != nil {
			return nil, err
		}
		if p.value.object.context != c {
			return nil, ErrContextMismatch
		}
		h, err := p.Raw()
		if err != nil {
			return nil, err
		}
		args = append(args, h)
	}
	d, err := decl.Raw()
	if err != nil {
		return nil, err
	}
	h, err := m.MkApp(d, args)
	if err != nil {
		return nil, err
	}
	return NewExpr(c, h)
}

// AddParamsToNode gives the node a fresh predicate of extended arity and
// appends params to all three relations. Existing incoming edges are updated
// separately by UnhoistLoop, matching the source mutation order.
func (g *RPFP) AddParamsToNode(node *Node, params []*Expr) error {
	if err := g.open(); err != nil {
		return err
	}
	if node == nil || node.closed || node.Owner != g {
		return ErrWrapperOwner
	}
	for _, p := range params {
		if err := p.open(); err != nil {
			return err
		}
		if p.value.object.context != g.context {
			return ErrContextMismatch
		}
	}
	m := g.context.manager
	domain := make([]ast.ASTHandle, 0, len(node.Annotation.individualParams)+len(params))
	for _, ref := range node.Annotation.individualParams {
		h, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		sort, err := m.GetSort(h)
		if err != nil {
			return err
		}
		domain = append(domain, sort)
	}
	for _, p := range params {
		h, err := p.Raw()
		if err != nil {
			return err
		}
		sort, err := m.GetSort(h)
		if err != nil {
			return err
		}
		domain = append(domain, sort)
	}
	old, err := m.RefGet(node.Name)
	if err != nil {
		return err
	}
	name, err := m.GetASTName(old)
	if err != nil {
		return err
	}
	// The wrapper converts numerical symbols through its signed string form.
	symbol, err := newSymbol(g.context, name)
	if err != nil {
		return err
	}
	text, err := symbol.String()
	if err != nil {
		return err
	}
	prefix, err := m.Symbols.Intern(text)
	if err != nil {
		return err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	fresh, err := m.MkFreshFuncDecl(prefix, usymbol.Symbol{}, domain, boolean)
	if err != nil {
		return err
	}
	if err := m.SetRef(node.Name, fresh); err != nil {
		return err
	}
	for _, relation := range []*Transformer{node.Annotation, node.Bound, node.Underapprox} {
		if err := relation.AddParams(params); err != nil {
			return err
		}
	}
	return nil
}
