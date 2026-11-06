package ast

// Source: src/ast/func_decl_dependencies.cpp;
// src/ast/func_decl_dependencies.h.

// FuncDeclSet is the concrete uninterpreted-function declaration set used by
// retained macro and model consumers. Handles are borrowed until Insert
// successfully gives a set to FuncDeclDependencies.
type FuncDeclSet map[ASTHandle]struct{}

// CollectFuncDecls adds uninterpreted declarations with positive arity that
// occur in root. When nonGroundOnly is true, declarations at ground
// applications are skipped. Traversal still descends through those apps.
func (m *ASTManager) CollectFuncDecls(root ASTHandle, out FuncDeclSet, nonGroundOnly bool) error {
	if out == nil {
		return ErrASTHandle
	}
	visited := make(map[ASTHandle]struct{})
	stack := []ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		n := stack[last]
		stack = stack[:last]
		if _, found := visited[n]; found {
			continue
		}
		visited[n] = struct{}{}
		kind, err := m.GetKind(n)
		if err != nil {
			return err
		}
		switch kind {
		case ASTApp:
			count, err := m.GetNumArgs(n)
			if err != nil {
				return err
			}
			if count != 0 {
				ground, err := m.IsGround(n)
				if err != nil {
					return err
				}
				decl, err := m.GetDecl(n)
				if err != nil {
					return err
				}
				family, err := m.GetFamilyID(decl)
				if err != nil {
					return err
				}
				if family == NullFamilyID && (!nonGroundOnly || !ground) {
					out[decl] = struct{}{}
				}
			}
			for i := count; i != 0; i-- {
				child, err := m.GetArg(n, i-1)
				if err != nil {
					return err
				}
				stack = append(stack, child)
			}
		case ASTQuantifier:
			body, err := m.GetQuantifierBody(n)
			if err != nil {
				return err
			}
			stack = append(stack, body)
			patterns, err := m.GetNumPatterns(n)
			if err != nil {
				return err
			}
			for i := patterns; i != 0; i-- {
				p, err := m.GetPattern(n, i-1)
				if err != nil {
					return err
				}
				stack = append(stack, p)
			}
			noPatterns, err := m.GetNumNoPatterns(n)
			if err != nil {
				return err
			}
			for i := noPatterns; i != 0; i-- {
				p, err := m.GetNoPattern(n, i-1)
				if err != nil {
					return err
				}
				stack = append(stack, p)
			}
		case ASTVar:
		default:
			return ErrASTDomain
		}
	}
	return nil
}

type funcDeclDependencyEntry struct {
	key     ASTRef
	deps    FuncDeclSet
	depRefs []ASTRef
	nextRef int
}

// FuncDeclDependencies owns pins for every successfully inserted declaration
// and its dependency set. Native maps replace the source object maps and sets.
type FuncDeclDependencies struct {
	self    *FuncDeclDependencies
	manager *ASTManager
	deps    map[ASTHandle]*funcDeclDependencyEntry
	order   []ASTHandle
	closed  bool
}

func NewFuncDeclDependencies(m *ASTManager) (*FuncDeclDependencies, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return nil, e
	}
	m.mu.Unlock()
	d := &FuncDeclDependencies{manager: m, deps: make(map[ASTHandle]*funcDeclDependencyEntry)}
	d.self = d
	return d, nil
}

func (d *FuncDeclDependencies) open() error {
	if d == nil || d.self != d || d.manager == nil || d.closed {
		return ErrASTHandle
	}
	_, err := d.manager.ProofsEnabled()
	return err
}

func (d *FuncDeclDependencies) Collect(root ASTHandle, out FuncDeclSet) error {
	if err := d.open(); err != nil {
		return err
	}
	return d.manager.CollectFuncDecls(root, out, false)
}

func (d *FuncDeclDependencies) CollectNonGround(root ASTHandle, out FuncDeclSet) error {
	if err := d.open(); err != nil {
		return err
	}
	return d.manager.CollectFuncDecls(root, out, true)
}

