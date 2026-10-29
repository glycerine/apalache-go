package ast

// Source: src/ast/expr_abstract.cpp:22-145; src/util/ref_buffer.h:38-99.

import (
	"math"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ExprAbstractor replaces selected constants with de Bruijn variables. Its
// pinned slice owns each generated expression until Close, including when an
// AST allocation interrupts an abstraction.
type ExprAbstractor struct {
	self    *ExprAbstractor
	manager *ASTManager
	pinned  []ASTRef
	drain   int
	closed  bool
}

func NewExprAbstractor(m *ASTManager) (*ExprAbstractor, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return nil, e
	}
	m.mu.Unlock()
	a := &ExprAbstractor{manager: m}
	a.self = a
	return a, nil
}

func (a *ExprAbstractor) pin(h ASTHandle) error {
	r, e := a.manager.NewRef(h)
	if e != nil {
		return e
	}
	a.pinned = append(a.pinned, r)
	return nil
}

func (a *ExprAbstractor) Close() error {
	if a == nil || a.self != a || a.manager == nil {
		return ErrASTHandle
	}
	if a.closed {
		return nil
	}
	if e := a.cleanupPins(); e != nil {
		return e
	}
	a.closed = true
	return nil
}

func (a *ExprAbstractor) cleanupPins() error {
	for a.drain < len(a.pinned) {
		e := a.manager.DelRef(&a.pinned[a.drain])
		if a.pinned[a.drain] == (ASTRef{}) {
			a.drain++
		}
		if e != nil {
			return e
		}
	}
	a.pinned = nil
	a.drain = 0
	return nil
}

// Abstract writes an owning result reference after replacing bound constants.
// A failed call retains acquired references for an explicit Close retry.
func (a *ExprAbstractor) Abstract(base uint32, bound []ASTHandle, root ASTHandle, out ASTRef) error {
	if a == nil || a.self != a || a.manager == nil || a.closed || len(a.pinned) != 0 || len(bound) > math.MaxUint32 {
		return ErrASTHandle
	}
	result, e := a.abstract(base, bound, root)
	if e != nil {
		return e
	}
	if e = a.manager.SetRef(out, result); e != nil {
		return e
	}
	if e = a.cleanupPins(); e != nil {
		return e
	}
	return a.traceExprAbstract(root, nil, result, false)
}

// MkForall abstracts the supplied constant applications and publishes the
// resulting quantifier through the caller's owning result reference.
func (a *ExprAbstractor) MkForall(bound []ASTHandle, body ASTHandle, out ASTRef) error {
	return a.mkQuantifier(true, bound, body, out)
}

// MkExists is the existential counterpart of MkForall.
func (a *ExprAbstractor) MkExists(bound []ASTHandle, body ASTHandle, out ASTRef) error {
	return a.mkQuantifier(false, bound, body, out)
}

func (a *ExprAbstractor) mkQuantifier(forall bool, bound []ASTHandle, body ASTHandle, out ASTRef) error {
	if e := a.Abstract(0, bound, body, out); e != nil {
		return e
	}
	if len(bound) == 0 {
		return a.traceExprAbstract(body, nil, body, true)
	}
	m := a.manager
	sorts := make([]ASTHandle, 0, len(bound))
	names := make([]usymbol.Symbol, 0, len(bound))
	for _, b := range bound {
		kind, e := m.GetKind(b)
		if e != nil {
			return e
		}
		if kind != ASTApp {
			return ErrASTDomain
		}
		sort, e := m.GetSort(b)
		if e != nil {
			return e
		}
		decl, e := m.GetDecl(b)
		if e != nil {
			return e
		}
		name, e := m.GetASTName(decl)
		if e != nil {
			return e
		}
		sorts = append(sorts, sort)
		names = append(names, name)
	}
	abstracted, e := m.RefGet(out)
	if e != nil {
		return e
	}
	q, e := m.MkQuantifier(ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names, Body: abstracted})
	if e != nil {
		return e
	}
	if e = m.SetRef(out, q); e != nil {
		return e
	}
	return a.traceExprAbstract(body, bound, q, true)
}

