package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:759-902,919-927.

import "github.com/glycerine/g3/pkg/ast"

type matchPair struct{ pattern, instance ast.ASTHandle }

// matchSubst is the concrete expression-pair cache and variable binding state
// used by UFBV's instance check. The cache and todo borrow AST handles; each
// inserted substitution binding retains its target until reset or close.
type matchSubst struct {
	manager  *ast.ASTManager
	cache    map[matchPair]struct{}
	bindings map[ast.ASTHandle]ast.ASTHandle
	byIndex  map[uint32]ast.ASTHandle
	owned    []ast.ASTHandle
	scratch  []ast.ASTHandle
	todo     []matchPair
	allEqual bool
	closed   bool
}

func (s *matchSubst) bind(pattern, instance ast.ASTHandle) error {
	index, e := s.manager.GetVarIndex(pattern)
	if e != nil {
		return e
	}
	if e := s.manager.IncRef(instance); e != nil {
		return e
	}
	s.owned = append(s.owned, instance)
	s.bindings[pattern] = instance
	s.byIndex[index] = instance
	return nil
}

func (s *matchSubst) releaseScratch() error {
	for len(s.scratch) != 0 {
		h := s.scratch[0]
		consumed, e := s.manager.DecRefConsumed(h)
		if consumed {
			s.scratch[0] = ast.ASTHandle{}
			s.scratch = s.scratch[1:]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	return nil
}

// substitution::reset clears lookup, releases m_refs, then resets its apply
// cache and m_new_exprs. Keep an unconsumed reference for a later retry.
func (s *matchSubst) resetBindings() error {
	clear(s.bindings)
	clear(s.byIndex)
	for len(s.owned) != 0 {
		h := s.owned[0]
		consumed, e := s.manager.DecRefConsumed(h)
		if consumed {
			s.owned[0] = ast.ASTHandle{}
			s.owned = s.owned[1:]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	return s.releaseScratch()
}

func (s *matchSubst) close() error {
	if s == nil || s.closed {
		return nil
	}
	if e := s.resetBindings(); e != nil {
		return e
	}
	clear(s.cache)
	s.todo = nil
	s.closed = true
	return nil
}

func newMatchSubst(m *ast.ASTManager) (*matchSubst, error) {
	if m == nil {
		return nil, ast.ErrASTDomain
	}
	return &matchSubst{manager: m, cache: make(map[matchPair]struct{}), bindings: make(map[ast.ASTHandle]ast.ASTHandle), byIndex: make(map[uint32]ast.ASTHandle)}, nil
}

func (s *matchSubst) sameDecl(a, b ast.ASTHandle) (bool, error) {
	da, e := s.manager.GetDecl(a)
	if e != nil {
		return false, e
	}
	db, e := s.manager.GetDecl(b)
	return da == db, e
}

// bindIdentical traverses an equal, non-ground application. A variable may be
// left unbound or already bound to itself at offset 1; another binding fails.
func (s *matchSubst) bindIdentical(root ast.ASTHandle) (bool, error) {
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]struct{})
	for len(stack) != 0 {
		i := len(stack) - 1
		h := stack[i]
		stack = stack[:i]
		if _, done := seen[h]; done {
			continue
		}
		seen[h] = struct{}{}
		kind, e := s.manager.GetKind(h)
		if e != nil {
			return false, e
		}
		switch kind {
		case ast.ASTVar:
			index, e := s.manager.GetVarIndex(h)
			if e != nil {
				return false, e
			}
			if bound, found := s.byIndex[index]; found && bound != h {
				return false, nil
			}
			if e := s.bind(h, h); e != nil {
				return false, e
			}
		case ast.ASTQuantifier:
			return false, nil
		case ast.ASTApp:
			count, e := s.manager.GetNumArgs(h)
			if e != nil {
				return false, e
			}
			for j := count; j > 0; j-- {
				child, e := s.manager.GetArg(h, j-1)
				if e != nil {
					return false, e
				}
				stack = append(stack, child)
			}
		default:
			return false, ast.ErrASTDomain
		}
	}
	return true, nil
}

// matchArgs follows match_subst::match_args, including its quick equal and
// quick declaration-failure paths. args are the proposed instance arguments.
func (s *matchSubst) matchArgs(lhs ast.ASTHandle, args []ast.ASTHandle) (bool, error) {
	if s == nil || s.manager == nil || s.closed {
		return false, ast.ErrASTDomain
	}
	clear(s.cache)
	s.todo = s.todo[:0]
	count, e := s.manager.GetNumArgs(lhs)
	if e != nil {
		return false, e
	}
	if int(count) != len(args) {
		return false, ast.ErrASTDomain
	}
	s.allEqual = true
	for i, instance := range args {
		pattern, e := s.manager.GetArg(lhs, uint32(i))
		if e != nil {
			return false, e
		}
		if pattern != instance {
			s.allEqual = false
		}
		pk, e := s.manager.GetKind(pattern)
		if e != nil {
			return false, e
		}
		ik, e := s.manager.GetKind(instance)
		if e != nil {
			return false, e
		}
		if pk == ast.ASTApp && ik == ast.ASTApp {
			equal, e := s.sameDecl(pattern, instance)
			if e != nil || !equal {
				return false, e
			}
		}
		s.todo = append(s.todo, matchPair{pattern, instance})
	}
	if s.allEqual {
		return true, nil
	}
	if e := s.resetBindings(); e != nil {
		return false, e
	}
	for len(s.todo) != 0 {
		i := len(s.todo) - 1
		p := s.todo[i]
		pk, e := s.manager.GetKind(p.pattern)
		if e != nil {
			return false, e
		}
		if pk == ast.ASTVar {
			index, e := s.manager.GetVarIndex(p.pattern)
			if e != nil {
				return false, e
			}
			if bound, found := s.byIndex[index]; found && bound != p.instance {
				return false, nil
			}
			if _, found := s.byIndex[index]; !found {
				if e := s.bind(p.pattern, p.instance); e != nil {
					return false, e
				}
			}
			s.todo = s.todo[:i]
			continue
		}
		ik, e := s.manager.GetKind(p.instance)
		if e != nil {
			return false, e
		}
		if ik == ast.ASTVar || pk == ast.ASTQuantifier || ik == ast.ASTQuantifier {
			return false, nil
		}
		if pk != ast.ASTApp || ik != ast.ASTApp {
			if enabled, e := forwardAssertionEnabled(); e != nil {
				return false, e
			} else if enabled {
				if e := forwardAssertionFailure(838, "is_app(p.first) && is_app(p.second)"); e != nil {
					return false, e
				}
			}
			return false, ast.ErrASTDomain
		}
		groundPattern, e := s.manager.IsGround(p.pattern)
		if e != nil {
			return false, e
		}
		groundInstance, e := s.manager.IsGround(p.instance)
		if e != nil {
			return false, e
		}
		if groundPattern && !groundInstance {
			return false, nil
		}
		if p.pattern == p.instance && groundPattern {
			if enabled, e := forwardAssertionEnabled(); e != nil {
				return false, e
			} else if enabled {
				groundAgain, e := s.manager.IsGround(p.instance)
				if e != nil {
					return false, e
				}
				if !groundAgain {
					if e := forwardAssertionFailure(844, "to_app(p.second)->is_ground()"); e != nil {
						return false, e
					}
				}
			}
			s.todo = s.todo[:i]
			continue
		}
		if _, seen := s.cache[p]; seen {
			s.todo = s.todo[:i]
			continue
		}
		if p.pattern == p.instance {
			ok, e := s.bindIdentical(p.pattern)
			if e != nil || !ok {
				return false, e
			}
			s.todo = s.todo[:i]
			s.cache[p] = struct{}{}
			continue
		}
		equal, e := s.sameDecl(p.pattern, p.instance)
		if e != nil || !equal {
			return false, e
		}
		pn, e := s.manager.GetNumArgs(p.pattern)
		if e != nil {
			return false, e
		}
		in, e := s.manager.GetNumArgs(p.instance)
		if e != nil {
			return false, e
		}
		if pn != in {
			return false, nil
		}
		s.todo = s.todo[:i]
		if pn == 0 {
			continue
		}
		s.cache[p] = struct{}{}
		for j := pn; j > 0; j-- {
			left, e := s.manager.GetArg(p.pattern, j-1)
			if e != nil {
				return false, e
			}
			right, e := s.manager.GetArg(p.instance, j-1)
			if e != nil {
				return false, e
			}
			s.todo = append(s.todo, matchPair{left, right})
		}
	}
	return true, nil
}

// isInstance is match_subst::operator()(expr*,expr*), which only checks
// matching and does not construct a rewritten RHS.
func (s *matchSubst) isInstance(pattern, instance ast.ASTHandle) (bool, error) {
	if s == nil || s.manager == nil || s.closed {
		return false, ast.ErrASTDomain
	}
	clear(s.cache)
	s.todo = s.todo[:0]
	pk, e := s.manager.GetKind(pattern)
	if e != nil {
		return false, e
	}
	if pk == ast.ASTVar {
		return true, nil
	}
	ik, e := s.manager.GetKind(instance)
	if e != nil {
		return false, e
	}
	if pk != ast.ASTApp || ik != ast.ASTApp {
		return false, nil
	}
	equal, e := s.sameDecl(pattern, instance)
	if e != nil || !equal {
		return false, e
	}
	pn, e := s.manager.GetNumArgs(pattern)
	if e != nil {
		return false, e
	}
	in, e := s.manager.GetNumArgs(instance)
	if e != nil || pn != in {
		return false, e
	}
	args := make([]ast.ASTHandle, in)
	for i := range args {
		args[i], e = s.manager.GetArg(instance, uint32(i))
		if e != nil {
			return false, e
		}
	}
	return s.matchArgs(pattern, args)
}
