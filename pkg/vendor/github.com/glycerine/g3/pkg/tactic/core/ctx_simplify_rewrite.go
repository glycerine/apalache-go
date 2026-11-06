package core

// Source: src/tactic/core/ctx_simplify_tactic.cpp imp traversal and scoped cache.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *ContextSimplifyTactic) cacheResult(from, to ast.ASTHandle) (err error) {
	r := t.current
	if err = t.traceShared(from); err != nil {
		return err
	}
	shared, err := ContextSimplifierShared(t.manager, r.occs, from)
	if err != nil || !shared {
		return err
	}
	if err = t.traceCache(from, to); err != nil {
		return err
	}
	n := uint64(reflect.TypeOf(contextCacheResult{}).Size())
	if err = t.manager.Budget().Reserve(n); err != nil {
		return err
	}
	f := &contextSimplifyRefs{m: t.manager}
	defer t.release(f, &err)
	published := false
	defer func() {
		if !published {
			t.manager.Budget().Release(n)
		}
	}()
	cell := r.cache[from]
	if cell == nil {
		key, e := f.newRef(from)
		if e != nil {
			return e
		}
		cell = &contextCacheCell{key: key}
	}
	value, err := f.newRef(to)
	if err != nil {
		return err
	}
	level := r.simp.ScopeLevel()
	cell.results = append(cell.results, contextCacheResult{value: to, ref: value, level: level, bytes: n})
	r.cache[from] = cell
	for len(r.undo) <= int(level) {
		r.undo = append(r.undo, nil)
	}
	r.undo[level] = append(r.undo[level], from)
	f.refs = nil
	published = true
	return nil
}
func (t *ContextSimplifyTactic) restoreCache(r *contextSimplifyImplementation, level uint32) error {
	if int(level) >= len(r.undo) {
		return nil
	}
	for len(r.undo[level]) > 0 {
		i := len(r.undo[level]) - 1
		key := r.undo[level][i]
		cell := r.cache[key]
		if cell == nil || len(cell.results) == 0 {
			return ast.ErrASTDomain
		}
		j := len(cell.results) - 1
		entry := &cell.results[j]
		if entry.level != level {
			return ast.ErrASTDomain
		}
		if t.traceTags().ShouldTrace("ctx_simplify_tactic_cache") && entry.traceRef == (ast.ASTRef{}) {
			var err error
			entry.traceRef, err = t.manager.NewRef(entry.value)
			if err != nil {
				return err
			}
		}
		if entry.ref != (ast.ASTRef{}) {
			if err := t.manager.DelRef(&entry.ref); err != nil {
				return err
			}
		}
		var next ast.ASTHandle
		if j > 0 {
			next = cell.results[j-1].value
		}
		if err := t.traceRestore(key, *entry, next); err != nil {
			return err
		}
		if entry.traceRef != (ast.ASTRef{}) {
			if err := t.manager.DelRef(&entry.traceRef); err != nil {
				return err
			}
		}
		if j == 0 && cell.key != (ast.ASTRef{}) {
			if err := t.manager.DelRef(&cell.key); err != nil {
				return err
			}
		}
		t.manager.Budget().Release(entry.bytes)
		entry.bytes = 0
		cell.results = cell.results[:j]
		if j == 0 {
			delete(r.cache, key)
		}
		r.undo[level] = r.undo[level][:i]
	}
	return nil
}
func (t *ContextSimplifyTactic) pop(r *contextSimplifyImplementation, n uint32) error {
	if n == 0 {
		return nil
	}
	level := r.simp.ScopeLevel()
	if n > level {
		return ast.ErrASTDomain
	}
	if err := r.simp.Pop(n); err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		if err := t.restoreCache(r, level-i); err != nil {
			return err
		}
	}
	if ast.ASTSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		if d != nil {
			enabled, err := d.ConditionalAssertionEnabled("ctx_simplify_tactic")
			if err != nil {
				return err
			}
			if enabled {
				return t.checkContextCache(r)
			}
		}
	}
	return nil
}
func (t *ContextSimplifyTactic) popTo(level uint32) error {
	r := t.current
	if level > r.simp.ScopeLevel() {
		return ast.ErrASTDomain
	}
	return t.pop(r, r.simp.ScopeLevel()-level)
}
func (t *ContextSimplifyTactic) simplify(h ast.ASTHandle, out ast.ASTRef) error {
	m, r := t.manager, t.current
	if err := m.SetRef(out, ast.ASTHandle{}); err != nil {
		return err
	}
	if r.depth >= r.maxDepth || r.steps >= r.maxSteps {
		return m.SetRef(out, h)
	}
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return m.SetRef(out, h)
	}
	may, err := r.simp.MaySimplify(h)
	if err != nil {
		return err
	}
	if !may {
		return m.SetRef(out, h)
	}
	if err = t.checkpoint(); err != nil {
		return err
	}
	if err = t.traceProcessing(h); err != nil {
		return err
	}
	if cell := r.cache[h]; cell != nil && len(cell.results) > 0 {
		entry := cell.results[len(cell.results)-1]
		if entry.level == r.simp.ScopeLevel() {
			return m.SetRef(out, entry.value)
		}
	}
	if found, e := r.simp.Simplify(h, out); e != nil || found {
		if e == nil {
			value, e := m.RefGet(out)
			if e != nil {
				return e
			}
			return t.assertContext(!value.IsNull())
		}
		return e
	}
	r.steps++
	r.depth++
	family, err := m.GetFamilyID(h)
	if err != nil {
		return err
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return err
	}
	if family == m.BasicFamilyID() && (op == ast.ASTOpOr || op == ast.ASTOpAnd) {
		err = t.simplifyOrAnd(h, out, op == ast.ASTOpOr)
	} else if family == m.BasicFamilyID() && op == ast.ASTOpIte {
		err = t.simplifyITE(h, out)
	} else {
		err = t.simplifyApp(h, out)
	}
	if err != nil {
		return err
	}
	r.depth--
	value, err := m.RefGet(out)
	if err != nil {
		return err
	}
	if err = t.assertContext(!value.IsNull()); err != nil {
		return err
	}
	return t.traceSimplified(h, value)
}
func (t *ContextSimplifyTactic) rewriteArg(f *contextSimplifyRefs, h ast.ASTHandle) (ast.ASTHandle, error) {
	out, err := f.newRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.simplify(h, out); err != nil {
		return ast.ASTHandle{}, err
	}
	return t.manager.RefGet(out)
}
func (t *ContextSimplifyTactic) makeApp(h ast.ASTHandle, args []ast.ASTHandle, out ast.ASTRef) error {
	d, err := t.manager.GetDecl(h)
	if err != nil {
		return err
	}
	return t.current.mkApp.Make(d, ast.SliceArguments(args), out)
}
func (t *ContextSimplifyTactic) simplifyApp(h ast.ASTHandle, out ast.ASTRef) (err error) {
	n, err := t.manager.GetNumArgs(h)
	if err != nil {
		return err
	}
	if n == 0 {
		return t.manager.SetRef(out, h)
	}
	f := &contextSimplifyRefs{m: t.manager}
	defer t.release(f, &err)
	args := make([]ast.ASTHandle, 0, n)
	modified := false
	for i := uint32(0); i < n; i++ {
		a, e := t.manager.GetArg(h, i)
		if e != nil {
			return e
		}
		b, e := t.rewriteArg(f, a)
		if e != nil {
			return e
		}
		if b.IsNull() {
			if e = t.traceNullArgument(a); e != nil {
				return e
			}
			if e = t.assertContext(false); e != nil {
				return e
			}
		}
		modified = modified || a != b
		args = append(args, b)
	}
	if !modified {
		return t.manager.SetRef(out, h)
	}
	return t.makeApp(h, args, out)
}
func (t *ContextSimplifyTactic) simplifyOrAnd(h ast.ASTHandle, out ast.ASTRef, or bool) (err error) {
	m, r := t.manager, t.current
	f := &contextSimplifyRefs{m: m}
	defer t.release(f, &err)
	old := r.simp.ScopeLevel()
	modified := false
	n, err := m.GetNumArgs(h)
	if err != nil {
		return err
	}
	first := make([]ast.ASTHandle, 0, n)
	for i := uint32(0); i < n; i++ {
		a, e := m.GetArg(h, i)
		if e != nil {
			return e
		}
		b, e := t.rewriteArg(f, a)
		if e != nil {
			return e
		}
		modified = modified || a != b
		if i+1 < n && !m.IsTrue(b) && !m.IsFalse(b) {
			ok, e := r.simp.AssertExpression(b, or)
			if e != nil {
				return e
			}
			if !ok {
				if or {
					b, e = m.MkTrue()
				} else {
					b, e = m.MkFalse()
				}
				if e != nil {
					return e
				}
				if e = m.SetRef(f.refs[len(f.refs)-1], b); e != nil {
					return e
				}
			}
		}
		if or && m.IsFalse(b) || !or && m.IsTrue(b) {
			modified = true
			if e = f.dropLast(); e != nil {
				return e
			}
			continue
		}
		if or && m.IsTrue(b) || !or && m.IsFalse(b) {
			if e = m.SetRef(out, b); e != nil {
				return e
			}
			if e = t.popTo(old); e != nil {
				return e
			}
			return t.cacheResult(h, b)
		}
		first = append(first, b)
	}
	if err = t.popTo(old); err != nil {
		return err
	}
	second := make([]ast.ASTHandle, 0, len(first))
	for i := len(first) - 1; i >= 0; i-- {
		a := first[i]
		b, e := t.rewriteArg(f, a)
		if e != nil {
			return e
		}
		modified = modified || a != b
		if i > 0 && !m.IsTrue(b) && !m.IsFalse(b) {
			ok, e := r.simp.AssertExpression(b, or)
			if e != nil {
				return e
			}
			if !ok {
				if or {
					b, e = m.MkTrue()
				} else {
					b, e = m.MkFalse()
				}
				if e != nil {
					return e
				}
				if e = m.SetRef(f.refs[len(f.refs)-1], b); e != nil {
					return e
				}
			}
		}
		if or && m.IsFalse(b) || !or && m.IsTrue(b) {
			modified = true
			if e = f.dropLast(); e != nil {
				return e
			}
			continue
		}
		if or && m.IsTrue(b) || !or && m.IsFalse(b) {
			if e = m.SetRef(out, b); e != nil {
				return e
			}
			if e = t.popTo(old); e != nil {
				return e
			}
			return t.cacheResult(h, b)
		}
		second = append(second, b)
	}
	if err = t.popTo(old); err != nil {
		return err
	}
	if !modified {
		err = m.SetRef(out, h)
	} else if len(second) == 0 {
		var b ast.ASTHandle
		if or {
			b, err = m.MkFalse()
		} else {
			b, err = m.MkTrue()
		}
		if err == nil {
			err = m.SetRef(out, b)
		}
	} else if len(second) == 1 {
		err = m.SetRef(out, second[0])
	} else {
		for i, j := 0, len(second)-1; i < j; i, j = i+1, j-1 {
			second[i], second[j] = second[j], second[i]
		}
		err = t.makeApp(h, second, out)
	}
	if err != nil {
		return err
	}
	value, err := m.RefGet(out)
	if err != nil {
		return err
	}
	return t.cacheResult(h, value)
}
func (t *ContextSimplifyTactic) simplifyITE(h ast.ASTHandle, out ast.ASTRef) (err error) {
	m, r := t.manager, t.current
	old := r.simp.ScopeLevel()
	f := &contextSimplifyRefs{m: m}
	defer t.release(f, &err)
	c, err := m.GetArg(h, 0)
	if err != nil {
		return err
	}
	a, err := m.GetArg(h, 1)
	if err != nil {
		return err
	}
	b, err := m.GetArg(h, 2)
	if err != nil {
		return err
	}
	nc, err := t.rewriteArg(f, c)
	if err != nil {
		return err
	}
	if m.IsTrue(nc) {
		err = t.simplify(a, out)
	} else if m.IsFalse(nc) {
		err = t.simplify(b, out)
	} else {
		ok, e := r.simp.AssertExpression(nc, false)
		if e != nil {
			return e
		}
		if !ok {
			if e = t.simplify(b, out); e != nil {
				return e
			}
			value, e := m.RefGet(out)
			if e != nil {
				return e
			}
			return t.cacheResult(h, value)
		}
		na, e := t.rewriteArg(f, a)
		if e != nil {
			return e
		}
		if e = t.popTo(old); e != nil {
			return e
		}
		ok, e = r.simp.AssertExpression(nc, true)
		if e != nil {
			return e
		}
		if !ok {
			if e = m.SetRef(out, na); e != nil {
				return e
			}
			return t.cacheResult(h, na)
		}
		nb, e := t.rewriteArg(f, b)
		if e != nil {
			return e
		}
		if e = t.popTo(old); e != nil {
			return e
		}
		if c == nc && a == na && b == nb {
			err = m.SetRef(out, h)
		} else if na == nb {
			err = m.SetRef(out, na)
		} else {
			if e = t.traceITE(nc, na, nb); e != nil {
				return e
			}
			err = t.makeApp(h, []ast.ASTHandle{nc, na, nb}, out)
		}
	}
	if err != nil {
		return err
	}
	value, err := m.RefGet(out)
	if err != nil {
		return err
	}
	return t.cacheResult(h, value)
}
func (t *ContextSimplifyTactic) expressionSize(root ast.ASTHandle) (uint32, error) {
	seen := make(map[ast.ASTHandle]bool)
	todo := []ast.ASTHandle{root}
	for len(todo) > 0 {
		h := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		kind, e := t.manager.GetKind(h)
		if e != nil {
			return 0, e
		}
		if kind == ast.ASTApp {
			n, e := t.manager.GetNumArgs(h)
			if e != nil {
				return 0, e
			}
			for i := uint32(0); i < n; i++ {
				a, e := t.manager.GetArg(h, i)
				if e != nil {
					return 0, e
				}
				todo = append(todo, a)
			}
		}
	}
	return uint32(len(seen)), nil
}
func (t *ContextSimplifyTactic) processProofGoal(g *tactic.Goal) (err error) {
	m, r := t.manager, t.current
	f := &contextSimplifyRefs{m: m}
	defer t.release(f, &err)
	out, err := f.newRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	pr, err := f.newRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	n, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		inconsistent, e := g.Inconsistent()
		if e != nil {
			return e
		}
		if inconsistent {
			break
		}
		h, e := g.Form(i)
		if e != nil {
			return e
		}
		if e = t.traceProcess(h); e != nil {
			return e
		}
		if e = t.assertContext(r.simp.ScopeLevel() == 0); e != nil {
			return e
		}
		r.depth = 0
		if e = t.simplify(h, out); e != nil {
			return e
		}
		value, e := m.RefGet(out)
		if e != nil {
			return e
		}
		if e = t.assertContext(r.simp.ScopeLevel() == 0 && r.depth == 0 && !value.IsNull()); e != nil {
			return e
		}
		if e = t.traceProcessResult(h, value); e != nil {
			return e
		}
		if r.bailOnBlowup {
			before, e := t.expressionSize(h)
			if e != nil {
				return e
			}
			after, e := t.expressionSize(value)
			if e != nil {
				return e
			}
			if before < after {
				value = h
				if e = m.SetRef(out, h); e != nil {
					return e
				}
			}
		}
		rewrite, e := m.MkProofRewrite(h, value)
		if e != nil {
			return e
		}
		if e = m.SetRef(pr, rewrite); e != nil {
			return e
		}
		old, e := g.Proof(i)
		if e != nil {
			return e
		}
		proof, e := m.MkProofModusPonens(old, rewrite)
		if e != nil {
			return e
		}
		if e = m.SetRef(pr, proof); e != nil {
			return e
		}
		dep, e := g.Dependency(i)
		if e != nil {
			return e
		}
		if e = g.Update(i, value, proof, dep); e != nil {
			return e
		}
	}
	return nil
}
func (t *ContextSimplifyTactic) processGoal(g *tactic.Goal) (err error) {
	m, r := t.manager, t.current
	if err = t.assertContext(r.simp.ScopeLevel() == 0); err != nil {
		return err
	}
	old := r.simp.ScopeLevel()
	f := &contextSimplifyRefs{m: m}
	defer t.release(f, &err)
	out, err := f.newRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	pass := func(backwards bool) error {
		n, e := g.Size()
		if e != nil {
			return e
		}
		for j := uint32(0); j < n; j++ {
			bad, e := g.Inconsistent()
			if e != nil {
				return e
			}
			if bad {
				break
			}
			i := j
			if backwards {
				i = n - 1 - j
			}
			r.depth = 0
			h, e := g.Form(i)
			if e != nil {
				return e
			}
			if e = t.simplify(h, out); e != nil {
				return e
			}
			value, e := m.RefGet(out)
			if e != nil {
				return e
			}
			dep, e := g.Dependency(i)
			if e != nil {
				return e
			}
			more := i+1 < n
			if backwards {
				more = i > 0
			}
			if more && !m.IsTrue(value) && !m.IsFalse(value) && dep.IsNull() {
				ok, e := r.simp.AssertExpression(value, false)
				if e != nil {
					return e
				}
				if !ok {
					value, e = m.MkFalse()
					if e != nil {
						return e
					}
					if e = m.SetRef(out, value); e != nil {
						return e
					}
				}
			}
			if e = g.Update(i, value, ast.ASTHandle{}, dep); e != nil {
				return e
			}
		}
		return nil
	}
	if err = pass(false); err != nil {
		return err
	}
	if err = t.popTo(old); err != nil {
		return err
	}
	// Source intentionally adds the updated goal's counts without resetting.
	if err = r.occs.Count(g); err != nil {
		return err
	}
	if err = pass(true); err != nil {
		return err
	}
	return t.popTo(old)
}