func (a *ExprAbstractor) abstract(base uint32, bound []ASTHandle, root ASTHandle) (ASTHandle, error) {
	m := a.manager
	if len(bound) == 0 {
		return root, nil
	}
	if ASTSourceDebug {
		count, e := m.GetRefCount(root)
		if e != nil {
			return ASTHandle{}, e
		}
		if count == 0 {
			return ASTHandle{}, ErrASTDomain
		}
	}
	result := make(map[ASTHandle]ASTHandle)
	stack := []ASTHandle{root}
	for i, b := range bound {
		sort, e := m.GetSort(b)
		if e != nil {
			return ASTHandle{}, e
		}
		v, e := m.MkVar(base+uint32(len(bound)-i-1), sort)
		if e != nil {
			return ASTHandle{}, e
		}
		if e = a.pin(v); e != nil {
			return ASTHandle{}, e
		}
		result[b] = v
	}
	for len(stack) != 0 {
		last := len(stack) - 1
		curr := stack[last]
		if _, ok := result[curr]; ok {
			stack = stack[:last]
			continue
		}
		kind, e := m.GetKind(curr)
		if e != nil {
			return ASTHandle{}, e
		}
		switch kind {
		case ASTVar:
			result[curr] = curr
			stack = stack[:last]
		case ASTApp:
			n, e := m.GetNumArgs(curr)
			if e != nil {
				return ASTHandle{}, e
			}
			args := make([]ASTHandle, 0, n)
			changed, visited := false, true
			for i := uint32(0); i < n; i++ {
				child, e := m.GetArg(curr, i)
				if e != nil {
					return ASTHandle{}, e
				}
				v, ok := result[child]
				if !ok {
					stack = append(stack, child)
					visited = false
				} else {
					changed = changed || v != child
					args = append(args, v)
				}
			}
			if !visited {
				continue
			}
			v := curr
			if changed {
				decl, e := m.GetDecl(curr)
				if e != nil {
					return ASTHandle{}, e
				}
				v, e = m.MkApp(decl, args)
				if e != nil {
					return ASTHandle{}, e
				}
				if e = a.pin(v); e != nil {
					return ASTHandle{}, e
				}
			}
			result[curr] = v
			stack = stack[:last]
		case ASTQuantifier:
			count, e := m.GetNumDeclarations(curr)
			if e != nil {
				return ASTHandle{}, e
			}
			patternsCount, e := m.GetNumPatterns(curr)
			if e != nil {
				return ASTHandle{}, e
			}
			patterns := make([]ASTHandle, 0, patternsCount)
			for i := uint32(0); i < patternsCount; i++ {
				p, e := m.GetPattern(curr, i)
				if e != nil {
					return ASTHandle{}, e
				}
				p, e = a.abstract(base+count, bound, p)
				if e != nil {
					return ASTHandle{}, e
				}
				if e = a.pin(p); e != nil {
					return ASTHandle{}, e
				}
				patterns = append(patterns, p)
			}
			body, e := m.GetQuantifierBody(curr)
			if e != nil {
				return ASTHandle{}, e
			}
			body, e = a.abstract(base+count, bound, body)
			if e != nil {
				return ASTHandle{}, e
			}
			if e = a.pin(body); e != nil {
				return ASTHandle{}, e
			}
			v, e := m.UpdateQuantifierPatterns(curr, patterns, body)
			if e != nil {
				return ASTHandle{}, e
			}
			if e = a.pin(v); e != nil {
				return ASTHandle{}, e
			}
			result[curr] = v
			stack = stack[:last]
		default:
			return ASTHandle{}, ErrASTDomain
		}
	}
	v, found := result[root]
	if !found {
		return ASTHandle{}, ErrASTDomain
	}
	return v, nil
}