func (d *FuncDeclDependencies) createsCycle(root ASTHandle, incoming FuncDeclSet) bool {
	const (
		open uint8 = iota
		active
		closed
	)
	colors := make(map[ASTHandle]uint8)
	var visit func(ASTHandle) bool
	visit = func(n ASTHandle) bool {
		switch colors[n] {
		case active:
			return true
		case closed:
			return false
		}
		colors[n] = active
		children := FuncDeclSet(nil)
		if n == root {
			children = incoming
		} else if entry := d.deps[n]; entry != nil {
			children = entry.deps
		}
		for child := range children {
			if visit(child) {
				return true
			}
		}
		colors[n] = closed
		return false
	}
	return visit(root)
}

func (d *FuncDeclDependencies) validateDecl(h ASTHandle) error {
	kind, err := d.manager.GetKind(h)
	if err != nil {
		return err
	}
	if kind != ASTFuncDecl {
		return ErrASTDomain
	}
	return nil
}

// Insert returns false for a duplicate key or a dependency cycle.
func (d *FuncDeclDependencies) Insert(decl ASTHandle, dependencies FuncDeclSet) (bool, error) {
	if err := d.open(); err != nil {
		return false, err
	}
	if dependencies == nil {
		return false, ErrASTHandle
	}
	if _, found := d.deps[decl]; found {
		return false, nil
	}
	if err := d.validateDecl(decl); err != nil {
		return false, err
	}
	for dependency := range dependencies {
		if err := d.validateDecl(dependency); err != nil {
			return false, err
		}
	}
	if d.createsCycle(decl, dependencies) {
		return false, nil
	}
	entry := &funcDeclDependencyEntry{deps: make(FuncDeclSet, len(dependencies))}
	key, err := d.manager.NewRef(decl)
	if err != nil {
		return false, err
	}
	entry.key = key
	for dependency := range dependencies {
		ref, e := d.manager.NewRef(dependency)
		if e != nil {
			for i := len(entry.depRefs) - 1; i >= 0; i-- {
				_ = d.manager.DelRef(&entry.depRefs[i])
			}
			_ = d.manager.DelRef(&entry.key)
			return false, e
		}
		entry.deps[dependency] = struct{}{}
		entry.depRefs = append(entry.depRefs, ref)
	}
	d.deps[decl] = entry
	d.order = append(d.order, decl)
	return true, nil
}

func (d *FuncDeclDependencies) Contains(decl ASTHandle) bool {
	if d == nil || d.self != d || d.closed {
		return false
	}
	_, found := d.deps[decl]
	return found
}

func (d *FuncDeclDependencies) GetDependencies(decl ASTHandle) FuncDeclSet {
	if d == nil || d.self != d || d.closed {
		return nil
	}
	entry := d.deps[decl]
	if entry == nil {
		return nil
	}
	result := make(FuncDeclSet, len(entry.deps))
	for h := range entry.deps {
		result[h] = struct{}{}
	}
	return result
}

func (d *FuncDeclDependencies) releaseEntry(entry *funcDeclDependencyEntry) error {
	if entry.key != (ASTRef{}) {
		if err := d.manager.DelRef(&entry.key); err != nil {
			return err
		}
	}
	for entry.nextRef < len(entry.depRefs) {
		if err := d.manager.DelRef(&entry.depRefs[entry.nextRef]); err != nil {
			return err
		}
		entry.nextRef++
	}
	return nil
}

func (d *FuncDeclDependencies) Erase(decl ASTHandle) error {
	if err := d.open(); err != nil {
		return err
	}
	entry := d.deps[decl]
	if entry == nil {
		return nil
	}
	if err := d.releaseEntry(entry); err != nil {
		return err
	}
	delete(d.deps, decl)
	for i, h := range d.order {
		if h == decl {
			copy(d.order[i:], d.order[i+1:])
			d.order = d.order[:len(d.order)-1]
			break
		}
	}
	return nil
}

func (d *FuncDeclDependencies) Reset() error {
	if err := d.open(); err != nil {
		return err
	}
	for len(d.order) != 0 {
		if err := d.Erase(d.order[0]); err != nil {
			return err
		}
	}
	return nil
}

func (d *FuncDeclDependencies) Close() error {
	if d == nil || d.self != d {
		return ErrASTHandle
	}
	if d.closed {
		return nil
	}
	if err := d.Reset(); err != nil {
		return err
	}
	d.closed = true
	return nil
}
