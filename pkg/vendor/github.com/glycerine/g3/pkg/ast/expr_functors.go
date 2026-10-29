package ast

// Source: src/ast/expr_functors.cpp; src/ast/expr_functors.h.

// ExprPredicate is the checked Go counterpart of the source i_expr_pred.
type ExprPredicate func(ASTHandle) (bool, error)

// SortPredicate is the checked Go counterpart of the source i_sort_pred.
type SortPredicate func(ASTHandle) (bool, error)

// CheckPredicate memoizes whether a predicate holds at or below an expression.
type CheckPredicate struct {
	self      *CheckPredicate
	manager   *ASTManager
	predicate ExprPredicate
	holds     map[ASTHandle]struct{}
	visited   map[ASTHandle]struct{}
	refs      []ASTRef
	closed    bool
}

func NewCheckPredicate(m *ASTManager, predicate ExprPredicate) (*CheckPredicate, error) {
	if m == nil || predicate == nil {
		return nil, ErrASTHandle
	}
	if err := m.lock(); err != nil {
		return nil, err
	}
	m.mu.Unlock()
	c := &CheckPredicate{manager: m, predicate: predicate, holds: make(map[ASTHandle]struct{}), visited: make(map[ASTHandle]struct{})}
	c.self = c
	return c, nil
}

func (c *CheckPredicate) open() error {
	if c == nil || c.self != c || c.closed || c.manager == nil || c.predicate == nil {
		return ErrASTHandle
	}
	return nil
}

func (c *CheckPredicate) Check(root ASTHandle) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if _, err := c.manager.GetKind(root); err != nil {
		return false, err
	}
	if _, ok := c.visited[root]; !ok {
		ref, err := c.manager.NewRef(root)
		if err != nil {
			return false, err
		}
		c.refs = append(c.refs, ref)
		if err = c.visit(root); err != nil {
			return false, err
		}
	}
	_, result := c.holds[root]
	return result, nil
}

func (c *CheckPredicate) visit(root ASTHandle) error {
	stack := []ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		h := stack[last]
		yes, err := c.predicate(h)
		if err != nil {
			return err
		}
		if yes {
			c.holds[h] = struct{}{}
		}
		if _, ok := c.visited[h]; ok {
			stack = stack[:last]
			continue
		}
		kind, err := c.manager.GetKind(h)
		if err != nil {
			return err
		}
		switch kind {
		case ASTApp:
			n, e := c.manager.GetNumArgs(h)
			if e != nil {
				return e
			}
			allVisited := true
			for i := uint32(0); i < n; i++ {
				child, childErr := c.manager.GetArg(h, i)
				if childErr != nil {
					return childErr
				}
				if _, ok := c.visited[child]; !ok {
					stack = append(stack, child)
					allVisited = false
				} else if _, ok = c.holds[child]; ok {
					c.holds[h] = struct{}{}
				}
			}
			if allVisited {
				c.visited[h] = struct{}{}
				stack = stack[:last]
			}
		case ASTQuantifier:
			body, e := c.manager.GetQuantifierBody(h)
			if e != nil {
				return e
			}
			if _, ok := c.visited[body]; ok {
				stack = stack[:last]
				if _, ok = c.holds[body]; ok {
					c.holds[h] = struct{}{}
				}
				c.visited[h] = struct{}{}
			} else {
				stack = append(stack, body)
			}
		case ASTVar:
			stack = stack[:last]
			c.visited[h] = struct{}{}
		default:
			return ErrASTDomain
		}
	}
	return nil
}

func (c *CheckPredicate) Reset() error {
	if err := c.open(); err != nil {
		return err
	}
	c.holds = make(map[ASTHandle]struct{})
	c.visited = make(map[ASTHandle]struct{})
	for len(c.refs) != 0 {
		last := len(c.refs) - 1
		if err := c.manager.DelRef(&c.refs[last]); err != nil {
			return err
		}
		c.refs = c.refs[:last]
	}
	return nil
}

func (c *CheckPredicate) Close() error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.Reset(); err != nil {
		return err
	}
	c.closed = true
	c.predicate = nil
	return nil
}

// ContainsApp memoizes whether its owned target application occurs below an
// expression or any expression in a slice.
type ContainsApp struct {
	self    *ContainsApp
	manager *ASTManager
	target  ASTRef
	check   *CheckPredicate
	closed  bool
}

func NewContainsApp(m *ASTManager, target ASTHandle) (*ContainsApp, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	kind, err := m.GetKind(target)
	if err != nil {
		return nil, err
	}
	if kind != ASTApp {
		return nil, ErrASTDomain
	}
	ref, err := m.NewRef(target)
	if err != nil {
		return nil, err
	}
	c := &ContainsApp{manager: m, target: ref}
	c.self = c
	c.check, err = NewCheckPredicate(m, func(h ASTHandle) (bool, error) {
		x, e := m.RefGet(c.target)
		return h == x, e
	})
	if err != nil {
		_ = m.DelRef(&c.target)
		return nil, err
	}
	return c, nil
}

