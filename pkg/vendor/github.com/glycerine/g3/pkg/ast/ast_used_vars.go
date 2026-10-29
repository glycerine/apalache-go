package ast

// Source: src/ast/used_vars.cpp:22-113; src/ast/used_vars.h:25-56.

type astUsedVarFrame struct {
	node  astNodeRef
	delta uint32
}

// ASTUsedVars collects the sorts of free de Bruijn variables. Found sorts are
// borrowed from the manager; the collector owns only native traversal state.
type ASTUsedVars struct {
	self    *ASTUsedVars
	manager *ASTManager
	found   []astNodeRef
}

func NewASTUsedVars(m *ASTManager) (*ASTUsedVars, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	m.mu.Unlock()
	u := &ASTUsedVars{manager: m}
	u.self = u
	return u, nil
}

func (u *ASTUsedVars) lock() (*ASTManager, error) {
	if u == nil || u.self != u || u.manager == nil {
		return nil, ErrASTHandle
	}
	if e := u.manager.lock(); e != nil {
		return nil, e
	}
	return u.manager, nil
}

// Collect implements operator(): clear the found-variable vector, then scan.
func (u *ASTUsedVars) Collect(root ASTHandle) error {
	u.found = u.found[:0]
	return u.Process(root)
}

// Process accumulates variables across calls while resetting the source's
// expression/depth cache and work stack for this call.
func (u *ASTUsedVars) Process(root ASTHandle) error {
	m, e := u.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(root)
	if e != nil {
		return e
	}
	cache := make(map[astUsedVarFrame]struct{})
	todo := []astUsedVarFrame{{node: r}}
	for len(todo) != 0 {
		i := len(todo) - 1
		item := todo[i]
		todo = todo[:i]
		n := m.nodeAt(item.node.slot)
		if n.rc > 1 {
			if _, found := cache[item]; found {
				continue
			}
			cache[item] = struct{}{}
		}
		switch n.kind {
		case ASTApp:
			a := m.appAt(item.node.slot)
			for j := a.count; j != 0; j-- {
				child, err := m.readInternal(a.args, j-1)
				if err != nil {
					return err
				}
				todo = append(todo, astUsedVarFrame{node: child, delta: item.delta})
			}
		case ASTVar:
			v := m.varAt(item.node.slot)
			if v.index < item.delta {
				continue
			}
			index := v.index - item.delta
			if uint64(index)+1 > uint64(^uint(0)>>1) {
				return ErrASTDomain
			}
			if int(index) >= len(u.found) {
				u.found = append(u.found, make([]astNodeRef, int(index)+1-len(u.found))...)
			}
			u.found[index] = v.sort
		case ASTQuantifier:
			q := m.quantifierAt(item.node.slot)
			if q.count > ^uint32(0)-item.delta {
				return ErrASTDomain
			}
			delta := item.delta + q.count
			for j := q.patternCount; j != 0; j-- {
				child, err := m.readInternal(q.patterns, j-1)
				if err != nil {
					return err
				}
				todo = append(todo, astUsedVarFrame{node: child, delta: delta})
			}
			for j := q.noPatternCount; j != 0; j-- {
				child, err := m.readInternal(q.noPatterns, j-1)
				if err != nil {
					return err
				}
				todo = append(todo, astUsedVarFrame{node: child, delta: delta})
			}
			todo = append(todo, astUsedVarFrame{node: q.body, delta: delta})
		default:
			return ErrASTDomain
		}
	}
	return nil
}

func (u *ASTUsedVars) Reset() { u.found = u.found[:0] }

func (u *ASTUsedVars) MaxFoundVarIndexPlusOne() uint32 { return uint32(len(u.found)) }

func (u *ASTUsedVars) Get(index uint32) (ASTHandle, error) {
	m, e := u.lock()
	if e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if uint64(index) >= uint64(len(u.found)) {
		return ASTHandle{}, ErrASTDomain
	}
	return m.external(u.found[index]), nil
}

func (u *ASTUsedVars) Contains(index uint32) ASTHandle {
	if u == nil || u.self != u || uint64(index) >= uint64(len(u.found)) {
		return ASTHandle{}
	}
	return u.manager.external(u.found[index])
}

func (u *ASTUsedVars) UsesAllVars(count uint32) bool {
	if u == nil || u.self != u || uint64(count) > uint64(len(u.found)) {
		return false
	}
	for i := uint32(0); i < count; i++ {
		if u.found[i].slot == 0 {
			return false
		}
	}
	return true
}

func (u *ASTUsedVars) UsesAVar(count uint32) bool {
	if u == nil || u.self != u {
		return false
	}
	if uint64(count) > uint64(len(u.found)) {
		count = uint32(len(u.found))
	}
	for i := uint32(0); i < count; i++ {
		if u.found[i].slot != 0 {
			return true
		}
	}
	return false
}

func (u *ASTUsedVars) NumVars() uint32 {
	if u == nil || u.self != u {
		return 0
	}
	var count uint32
	for _, sort := range u.found {
		if sort.slot != 0 {
			count++
		}
	}
	return count
}
