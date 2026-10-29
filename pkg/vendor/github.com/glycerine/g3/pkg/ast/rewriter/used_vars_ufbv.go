package rewriter

// Source: src/ast/used_vars.cpp:21-96; concrete UFBV quantifier-cleanup use.

import "github.com/glycerine/g3/pkg/ast"

type usedVarExprDepth struct {
	expr  ast.ASTHandle
	depth uint32
}

// ufbvUsedVars is the reached used_vars collector for UFBV's
// elim_unused_vars call. found[i] is the sort of free variable i, or null.
// Expression/depth pairs use a native map instead of the source hashtable.
type ufbvUsedVars struct {
	manager *ast.ASTManager
	found   []ast.ASTHandle
}

func newUFBVUsedVars(m *ast.ASTManager) (*ufbvUsedVars, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	return &ufbvUsedVars{manager: m}, nil
}

func (u *ufbvUsedVars) reset() { u.found = u.found[:0] }

// process accumulates free variables from body, patterns or no-patterns. Its
// per-call cache is cleared just as used_vars::process clears m_cache.
func (u *ufbvUsedVars) process(root ast.ASTHandle) error {
	if u == nil || u.manager == nil {
		return ast.ErrASTHandle
	}
	seen := make(map[usedVarExprDepth]struct{})
	todo := []usedVarExprDepth{{expr: root}}
	for len(todo) != 0 {
		i := len(todo) - 1
		item := todo[i]
		todo = todo[:i]
		if _, present := seen[item]; present {
			continue
		}
		seen[item] = struct{}{}
		kind, e := u.manager.GetKind(item.expr)
		if e != nil {
			return e
		}
		switch kind {
		case ast.ASTVar:
			index, e := u.manager.GetVarIndex(item.expr)
			if e != nil {
				return e
			}
			if index < item.depth {
				continue
			}
			index -= item.depth
			if uint64(index) >= uint64(int(^uint(0)>>1)) {
				return ast.ErrASTDomain
			}
			if int(index) >= len(u.found) {
				u.found = append(u.found, make([]ast.ASTHandle, int(index)+1-len(u.found))...)
			}
			sort, e := u.manager.GetSort(item.expr)
			if e != nil {
				return e
			}
			u.found[index] = sort
		case ast.ASTApp:
			n, e := u.manager.GetNumArgs(item.expr)
			if e != nil {
				return e
			}
			for j := n; j > 0; j-- {
				child, e := u.manager.GetArg(item.expr, j-1)
				if e != nil {
					return e
				}
				todo = append(todo, usedVarExprDepth{child, item.depth})
			}
		case ast.ASTQuantifier:
			n, e := u.manager.GetNumDeclarations(item.expr)
			if e != nil {
				return e
			}
			if n > ^uint32(0)-item.depth {
				return ast.ErrASTDomain
			}
			depth := item.depth + n
			pn, e := u.manager.GetNumPatterns(item.expr)
			if e != nil {
				return e
			}
			for j := pn; j > 0; j-- {
				child, e := u.manager.GetPattern(item.expr, j-1)
				if e != nil {
					return e
				}
				todo = append(todo, usedVarExprDepth{child, depth})
			}
			nn, e := u.manager.GetNumNoPatterns(item.expr)
			if e != nil {
				return e
			}
			for j := nn; j > 0; j-- {
				child, e := u.manager.GetNoPattern(item.expr, j-1)
				if e != nil {
					return e
				}
				todo = append(todo, usedVarExprDepth{child, depth})
			}
			body, e := u.manager.GetQuantifierBody(item.expr)
			if e != nil {
				return e
			}
			todo = append(todo, usedVarExprDepth{body, depth})
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (u *ufbvUsedVars) contains(i uint32) bool {
	return uint64(i) < uint64(len(u.found)) && !u.found[i].IsNull()
}

func (u *ufbvUsedVars) usesAll(n uint32) bool {
	if uint64(n) > uint64(len(u.found)) {
		return false
	}
	for i := uint32(0); i < n; i++ {
		if !u.contains(i) {
			return false
		}
	}
	return true
}

func (u *ufbvUsedVars) usesAny(n uint32) bool {
	if uint64(n) > uint64(len(u.found)) {
		n = uint32(len(u.found))
	}
	for i := uint32(0); i < n; i++ {
		if u.contains(i) {
			return true
		}
	}
	return false
}

func (u *ufbvUsedVars) count() uint32 {
	var result uint32
	for _, sort := range u.found {
		if !sort.IsNull() {
			result++
		}
	}
	return result
}
