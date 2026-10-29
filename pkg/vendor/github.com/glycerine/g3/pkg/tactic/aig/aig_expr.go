package aig

// Source: src/tactic/aig/aig.cpp:420-644,1468-1480,1688-1697.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type exprFrame struct {
	expr  ast.ASTHandle
	op    int32
	args  []ast.ASTHandle
	next  int
	start int
	cache bool
}

type exprConverter struct {
	manager *Manager
	frames  []exprFrame
	results []*Ref
	cache   map[ast.ASTHandle]*Ref
}

func (c *exprConverter) pushCached(h ast.ASTHandle) (bool, error) {
	if old := c.cache[h]; old != nil {
		copy, err := old.Clone()
		if err != nil {
			return false, err
		}
		c.results = append(c.results, copy)
		return true, nil
	}
	return false, nil
}

func (c *exprConverter) variable(h ast.ASTHandle) error {
	if found, err := c.pushCached(h); found || err != nil {
		return err
	}
	r, err := c.manager.MakeVariable(h)
	if err != nil {
		return err
	}
	if err = c.manager.traceExprCacheResult(h, r.lit); err != nil {
		return errors.Join(err, r.Close())
	}
	owned, err := r.Clone()
	if err != nil {
		return errors.Join(err, r.Close())
	}
	c.cache[h] = owned
	c.results = append(c.results, r)
	return nil
}

// visit appends a leaf result or pushes one source frame for a Boolean app.
func (c *exprConverter) visit(h ast.ASTHandle) error {
	m := c.manager.ast
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return c.variable(h)
	}
	family, err := m.GetFamilyID(h)
	if err != nil {
		return err
	}
	if family != m.BasicFamilyID() {
		return c.variable(h)
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return err
	}
	switch op {
	case ast.ASTOpTrue:
		r, err := c.manager.True()
		if err == nil {
			c.results = append(c.results, r)
		}
		return err
	case ast.ASTOpFalse:
		r, err := c.manager.False()
		if err == nil {
			c.results = append(c.results, r)
		}
		return err
	case ast.ASTOpEq:
		arg, err := m.GetArg(h, 0)
		if err != nil {
			return err
		}
		sort, err := m.GetSort(arg)
		if err != nil {
			return err
		}
		boolean, err := m.MkBoolSort()
		if err != nil {
			return err
		}
		if sort != boolean {
			return c.variable(h)
		}
	case ast.ASTOpNot, ast.ASTOpOr, ast.ASTOpAnd, ast.ASTOpIff, ast.ASTOpXor, ast.ASTOpImplies, ast.ASTOpIte:
	default:
		return c.variable(h)
	}
	count, err := m.GetRefCount(h)
	if err != nil {
		return err
	}
	if count > 1 {
		if found, err := c.pushCached(h); found || err != nil {
			return err
		}
	}
	n, err := m.GetNumArgs(h)
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, n)
	for i := range args {
		args[i], err = m.GetArg(h, uint32(i))
		if err != nil {
			return err
		}
	}
	c.frames = append(c.frames, exprFrame{expr: h, op: op, args: args, start: len(c.results), cache: count > 1})
	return nil
}

func (c *exprConverter) build(fr exprFrame) (*Ref, error) {
	args := c.results[fr.start:]
	m := c.manager
	switch fr.op {
	case ast.ASTOpNot:
		if len(args) != 1 {
			return nil, ast.ErrASTDomain
		}
		return args[0].Not()
	case ast.ASTOpAnd:
		return m.AndArgs(args)
	case ast.ASTOpOr:
		return m.OrArgs(args)
	case ast.ASTOpEq, ast.ASTOpIff:
		if len(args) != 2 {
			return nil, ast.ErrASTDomain
		}
		return m.Iff(args[0], args[1])
	case ast.ASTOpXor:
		if len(args) != 2 {
			return nil, ast.ErrASTDomain
		}
		return m.Xor(args[0], args[1])
	case ast.ASTOpImplies:
		if len(args) != 2 {
			return nil, ast.ErrASTDomain
		}
		return m.Implies(args[0], args[1])
	case ast.ASTOpIte:
		if len(args) != 3 {
			return nil, ast.ErrASTDomain
		}
		return m.Ite(args[0], args[1], args[2])
	}
	return nil, ast.ErrASTDomain
}

func (c *exprConverter) complete(fr exprFrame) error {
	r, err := c.build(fr)
	if err != nil {
		return err
	}
	if fr.cache {
		if e := c.manager.traceExprCacheResult(fr.expr, r.lit); e != nil {
			return errors.Join(e, r.Close())
		}
		copy, e := r.Clone()
		if e != nil {
			return errors.Join(e, r.Close())
		}
		c.cache[fr.expr] = copy
	}
	for i := fr.start; i < len(c.results); i++ {
		err = errors.Join(err, c.results[i].Close())
	}
	c.results = c.results[:fr.start]
	if err != nil {
		return errors.Join(err, r.Close())
	}
	c.results = append(c.results, r)
	return nil
}

func (c *exprConverter) cleanup() error {
	var err error
	for _, r := range c.cache {
		c.manager.traceExprCacheDecRef(r.lit)
		err = errors.Join(err, r.Close())
	}
	for i := len(c.results) - 1; i >= 0; i-- {
		err = errors.Join(err, c.results[i].Close())
	}
	c.results = nil
	c.cache = nil
	c.frames = nil
	return err
}

func (c *exprConverter) convertOne(h ast.ASTHandle) (*Ref, error) {
	if len(c.frames) != 0 || len(c.results) != 0 {
		return nil, ast.ErrASTDomain
	}
	err := c.visit(h)
	for err == nil && len(c.frames) != 0 {
		if err = c.manager.checkpoint(); err != nil {
			break
		}
		last := len(c.frames) - 1
		fr := &c.frames[last]
		if fr.next < len(fr.args) {
			arg := fr.args[fr.next]
			fr.next++
			err = c.visit(arg)
			continue
		}
		finished := *fr
		err = c.complete(finished)
		c.frames = c.frames[:last]
	}
	var out *Ref
	if err == nil {
		if len(c.results) != 1 {
			err = ast.ErrASTDomain
		} else {
			out = c.results[0]
			c.results = nil
		}
	}
	return out, err
}

// MkAIG converts one borrowed AST expression using the source's explicit
// postorder frame/result stacks and expression-identity cache. The caller
// keeps the input AST live until this call returns.
func (m *Manager) MkAIG(h ast.ASTHandle) (*Ref, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	c := exprConverter{manager: m, cache: make(map[ast.ASTHandle]*Ref)}
	out, err := c.convertOne(h)
	err = errors.Join(err, c.cleanup())
	if err != nil && out != nil {
		err = errors.Join(err, out.Close())
		out = nil
	}
	return out, err
}
