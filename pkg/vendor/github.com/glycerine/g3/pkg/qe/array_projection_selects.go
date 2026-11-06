package qe

// Source: src/qe/qe_arrays.cpp select occurrence checking/Ackermann reduction.

import "github.com/glycerine/g3/pkg/ast"

func (p *ArrayProjection) update(h ast.ASTHandle) (bool, error) {
	target, err := p.variable.Target()
	if err != nil {
		return false, err
	}
	if h == target {
		return true, nil
	}
	args, store, err := p.operation(h, p.family, ast.ASTArrayOpStore)
	if err != nil {
		return false, err
	}
	if store && len(args) > 0 {
		contains, err := p.variable.Check(args[0])
		if err != nil {
			return false, err
		}
		if contains {
			for _, arg := range args[1:] {
				yes, err := p.variable.Check(arg)
				if err != nil {
					return false, err
				}
				if yes {
					return false, nil
				}
			}
		}
	}
	// The source do/while(false) exits after assigning the store base; it
	// never checks that base on another iteration. Only the bare target wins.
	return false, nil
}
func (p *ArrayProjection) disequality(h ast.ASTHandle) (bool, error) {
	args, negative, err := p.operation(h, p.manager.BasicFamilyID(), ast.ASTOpNot)
	if err != nil || !negative || len(args) != 1 {
		return false, err
	}
	args, eq, err := p.operation(args[0], p.manager.BasicFamilyID(), ast.ASTOpEq)
	if err != nil || !eq || len(args) != 2 {
		return false, err
	}
	left, err := p.variable.Check(args[0])
	if err != nil {
		return false, err
	}
	right, err := p.variable.Check(args[1])
	if err != nil {
		return false, err
	}
	if left && !right {
		return p.update(args[0])
	}
	if right && !left {
		return p.update(args[1])
	}
	return false, nil
}
func (c *arrayProjectionCall) checkDisequalities(lits *[]ast.ASTRef) ([]ast.ASTHandle, bool, error) {
	p := c.owner
	m := p.manager
	var todo, selects []ast.ASTHandle
	seen := make(map[ast.ASTHandle]bool)
	for _, r := range *lits {
		h, err := m.RefGet(r)
		if err != nil {
			return nil, false, err
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return nil, false, err
		}
		if kind != ast.ASTApp {
			return nil, false, ast.ErrASTDomain
		}
		skip, err := p.disequality(h)
		if err != nil {
			return nil, false, err
		}
		if skip {
			continue
		}
		contains, err := p.variable.Check(h)
		if err != nil {
			return nil, false, err
		}
		if contains {
			todo = append(todo, h)
		}
	}
	target, err := p.variable.Target()
	if err != nil {
		return nil, false, err
	}
	for len(todo) != 0 {
		h := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		if h == target {
			return selects, false, nil
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return nil, false, err
		}
		if kind != ast.ASTApp {
			return nil, false, ast.ErrASTDomain
		}
		n, err := m.GetNumArgs(h)
		if err != nil {
			return nil, false, err
		}
		start := uint32(0)
		args, selectOp, err := p.operation(h, p.family, ast.ASTArrayOpSelect)
		if err != nil {
			return nil, false, err
		}
		if selectOp && len(args) > 0 && args[0] == target {
			start = 1
			if _, err = c.ref(h); err != nil {
				return nil, false, err
			}
			selects = append(selects, h)
		}
		for i := start; i < n; i++ {
			child, err := m.GetArg(h, i)
			if err != nil {
				return nil, false, err
			}
			todo = append(todo, child)
		}
	}
	return selects, true, nil
}
func (c *arrayProjectionCall) ackermannize(selects []ast.ASTHandle, additional, lits *[]ast.ASTRef) error {
	if len(selects) == 0 {
		return nil
	}
	p := c.owner
	m := p.manager
	sub, err := c.replacement()
	if err != nil {
		return err
	}
	var representatives []ast.ASTHandle
	for _, selectTerm := range selects {
		sort, err := m.GetSort(selectTerm)
		if err != nil {
			return err
		}
		fresh, err := c.hold(m.MkFreshConst("sel", sort))
		if err != nil {
			return err
		}
		value, err := c.eval(selectTerm)
		if err != nil {
			return err
		}
		if _, err = m.GetDecl(value); err != nil {
			return err
		}
		decl, err := m.GetDecl(fresh)
		if err != nil {
			return err
		}
		if err = c.model.RegisterConstant(decl, value); err != nil {
			return err
		}
		representatives = append(representatives, value)
		if err = p.appendRef(additional, fresh); err != nil {
			return err
		}
		if err = sub.Insert(selectTerm, value); err != nil {
			return err
		}
	}
	for _, r := range *lits {
		if err = sub.ApplyRef(r); err != nil {
			return err
		}
	}
	for i := 0; i < len(*lits); i++ {
		h, err := m.RefGet((*lits)[i])
		if err != nil {
			return err
		}
		if m.IsTrue(h) {
			if err = p.erase(lits, &i); err != nil {
				return err
			}
		}
	}
	if err = partitionProjectionArguments(m, c.evaluator, selects, &c.refs, lits); err != nil {
		return err
	}
	return partitionProjectionValues(m, c.evaluator, representatives, &c.refs, lits)
}