func (c *ContainsApp) open() error {
	if c == nil || c.self != c || c.closed || c.manager == nil {
		return ErrASTHandle
	}
	return nil
}

func (c *ContainsApp) Check(h ASTHandle) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if c.check == nil {
		return false, ErrASTHandle
	}
	return c.check.Check(h)
}

func (c *ContainsApp) CheckSlice(expressions []ASTHandle) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if c.check == nil {
		return false, ErrASTHandle
	}
	for _, h := range expressions {
		yes, err := c.check.Check(h)
		if err != nil || yes {
			return yes, err
		}
	}
	return false, nil
}

func (c *ContainsApp) Target() (ASTHandle, error) {
	if err := c.open(); err != nil {
		return ASTHandle{}, err
	}
	return c.manager.RefGet(c.target)
}

func (c *ContainsApp) Close() error {
	if err := c.open(); err != nil {
		return err
	}
	if c.check != nil {
		if err := c.check.Close(); err != nil {
			return err
		}
		c.check = nil
	}
	if err := c.manager.DelRef(&c.target); err != nil {
		return err
	}
	c.closed = true
	return nil
}

// ASTMapProc provides the source bottom-up expression reconstruction cache.
type ASTMapProc struct {
	self    *ASTMapProc
	manager *ASTManager
	mapping *ASTExprMap
	args    []ASTHandle
	closed  bool
}

func NewASTMapProc(m *ASTManager) (*ASTMapProc, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	mapping, err := NewASTExprMap(m)
	if err != nil {
		return nil, err
	}
	p := &ASTMapProc{manager: m, mapping: mapping}
	p.self = p
	return p, nil
}

func (p *ASTMapProc) open() error {
	if p == nil || p.self != p || p.closed || p.manager == nil || p.mapping == nil {
		return ErrASTHandle
	}
	return nil
}

func (p *ASTMapProc) Reset() error {
	if err := p.open(); err != nil {
		return err
	}
	p.args = p.args[:0]
	return p.mapping.Reset()
}

func (p *ASTMapProc) Insert(source, replacement ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	return p.mapping.Insert(source, replacement, ASTHandle{})
}

func (p *ASTMapProc) VisitVar(variable ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	kind, err := p.manager.GetKind(variable)
	if err != nil {
		return err
	}
	if kind != ASTVar {
		return ErrASTDomain
	}
	return p.mapping.Insert(variable, variable, ASTHandle{})
}

func (p *ASTMapProc) VisitQuantifier(q ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	body, err := p.manager.GetQuantifierBody(q)
	if err != nil {
		return err
	}
	rewritten, _, found, err := p.mapping.Get(body)
	if err != nil {
		return err
	}
	if !found {
		return ErrASTDomain
	}
	updated, err := p.manager.UpdateQuantifierBody(q, rewritten)
	if err != nil {
		return err
	}
	return p.mapping.Insert(q, updated, ASTHandle{})
}

func (p *ASTMapProc) Reconstruct(app ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	kind, err := p.manager.GetKind(app)
	if err != nil {
		return err
	}
	if kind != ASTApp {
		return ErrASTDomain
	}
	p.args = p.args[:0]
	n, err := p.manager.GetNumArgs(app)
	if err != nil {
		return err
	}
	changed := false
	for i := uint32(0); i < n; i++ {
		child, e := p.manager.GetArg(app, i)
		if e != nil {
			return e
		}
		rewritten, _, found, e := p.mapping.Get(child)
		if e != nil {
			return e
		}
		if !found {
			return ErrASTDomain
		}
		p.args = append(p.args, rewritten)
		changed = changed || child != rewritten
	}
	result := app
	if changed {
		decl, e := p.manager.GetDecl(app)
		if e != nil {
			return e
		}
		result, err = p.manager.MkApp(decl, p.args)
		if err != nil {
			return err
		}
	}
	return p.mapping.Insert(app, result, ASTHandle{})
}

func (p *ASTMapProc) GetExpr(h ASTHandle) (ASTHandle, bool, error) {
	if err := p.open(); err != nil {
		return ASTHandle{}, false, err
	}
	value, _, found, err := p.mapping.Get(h)
	return value, found, err
}

func (p *ASTMapProc) Close() error {
	if err := p.open(); err != nil {
		return err
	}
	err := p.mapping.Close()
	if err == nil {
		p.closed = true
		p.args = nil
	}
	return err
}
