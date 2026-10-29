package ast

// Source: src/ast/used_symbols.h.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

// UsedSymbols is the concrete identity-renamer instantiation used by retained
// source consumers. It borrows its manager and all traversed AST handles.
type UsedSymbols struct {
	manager *ASTManager
	used    map[usymbol.Symbol]struct{}
}

func NewUsedSymbols(m *ASTManager) (*UsedSymbols, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return nil, e
	}
	m.mu.Unlock()
	return &UsedSymbols{manager: m, used: make(map[usymbol.Symbol]struct{})}, nil
}

func (u *UsedSymbols) Collect(root ASTHandle, ignoreQuantifiers bool) error {
	if u == nil || u.manager == nil || u.used == nil {
		return ErrASTHandle
	}
	clear(u.used)
	visited := make(map[ASTHandle]struct{})
	todo := []ASTHandle{root}
	visited[root] = struct{}{}
	visit := func(h ASTHandle) {
		if _, found := visited[h]; !found {
			visited[h] = struct{}{}
			todo = append(todo, h)
		}
	}
	for len(todo) != 0 {
		last := len(todo) - 1
		n := todo[last]
		todo = todo[:last]
		kind, err := u.manager.GetKind(n)
		if err != nil {
			return err
		}
		switch kind {
		case ASTApp:
			decl, err := u.manager.GetDecl(n)
			if err != nil {
				return err
			}
			name, err := u.manager.GetASTName(decl)
			if err != nil {
				return err
			}
			u.used[name] = struct{}{}
			count, err := u.manager.GetNumArgs(n)
			if err != nil {
				return err
			}
			for i := count; i != 0; i-- {
				child, err := u.manager.GetArg(n, i-1)
				if err != nil {
					return err
				}
				visit(child)
			}
		case ASTQuantifier:
			if ignoreQuantifiers {
				continue
			}
			qid, _, err := u.manager.GetQuantifierIDs(n)
			if err != nil {
				return err
			}
			u.used[qid] = struct{}{}
			count, err := u.manager.GetNumDeclarations(n)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				name, err := u.manager.GetDeclarationName(n, i)
				if err != nil {
					return err
				}
				u.used[name] = struct{}{}
			}
			patterns, err := u.manager.GetNumPatterns(n)
			if err != nil {
				return err
			}
			for i := uint32(0); i < patterns; i++ {
				p, err := u.manager.GetPattern(n, i)
				if err != nil {
					return err
				}
				visit(p)
			}
			noPatterns, err := u.manager.GetNumNoPatterns(n)
			if err != nil {
				return err
			}
			for i := uint32(0); i < noPatterns; i++ {
				p, err := u.manager.GetNoPattern(n, i)
				if err != nil {
					return err
				}
				visit(p)
			}
			body, err := u.manager.GetQuantifierBody(n)
			if err != nil {
				return err
			}
			visit(body)
		case ASTVar:
		default:
			return ErrASTDomain
		}
	}
	return nil
}

func (u *UsedSymbols) Contains(s usymbol.Symbol) bool {
	if u == nil {
		return false
	}
	_, found := u.used[s]
	return found
}

func (u *UsedSymbols) Insert(s usymbol.Symbol) {
	if u != nil && u.used != nil {
		u.used[s] = struct{}{}
	}
}

func (u *UsedSymbols) Erase(s usymbol.Symbol) {
	if u != nil {
		delete(u.used, s)
	}
}
