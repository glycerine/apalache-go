package substitution

// Source: src/ast/substitution/substitution_tree.cpp:22-571;
// src/ast/substitution/substitution_tree.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type treeSubst struct {
	register ast.ASTHandle
	value    ast.ASTHandle
	regRef   ast.ASTRef
	valueRef ast.ASTRef
}

type treeNode struct {
	leaf    bool
	subst   []treeSubst
	next    *treeNode
	child   *treeNode
	expr    ast.ASTHandle
	exprRef ast.ASTRef
}

// SubstitutionTree is the source register-linearized discrimination tree.
type SubstitutionTree struct {
	self      *SubstitutionTree
	manager   *ast.ASTManager
	roots     map[ast.ASTHandle]*treeNode
	variables map[ast.ASTHandle][]ast.ASTHandle
	varRefs   map[ast.ASTHandle]ast.ASTRef
	registers []ast.ASTHandle
	todo      []uint32
	used      map[uint32]struct{}
	nextReg   uint32
	maxReg    uint32
	size      uint32
	closed    bool
}

func NewSubstitutionTree(m *ast.ASTManager) (*SubstitutionTree, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	t := &SubstitutionTree{manager: m, roots: make(map[ast.ASTHandle]*treeNode), variables: make(map[ast.ASTHandle][]ast.ASTHandle), varRefs: make(map[ast.ASTHandle]ast.ASTRef), used: make(map[uint32]struct{})}
	t.self = t
	return t, nil
}
func (t *SubstitutionTree) open() error {
	if t == nil || t.self != t || t.manager == nil || t.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (t *SubstitutionTree) nextRegister() (uint32, error) {
	for {
		r := t.nextReg
		if r > t.maxReg {
			t.maxReg = r
		}
		if r == ^uint32(0) {
			return 0, ast.ErrASTDomain
		}
		t.nextReg++
		if _, used := t.used[r]; !used {
			return r, nil
		}
	}
}
func (t *SubstitutionTree) reg(i uint32) ast.ASTHandle {
	if uint64(i) >= uint64(len(t.registers)) {
		return ast.ASTHandle{}
	}
	return t.registers[i]
}
func (t *SubstitutionTree) setReg(i uint32, h ast.ASTHandle) error {
	if uint64(i) > uint64(^uint(0)>>1) {
		return ast.ErrASTDomain
	}
	if int(i) >= len(t.registers) {
		t.registers = append(t.registers, make([]ast.ASTHandle, int(i)+1-len(t.registers))...)
	}
	t.registers[i] = h
	return nil
}
func (t *SubstitutionTree) eraseTodoRegister(r uint32) error {
	if t.reg(r).IsNull() {
		return ast.ErrASTDomain
	}
	t.registers[r] = ast.ASTHandle{}
	for i, v := range t.todo {
		if v == r {
			t.todo = append(t.todo[:i], t.todo[i+1:]...)
			return nil
		}
	}
	return ast.ErrASTDomain
}
func (t *SubstitutionTree) ownSub(register, value ast.ASTHandle) (treeSubst, error) {
	rr, e := t.manager.NewRef(register)
	if e != nil {
		return treeSubst{}, e
	}
	vr, e := t.manager.NewRef(value)
	if e != nil {
		return treeSubst{}, errors.Join(e, t.manager.DelRef(&rr))
	}
	return treeSubst{register: register, value: value, regRef: rr, valueRef: vr}, nil
}
func (t *SubstitutionTree) releaseSub(s *treeSubst) error {
	return errors.Join(t.manager.DelRef(&s.valueRef), t.manager.DelRef(&s.regRef))
}

func (t *SubstitutionTree) linearize() (result []treeSubst, err error) {
	defer func() {
		if err != nil {
			for i := range result {
				err = errors.Join(err, t.releaseSub(&result[i]))
			}
		}
	}()
	for i := 0; i < len(t.todo); i++ {
		ri := t.todo[i]
		n := t.reg(ri)
		if n.IsNull() {
			return result, ast.ErrASTDomain
		}
		sort, e := t.manager.GetSort(n)
		if e != nil {
			return result, e
		}
		rvar, e := t.manager.MkVar(ri, sort)
		if e != nil {
			return result, e
		}
		kind, e := t.manager.GetKind(n)
		if e != nil {
			return result, e
		}
		out := n
		if kind != ast.ASTVar {
			if kind != ast.ASTApp {
				return result, ast.ErrASTDomain
			}
			count, e := t.manager.GetNumArgs(n)
			if e != nil {
				return result, e
			}
			if count != 0 {
				args := make([]ast.ASTHandle, count)
				for j := range args {
					oreg, e := t.nextRegister()
					if e != nil {
						return result, e
					}
					child, e := t.manager.GetArg(n, uint32(j))
					if e != nil {
						return result, e
					}
					if e = t.setReg(oreg, child); e != nil {
						return result, e
					}
					t.todo = append(t.todo, oreg)
					cs, e := t.manager.GetSort(child)
					if e != nil {
						return result, e
					}
					args[j], e = t.manager.MkVar(oreg, cs)
					if e != nil {
						return result, e
					}
				}
				decl, e := t.manager.GetDecl(n)
				if e != nil {
					return result, e
				}
				out, e = t.manager.MkApp(decl, args)
				if e != nil {
					return result, e
				}
			}
		}
		s, e := t.ownSub(rvar, out)
		if e != nil {
			return result, e
		}
		result = append(result, s)
	}
	return result, nil
}

func (t *SubstitutionTree) processArgs(in, out ast.ASTHandle) error {
	n, e := t.manager.GetNumArgs(out)
	if e != nil {
		return e
	}
	m, e := t.manager.GetNumArgs(in)
	if e != nil {
		return e
	}
	if n != m {
		return ast.ErrASTDomain
	}
	for i := uint32(0); i < n; i++ {
		a, e := t.manager.GetArg(in, i)
		if e != nil {
			return e
		}
		r, e := t.manager.GetArg(out, i)
		if e != nil {
			return e
		}
		idx, e := t.manager.GetVarIndex(r)
		if e != nil {
			return e
		}
		if e = t.setReg(idx, a); e != nil {
			return e
		}
		t.todo = append(t.todo, idx)
	}
	return nil
}
func (t *SubstitutionTree) resetRegisters(old int) error {
	if old < 0 || old > len(t.todo) {
		return ast.ErrASTDomain
	}
	for _, r := range t.todo[old:] {
		if int(r) >= len(t.registers) {
			return ast.ErrASTDomain
		}
		t.registers[r] = ast.ASTHandle{}
	}
	t.todo = t.todo[:old]
	return nil
}
func (t *SubstitutionTree) compatibility(sv []treeSubst) (uint32, error) {
	old := len(t.todo)
	var measure uint32
	for _, s := range sv {
		ri, e := t.manager.GetVarIndex(s.register)
		if e != nil {
			return 0, e
		}
		in := t.reg(ri)
		kind, e := t.manager.GetKind(s.value)
		if e != nil {
			return 0, e
		}
		if kind == ast.ASTVar {
			if s.value == in {
				measure++
			}
		} else {
			ik := ast.ASTKind(255)
			if !in.IsNull() {
				ik, e = t.manager.GetKind(in)
				if e != nil {
					return 0, e
				}
			}
			if ik == ast.ASTApp {
				od, e := t.manager.GetDecl(s.value)
				if e != nil {
					return 0, e
				}
				id, e := t.manager.GetDecl(in)
				if e != nil {
					return 0, e
				}
				if od == id {
					measure += 2
					if e = t.processArgs(in, s.value); e != nil {
						return 0, e
					}
				}
			}
		}
	}
	return measure, t.resetRegisters(old)
}
func (t *SubstitutionTree) bestChild(r *treeNode) (*treeNode, error) {
	var best *treeNode
	var max uint32
	for c := r.child; c != nil; c = c.next {
		v, e := t.compatibility(c.subst)
		if e != nil {
			return nil, e
		}
		if v > max {
			best, max = c, v
		}
	}
	return best, nil
}
func (t *SubstitutionTree) resetCompiler() error {
	if err := t.resetRegisters(0); err != nil {
		return err
	}
	clear(t.used)
	t.nextReg = 1
	return nil
}
func (t *SubstitutionTree) makeNode(expr ast.ASTHandle) (*treeNode, error) {
	sv, e := t.linearize()
	if e != nil {
		return nil, e
	}
	ref, e := t.manager.NewRef(expr)
	if e != nil {
		for i := range sv {
			e = errors.Join(e, t.releaseSub(&sv[i]))
		}
		return nil, e
	}
	return &treeNode{leaf: true, subst: sv, expr: expr, exprRef: ref}, nil
}
func (t *SubstitutionTree) markUsed(sv []treeSubst) error {
	for _, s := range sv {
		i, e := t.manager.GetVarIndex(s.register)
		if e != nil {
			return e
		}
		t.used[i] = struct{}{}
		kind, e := t.manager.GetKind(s.value)
		if e != nil {
			return e
		}
		if kind == ast.ASTApp {
			n, e := t.manager.GetNumArgs(s.value)
			if e != nil {
				return e
			}
			for j := uint32(0); j < n; j++ {
				a, e := t.manager.GetArg(s.value, j)
				if e != nil {
					return e
				}
				i, e := t.manager.GetVarIndex(a)
				if e != nil {
					return e
				}
				t.used[i] = struct{}{}
			}
		}
	}
	return nil
}

func (t *SubstitutionTree) Insert(expr ast.ASTHandle) error {
	if e := t.open(); e != nil {
		return e
	}
	kind, e := t.manager.GetKind(expr)
	if e != nil {
		return e
	}
	if kind == ast.ASTVar {
		return t.insertVariable(expr)
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	return t.insertApp(expr)
}
func (t *SubstitutionTree) insertVariable(v ast.ASTHandle) error {
	sort, e := t.manager.GetSort(v)
	if e != nil {
		return e
	}
	list := t.variables[sort]
	for _, x := range list {
		if x == v {
			return nil
		}
	}
	ref, e := t.manager.NewRef(v)
	if e != nil {
		return e
	}
	t.variables[sort] = append(list, v)
	t.varRefs[v] = ref
	return nil
}
func (t *SubstitutionTree) insertApp(expr ast.ASTHandle) error {
	if e := t.resetCompiler(); e != nil {
		return e
	}
	if e := t.setReg(0, expr); e != nil {
		return e
	}
	t.todo = append(t.todo, 0)
	decl, e := t.manager.GetDecl(expr)
	if e != nil {
		return e
	}
	r := t.roots[decl]
	if r == nil {
		r, e = t.makeNode(expr)
		if e != nil {
			return e
		}
		t.roots[decl] = r
		if e = t.resetRegisters(0); e != nil {
			return e
		}
		t.size++
		return nil
	}
	for {
		compatible := make([]treeSubst, 0, len(r.subst))
		incompatible := make([]treeSubst, 0, len(r.subst))
		for _, s := range r.subst {
			ri, e := t.manager.GetVarIndex(s.register)
			if e != nil {
				return e
			}
			in := t.reg(ri)
			kind, e := t.manager.GetKind(s.value)
			if e != nil {
				return e
			}
			ok := false
			if kind == ast.ASTVar {
				ok = s.value == in
			} else if !in.IsNull() {
				ik, e := t.manager.GetKind(in)
				if e != nil {
					return e
				}
				if ik == ast.ASTApp {
					od, e := t.manager.GetDecl(s.value)
					if e != nil {
						return e
					}
					id, e := t.manager.GetDecl(in)
					if e != nil {
						return e
					}
					ok = od == id
					if ok {
						if e = t.processArgs(in, s.value); e != nil {
							return e
						}
					}
				}
			}
			if ok {
				if e = t.eraseTodoRegister(ri); e != nil {
					return e
				}
				compatible = append(compatible, s)
			} else {
				incompatible = append(incompatible, s)
			}
		}
		if len(incompatible) == 0 {
			if len(t.todo) == 0 {
				if !r.leaf || r.expr != expr {
					return ast.ErrASTDomain
				}
				return t.resetRegisters(0)
			}
			if e = t.markUsed(r.subst); e != nil {
				return e
			}
			next, e := t.bestChild(r)
			if e != nil {
				return e
			}
			if next == nil {
				n, e := t.makeNode(expr)
				if e != nil {
					return e
				}
				n.next = r.child
				r.child = n
				if e = t.resetRegisters(0); e != nil {
					return e
				}
				t.size++
				return nil
			}
			r = next
			continue
		}
		if len(compatible) == 0 {
			return ast.ErrASTDomain
		}
		if e = t.markUsed(compatible); e != nil {
			return e
		}
		r.subst = compatible
		n, e := t.makeNode(expr)
		if e != nil {
			return e
		}
		incomp := &treeNode{leaf: r.leaf, subst: incompatible}
		if r.leaf {
			incomp.expr, incomp.exprRef = r.expr, r.exprRef
			r.expr, r.exprRef = ast.ASTHandle{}, ast.ASTRef{}
			r.leaf = false
		} else {
			incomp.child = r.child
		}
		incomp.next = n
		r.child = incomp
		if e = t.resetRegisters(0); e != nil {
			return e
		}
		t.size++
		return nil
	}
}

func (t *SubstitutionTree) fullyCompatible(sv []treeSubst) (bool, error) {
	old := len(t.todo)
	for _, s := range sv {
		ri, e := t.manager.GetVarIndex(s.register)
		if e != nil {
			return false, e
		}
		in := t.reg(ri)
		kind, e := t.manager.GetKind(s.value)
		if e != nil {
			return false, e
		}
		if kind == ast.ASTVar {
			if s.value != in {
				_ = t.resetRegisters(old)
				return false, nil
			}
		} else {
			if in.IsNull() {
				_ = t.resetRegisters(old)
				return false, nil
			}
			ik, e := t.manager.GetKind(in)
			if e != nil {
				return false, e
			}
			if ik != ast.ASTApp {
				_ = t.resetRegisters(old)
				return false, nil
			}
			a, e := t.manager.GetDecl(in)
			if e != nil {
				return false, e
			}
			b, e := t.manager.GetDecl(s.value)
			if e != nil {
				return false, e
			}
			if a != b {
				_ = t.resetRegisters(old)
				return false, nil
			}
			if e = t.processArgs(in, s.value); e != nil {
				return false, e
			}
		}
	}
	return true, t.resetRegisters(old)
}
func (t *SubstitutionTree) findCompatible(r *treeNode) (prev, child *treeNode, found bool, err error) {
	for child = r.child; child != nil; child = child.next {
		ok, e := t.fullyCompatible(child.subst)
		if e != nil {
			return nil, nil, false, e
		}
		if ok {
			return prev, child, true, nil
		}
		prev = child
	}
	return prev, nil, false, nil
}
func (t *SubstitutionTree) releaseNode(root *treeNode) error {
	var result error
	todo := []*treeNode{root}
	for len(todo) != 0 {
		i := len(todo) - 1
		n := todo[i]
		todo = todo[:i]
		for j := range n.subst {
			result = errors.Join(result, t.releaseSub(&n.subst[j]))
		}
		if n.leaf {
			result = errors.Join(result, t.manager.DelRef(&n.exprRef))
		} else {
			for c := n.child; c != nil; c = c.next {
				todo = append(todo, c)
			}
		}
	}
	return result
}

func (t *SubstitutionTree) Erase(expr ast.ASTHandle) error {
	if e := t.open(); e != nil {
		return e
	}
	kind, e := t.manager.GetKind(expr)
	if e != nil {
		return e
	}
	if kind == ast.ASTVar {
		return t.eraseVariable(expr)
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	return t.eraseApp(expr)
}
func (t *SubstitutionTree) eraseVariable(v ast.ASTHandle) error {
	sort, e := t.manager.GetSort(v)
	if e != nil {
		return e
	}
	list := t.variables[sort]
	for i, x := range list {
		if x == v {
			ref := t.varRefs[v]
			if e = t.manager.DelRef(&ref); e != nil {
				return e
			}
			delete(t.varRefs, v)
			t.variables[sort] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return nil
}
func (t *SubstitutionTree) eraseApp(expr ast.ASTHandle) error {
	decl, e := t.manager.GetDecl(expr)
	if e != nil {
		return e
	}
	r := t.roots[decl]
	if r == nil {
		return nil
	}
	if e = t.resetCompiler(); e != nil {
		return e
	}
	if e = t.setReg(0, expr); e != nil {
		return e
	}
	t.todo = append(t.todo, 0)
	var parent, prev *treeNode
	for {
		for _, s := range r.subst {
			ri, e := t.manager.GetVarIndex(s.register)
			if e != nil {
				return e
			}
			in := t.reg(ri)
			kind, e := t.manager.GetKind(s.value)
			if e != nil {
				return e
			}
			if kind == ast.ASTVar {
				if s.value != in {
					_ = t.resetRegisters(0)
					return nil
				}
			} else {
				if in.IsNull() {
					_ = t.resetRegisters(0)
					return nil
				}
				ik, e := t.manager.GetKind(in)
				if e != nil {
					return e
				}
				if ik != ast.ASTApp {
					_ = t.resetRegisters(0)
					return nil
				}
				a, e := t.manager.GetDecl(in)
				if e != nil {
					return e
				}
				b, e := t.manager.GetDecl(s.value)
				if e != nil {
					return e
				}
				if a != b {
					_ = t.resetRegisters(0)
					return nil
				}
				if e = t.processArgs(in, s.value); e != nil {
					return e
				}
			}
			if e = t.eraseTodoRegister(ri); e != nil {
				return e
			}
		}
		if len(t.todo) == 0 {
			if r.expr != expr {
				return ast.ErrASTDomain
			}
			if e = t.resetRegisters(0); e != nil {
				return e
			}
			if parent == nil {
				if e = t.releaseNode(r); e != nil {
					return e
				}
				delete(t.roots, decl)
			} else if parent.child != nil && parent.child.next != nil && parent.child.next.next != nil {
				if prev == nil {
					parent.child = r.next
				} else {
					prev.next = r.next
				}
				r.next = nil
				if e = t.releaseNode(r); e != nil {
					return e
				}
			} else {
				first := parent.child
				second := first.next
				other := first
				if other == r {
					other = second
				}
				parent.subst = append(parent.subst, other.subst...)
				other.subst = nil
				parent.leaf = other.leaf
				if other.leaf {
					parent.expr, parent.exprRef = other.expr, other.exprRef
					other.expr, other.exprRef = ast.ASTHandle{}, ast.ASTRef{}
					parent.child = nil
				} else {
					parent.child = other.child
					other.child = nil
				}
				other.next = nil
				if first == r {
					r.next = nil
				} else {
					first.next = nil
				}
				if e = t.releaseNode(r); e != nil {
					return e
				}
			}
			t.size--
			return nil
		}
		parent = r
		var found bool
		prev, r, found, e = t.findCompatible(r)
		if e != nil {
			return e
		}
		if !found {
			_ = t.resetRegisters(0)
			return nil
		}
	}
}

func (t *SubstitutionTree) Empty() (bool, error) {
	if e := t.open(); e != nil {
		return false, e
	}
	return t.size == 0, nil
}
func (t *SubstitutionTree) ApproxRegisters() (uint32, error) {
	if e := t.open(); e != nil {
		return 0, e
	}
	return t.maxReg + 1, nil
}
func (t *SubstitutionTree) Reset() error {
	if e := t.open(); e != nil {
		return e
	}
	var result error
	for _, r := range t.roots {
		result = errors.Join(result, t.releaseNode(r))
	}
	clear(t.roots)
	for h, ref := range t.varRefs {
		r := ref
		result = errors.Join(result, t.manager.DelRef(&r))
		delete(t.varRefs, h)
	}
	clear(t.variables)
	t.size = 0
	return result
}
func (t *SubstitutionTree) Close() error {
	if t == nil || t.self != t || t.manager == nil {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if e := t.Reset(); e != nil {
		return e
	}
	t.closed = true
	return nil
}
