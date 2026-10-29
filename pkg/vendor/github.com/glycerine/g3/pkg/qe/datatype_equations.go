package qe

// Source: src/qe/qe_datatypes.cpp recursive solving and foreign-field lifting.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *DatatypeProjection) basicArgs(h ast.ASTHandle, op int32) ([]ast.ASTHandle, bool, error) {
	m := p.manager
	k, err := m.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return nil, false, err
	}
	f, err := m.GetFamilyID(h)
	if err != nil || f != m.BasicFamilyID() {
		return nil, false, err
	}
	kind, err := m.GetDeclKind(h)
	if err != nil || kind != op {
		return nil, false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil {
		return nil, false, err
	}
	args := make([]ast.ASTHandle, n)
	for i := range args {
		args[i], err = m.GetArg(h, uint32(i))
		if err != nil {
			return nil, false, err
		}
	}
	return args, true, nil
}
func (c *datatypeProjectionCall) projectRecursive(md *model.ModelCore, additional, forms *[]ast.ASTRef) error {
	p := c.owner
	m := p.manager
	var eqs []ast.ASTHandle
	for i := 0; i < len(*forms); i++ {
		formula, err := m.RefGet((*forms)[i])
		if err != nil {
			return err
		}
		if _, err = c.ref(formula); err != nil {
			return err
		}
		rhs, solved, err := c.solveFormula(formula, &eqs)
		if err != nil {
			return err
		}
		if solved {
			if err = p.erase(forms, &i); err != nil {
				return err
			}
			if err = c.reduce(rhs, forms); err != nil {
				return err
			}
			for _, eq := range eqs {
				if err = p.appendRef(forms, eq); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return c.projectNonrecursive(md, additional, forms)
}
func (c *datatypeProjectionCall) solveFormula(formula ast.ASTHandle, eqs *[]ast.ASTHandle) (ast.ASTHandle, bool, error) {
	p := c.owner
	m := p.manager
	args, eq, err := p.basicArgs(formula, ast.ASTOpEq)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if eq && len(args) == 2 {
		left, err := p.variable.Check(args[0])
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		right, err := p.variable.Check(args[1])
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if left && !right {
			k, err := m.GetKind(args[0])
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			if k == ast.ASTApp {
				return c.solveApplication(args[0], args[1], eqs)
			}
		}
		if right && !left {
			k, err := m.GetKind(args[1])
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			if k == ast.ASTApp {
				return c.solveApplication(args[1], args[0], eqs)
			}
		}
	}
	args, negative, err := p.basicArgs(formula, ast.ASTOpNot)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if negative && len(args) == 1 {
		values, distinct, err := p.basicArgs(args[0], ast.ASTOpDistinct)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if distinct {
			out, err := c.ref(ast.ASTHandle{})
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			if err = pickProjectionEquality(m, c.evaluator, values, &c.refs, out); err != nil {
				return ast.ASTHandle{}, false, err
			}
			h, err := m.RefGet(out)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			return c.solveFormula(h, eqs)
		}
	}
	return ast.ASTHandle{}, false, nil
}
func (c *datatypeProjectionCall) access(constructor ast.ASTHandle, i uint32, accessors []ast.ASTHandle, value ast.ASTHandle) (ast.ASTHandle, error) {
	m := c.owner.manager
	k, err := m.GetKind(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if k == ast.ASTApp {
		d, err := m.GetDecl(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if d == constructor {
			return m.GetArg(value, i)
		}
	}
	if uint64(i) >= uint64(len(accessors)) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return c.hold(m.MkApp(accessors[i], []ast.ASTHandle{value}))
}
func (c *datatypeProjectionCall) solveApplication(a, b ast.ASTHandle, eqs *[]ast.ASTHandle) (ast.ASTHandle, bool, error) {
	p := c.owner
	m := p.manager
	target, err := p.variable.Target()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if a == target {
		return b, true, nil
	}
	constructor, yes, err := m.DatatypeConstructorApp(a)
	if err != nil || !yes {
		return ast.ASTHandle{}, false, err
	}
	rec, err := c.hold(m.DatatypeConstructorRecognizer(constructor))
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	accessors, err := m.DatatypeConstructorAccessors(constructor)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	for _, d := range accessors {
		if _, err = c.ref(d); err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	n, err := m.GetNumArgs(a)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if uint64(n) != uint64(len(accessors)) {
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	for i := uint32(0); i < n; i++ {
		left, err := m.GetArg(a, i)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		kind, err := m.GetKind(left)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		contains, err := p.variable.Check(left)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if !contains {
			continue
		}
		right, err := c.access(constructor, i, accessors, b)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		rhs, solved, err := c.solveApplication(left, right, eqs)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if !solved {
			continue
		}
		for j := uint32(0); j < n; j++ {
			if i == j {
				continue
			}
			r, err := c.access(constructor, j, accessors, b)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			l, err := m.GetArg(a, j)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			eq, err := c.hold(m.MkEq(r, l))
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			*eqs = append(*eqs, eq)
		}
		kind, err = m.GetKind(b)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		same := false
		if kind == ast.ASTApp {
			d, err := m.GetDecl(b)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			same = d == constructor
		}
		if !same {
			test, err := c.hold(m.MkApp(rec, []ast.ASTHandle{b}))
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			*eqs = append(*eqs, test)
		}
		return rhs, true, nil
	}
	return ast.ASTHandle{}, false, nil
}

// Solve is the datatype plugin's cheap foreign-field lifting pass. It neither
// removes variables nor updates the supplied model, matching the source hook.
func (p *DatatypeProjection) Solve(md *model.ModelCore, vars, forms *[]ast.ASTRef) (changed bool, err error) {
	c, err := p.begin()
	if err != nil {
		return false, err
	}
	defer p.finish(c, &err)
	if md == nil || md.Manager() != p.manager || vars == nil || forms == nil {
		return false, ast.ErrASTHandle
	}
	m := p.manager
	visited := make(map[ast.ASTHandle]bool)
	hasVariable := make(map[ast.ASTHandle]bool)
	inserted := false
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return false, err
	}
	for _, r := range *vars {
		h, e := m.RefGet(r)
		if e != nil {
			return false, e
		}
		if _, e = m.GetDecl(h); e != nil {
			return false, e
		}
		sort, e := m.GetSort(h)
		if e != nil {
			return false, e
		}
		if sort == boolSort {
			continue
		}
		datatype, e := p.datatype(h)
		if e != nil {
			return false, e
		}
		if datatype {
			continue
		}
		inserted = true
		visited[h] = true
		hasVariable[h] = true
	}
	if !inserted {
		return false, nil
	}
	for i := 0; i < len(*forms); i++ {
		h, e := m.RefGet((*forms)[i])
		if e != nil {
			return changed, e
		}
		args, eq, e := p.basicArgs(h, ast.ASTOpEq)
		if e != nil {
			return changed, e
		}
		if !eq || len(args) != 2 {
			continue
		}
		if _, e = c.ref(h); e != nil {
			return changed, e
		}
		reduced, e := c.reduceForeignEquality(visited, hasVariable, args[0], args[1], forms)
		if e != nil {
			return changed, e
		}
		if reduced {
			if e = p.erase(forms, &i); e != nil {
				return changed, e
			}
			changed = true
		}
	}
	if changed {
		if err = p.emit("lift_foreign", 214, func(w io.Writer) error {
			if e := writeExpressionVector(m, p.config, w, *vars); e != nil {
				return e
			}
			_, _ = io.WriteString(w, "\n")
			if e := writeExpressionVector(m, p.config, w, *forms); e != nil {
				return e
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		}); err != nil {
			return changed, err
		}
	}
	return changed, nil
}
func (c *datatypeProjectionCall) reduceForeignEquality(visited, hasVariable map[ast.ASTHandle]bool, left, right ast.ASTHandle, forms *[]ast.ASTRef) (bool, error) {
	p := c.owner
	m := p.manager
	for _, h := range []ast.ASTHandle{left, right} {
		k, err := m.GetKind(h)
		if err != nil {
			return false, err
		}
		if k != ast.ASTApp {
			return false, nil
		}
	}
	reduce := false
	_, yes, err := m.DatatypeConstructorApp(right)
	if err != nil {
		return false, err
	}
	if yes {
		contains, err := c.containsForeign(visited, hasVariable, right)
		if err != nil {
			return false, err
		}
		if contains {
			left, right = right, left
			reduce = true
		}
	}
	constructor, yes, err := m.DatatypeConstructorApp(left)
	if err != nil {
		return false, err
	}
	if yes {
		contains, err := c.containsForeign(visited, hasVariable, left)
		if err != nil {
			return false, err
		}
		reduce = reduce || contains
	}
	if !reduce {
		return false, nil
	}
	accessors, err := m.DatatypeConstructorAccessors(constructor)
	if err != nil {
		return false, err
	}
	for _, d := range accessors {
		if _, err = c.ref(d); err != nil {
			return false, err
		}
	}
	rightDecl, err := m.GetDecl(right)
	if err != nil {
		return false, err
	}
	if rightDecl != constructor {
		rec, err := c.hold(m.DatatypeConstructorRecognizer(constructor))
		if err != nil {
			return false, err
		}
		test, err := c.hold(m.MkApp(rec, []ast.ASTHandle{right}))
		if err != nil {
			return false, err
		}
		if err = p.appendRef(forms, test); err != nil {
			return false, err
		}
	}
	for i := range accessors {
		l, err := m.GetArg(left, uint32(i))
		if err != nil {
			return false, err
		}
		r, err := c.access(constructor, uint32(i), accessors, right)
		if err != nil {
			return false, err
		}
		eq, err := c.hold(m.MkEq(l, r))
		if err != nil {
			return false, err
		}
		if err = p.appendRef(forms, eq); err != nil {
			return false, err
		}
	}
	return true, nil
}
func (c *datatypeProjectionCall) containsForeign(visited, hasVariable map[ast.ASTHandle]bool, root ast.ASTHandle) (bool, error) {
	p := c.owner
	m := p.manager
	todo := []ast.ASTHandle{root}
	for len(todo) != 0 {
		h := todo[len(todo)-1]
		if visited[h] {
			todo = todo[:len(todo)-1]
			continue
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			visited[h] = true
			todo = todo[:len(todo)-1]
			continue
		}
		n, err := m.GetNumArgs(h)
		if err != nil {
			return false, err
		}
		more, contains := false, false
		for i := uint32(0); i < n; i++ {
			arg, err := m.GetArg(h, i)
			if err != nil {
				return false, err
			}
			if !visited[arg] {
				todo = append(todo, arg)
				more = true
			} else {
				contains = contains || hasVariable[arg]
			}
		}
		if more {
			continue
		}
		todo = todo[:len(todo)-1]
		if contains {
			hasVariable[h] = true
		}
		if err = p.emit("contains_foreign", 278, func(w io.Writer) error {
			_, _ = io.WriteString(w, "contains: ")
			if e := writeExpression(m, p.config, w, h); e != nil {
				return e
			}
			flag := 0
			if hasVariable[h] {
				flag = 1
			}
			_, _ = fmt.Fprintf(w, " %d\n", flag)
			return nil
		}); err != nil {
			return false, err
		}
		visited[h] = true
	}
	if err := p.emit("contains_foreign", 281, func(w io.Writer) error {
		_, _ = io.WriteString(w, "contains: ")
		if e := writeExpression(m, p.config, w, root); e != nil {
			return e
		}
		flag := 0
		if hasVariable[root] {
			flag = 1
		}
		_, _ = fmt.Fprintf(w, " %d\n", flag)
		return nil
	}); err != nil {
		return false, err
	}
	return hasVariable[root], nil
}
