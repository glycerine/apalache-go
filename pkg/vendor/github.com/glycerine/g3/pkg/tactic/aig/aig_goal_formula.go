package aig

// Source: src/tactic/aig/aig.cpp:746-938,1010-1033.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type goalAIGKind uint8

const (
	goalAIGAnd goalAIGKind = iota
	goalAIGAuxAnd
	goalAIGITE
)

type goalAIGFrame struct {
	node  *node
	kind  goalAIGKind
	first bool
}
type goalFormulaConverter struct {
	formulaConverter
	frames []goalAIGFrame
}

func (c *goalFormulaConverter) base(n *node) (ast.ASTHandle, error) {
	if n.and {
		if _, found := c.cache[n]; !found {
			c.manager.traceInvalidGoalCache(n)
		}
	}
	return c.formulaConverter.base(n)
}

func (c *goalFormulaConverter) cached(n *node) bool {
	if !n.and {
		return true
	}
	_, ok := c.cache[n]
	return ok
}
func (c *goalFormulaConverter) kind(n *node) goalAIGKind {
	if ok, _, _, _ := iteShape(n); ok {
		return goalAIGITE
	}
	return goalAIGAnd
}
func (c *goalFormulaConverter) visitAndChild(l Literal) bool {
	if c.cached(l.node) {
		return false
	}
	k := c.kind(l.node)
	if k != goalAIGITE && !l.inverted && l.node.refs == 1 {
		k = goalAIGAuxAnd
	}
	c.frames = append(c.frames, goalAIGFrame{node: l.node, kind: k, first: true})
	return true
}
func (c *goalFormulaConverter) visitITEChild(l Literal) bool {
	if c.cached(l.node) {
		return false
	}
	c.frames = append(c.frames, goalAIGFrame{node: l.node, kind: c.kind(l.node), first: true})
	return true
}
func (c *goalFormulaConverter) remember(n *node, h ast.ASTHandle) error {
	owned, err := c.manager.ast.NewRef(h)
	if err != nil {
		return err
	}
	c.cache[n] = owned
	return nil
}

func (c *goalFormulaConverter) buildAnd(n *node) (err error) {
	m := c.manager
	var temps []ast.ASTRef
	defer func() {
		for i := len(temps) - 1; i >= 0; i-- {
			err = errors.Join(err, m.releaseAST(&temps[i]))
		}
	}()
	var children []ast.ASTHandle
	var todo []*node
	addChild := func(l Literal) error {
		if !l.inverted && !c.cached(l.node) {
			if !l.node.and || l.node.refs != 1 {
				return ast.ErrASTDomain
			}
			todo = append(todo, l.node)
			return nil
		}
		base, e := c.base(l.node)
		if e != nil {
			return e
		}
		if !l.inverted {
			base, e = c.invert(base)
			if e != nil {
				return e
			}
		}
		owned, e := m.ast.NewRef(base)
		if e != nil {
			return e
		}
		temps = append(temps, owned)
		children = append(children, base)
		return nil
	}
	if e := addChild(n.children[0]); e != nil {
		return e
	}
	if e := addChild(n.children[1]); e != nil {
		return e
	}
	for len(todo) != 0 {
		last := len(todo) - 1
		child := todo[last]
		todo = todo[:last]
		if e := addChild(child.children[0]); e != nil {
			return e
		}
		if e := addChild(child.children[1]); e != nil {
			return e
		}
	}
	or, e := m.ast.MkOrArray(children)
	if e != nil {
		return e
	}
	orRef, e := m.ast.NewRef(or)
	if e != nil {
		return e
	}
	temps = append(temps, orRef)
	and, e := m.ast.MkNot(or)
	if e != nil {
		return e
	}
	if e := c.remember(n, and); e != nil {
		return e
	}
	m.traceGoalConversion(n, 863, "caching AND ")
	return nil
}

func (c *goalFormulaConverter) literal(l Literal) (ast.ASTHandle, error) {
	h, e := c.base(l.node)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if l.inverted {
		return c.invert(h)
	}
	return h, nil
}
func (c *goalFormulaConverter) buildITE(n *node) (err error) {
	ok, condition, then, otherwise := iteShape(n)
	if !ok {
		return ast.ErrASTDomain
	}
	if condition.inverted {
		condition = invert(condition)
		then, otherwise = otherwise, then
	}
	var temps []ast.ASTRef
	defer func() {
		for i := len(temps) - 1; i >= 0; i-- {
			err = errors.Join(err, c.manager.releaseAST(&temps[i]))
		}
	}()
	items := []Literal{condition, then, otherwise}
	isIFF := opposite(then, otherwise)
	if isIFF {
		items = items[:2]
	}
	var args [3]ast.ASTHandle
	for i, l := range items {
		h, e := c.literal(l)
		if e != nil {
			return e
		}
		owned, e := c.manager.ast.NewRef(h)
		if e != nil {
			return e
		}
		temps = append(temps, owned)
		args[i] = h
	}
	var result ast.ASTHandle
	var e error
	if isIFF {
		result, e = c.manager.ast.MkIff(args[0], args[1])
	} else {
		result, e = c.manager.ast.MkIte(args[0], args[1], args[2])
	}
	if e != nil {
		return e
	}
	if e := c.remember(n, result); e != nil {
		return e
	}
	c.manager.traceGoalConversion(n, 881, "caching ITE/IFF ")
	return nil
}

func (c *goalFormulaConverter) processRoot(n *node) (ast.ASTHandle, error) {
	if c.cached(n) {
		return c.base(n)
	}
	c.frames = append(c.frames, goalAIGFrame{node: n, kind: c.kind(n), first: true})
	for len(c.frames) != 0 {
		if err := c.manager.checkpoint(); err != nil {
			return ast.ASTHandle{}, err
		}
		last := len(c.frames) - 1
		fr := c.frames[last]
		if c.cached(fr.node) {
			c.frames = c.frames[:last]
			continue
		}
		if fr.first {
			c.frames[last].first = false
			visited := false
			switch fr.kind {
			case goalAIGAnd, goalAIGAuxAnd:
				for _, child := range fr.node.children {
					if c.visitAndChild(child) {
						visited = true
					}
				}
			case goalAIGITE:
				left, right := fr.node.children[0].node, fr.node.children[1].node
				a, b := left.children[0], left.children[1]
				cc, d := right.children[0], right.children[1]
				if c.visitITEChild(a) {
					visited = true
				}
				if c.visitITEChild(b) {
					visited = true
				}
				if cc.node != a.node && cc.node != b.node {
					if c.visitITEChild(cc) {
						visited = true
					}
				}
				if d.node != a.node && d.node != b.node {
					if c.visitITEChild(d) {
						visited = true
					}
				}
			}
			if visited {
				continue
			}
		}
		var err error
		switch fr.kind {
		case goalAIGAnd:
			err = c.buildAnd(fr.node)
		case goalAIGITE:
			err = c.buildITE(fr.node)
		case goalAIGAuxAnd:
			c.manager.traceGoalConversion(fr.node, 931, "skipping aux AND ")
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		c.frames = c.frames[:last]
	}
	return c.base(n)
}

// ToGoal follows the source's optimized goal writer: reset, split positive
// top-level ANDs, preserve ITEs, and assert each resulting AST expression.
func (m *Manager) ToGoal(r *Ref, g *tactic.Goal) (err error) {
	l, e := r.literal(m)
	if e != nil {
		return e
	}
	if g == nil || g.Manager() != m.ast {
		return ErrOwner
	}
	if e = g.Reset(); e != nil {
		return e
	}
	c := goalFormulaConverter{formulaConverter: formulaConverter{manager: m, cache: make(map[*node]ast.ASTRef)}}
	defer func() { err = errors.Join(err, c.cleanup()) }()
	roots := []Literal{l}
	for len(roots) != 0 {
		last := len(roots) - 1
		current := roots[last]
		roots = roots[:last]
		if current.inverted {
			h, e := c.processRoot(current.node)
			if e != nil {
				return e
			}
			h, e = c.invert(h)
			if e != nil {
				return e
			}
			owned, e := m.ast.NewRef(h)
			if e != nil {
				return e
			}
			e = g.Assert(h)
			returnErr := m.releaseAST(&owned)
			if e != nil || returnErr != nil {
				return errors.Join(e, returnErr)
			}
			continue
		}
		if ok, _, _, _ := iteShape(current.node); ok {
			h, e := c.processRoot(current.node)
			if e != nil {
				return e
			}
			if e = g.Assert(h); e != nil {
				return e
			}
			continue
		}
		if !current.node.and {
			h, e := c.base(current.node)
			if e != nil {
				return e
			}
			if e = g.Assert(h); e != nil {
				return e
			}
			continue
		}
		roots = append(roots, current.node.children[0], current.node.children[1])
	}
	return nil
}
