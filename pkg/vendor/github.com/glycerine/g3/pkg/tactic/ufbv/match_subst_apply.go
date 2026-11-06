package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:905-917;
// src/ast/substitution/substitution.cpp:83-213, specialized for its two
// zero-delta offsets and UFBV's variable bindings at offset 1.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type matchApplyKey struct {
	expr  ast.ASTHandle
	depth uint32
}

type matchApply struct {
	owner *matchSubst
	cache map[matchApplyKey]ast.ASTHandle
}

func (a *matchApply) keep(h ast.ASTHandle) (ast.ASTHandle, error) {
	if e := a.owner.manager.IncRef(h); e != nil {
		return ast.ASTHandle{}, e
	}
	a.owner.scratch = append(a.owner.scratch, h)
	return h, nil
}

func (a *matchApply) shiftTarget(target ast.ASTHandle, depth uint32) (ast.ASTHandle, error) {
	if depth == 0 {
		return target, nil
	}
	m := a.owner.manager
	shifter, e := rewriter.NewVarShifter(m)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	out, e := m.NewRef(ast.ASTHandle{})
	if e != nil {
		_ = shifter.Close()
		return ast.ASTHandle{}, e
	}
	if e = shifter.ShiftBy(target, depth, out); e != nil {
		_ = m.DelRef(&out)
		_ = shifter.Close()
		return ast.ASTHandle{}, e
	}
	h, e := m.RefGet(out)
	if e == nil {
		h, e = a.keep(h)
	}
	if cleanup := m.DelRef(&out); e == nil {
		e = cleanup
	}
	if cleanup := shifter.Close(); e == nil {
		e = cleanup
	}
	return h, e
}

func (a *matchApply) visit(h ast.ASTHandle, depth uint32) (ast.ASTHandle, error) {
	key := matchApplyKey{h, depth}
	if result, found := a.cache[key]; found {
		return result, nil
	}
	m := a.owner.manager
	kind, e := m.GetKind(h)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	result := h
	switch kind {
	case ast.ASTVar:
		index, e := m.GetVarIndex(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if index >= depth {
			if target, found := a.owner.byIndex[index-depth]; found {
				result, e = a.shiftTarget(target, depth)
				if e != nil {
					return ast.ASTHandle{}, e
				}
			}
		}
	case ast.ASTApp:
		count, e := m.GetNumArgs(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args := make([]ast.ASTHandle, count)
		changed := false
		for i := range args {
			old, e := m.GetArg(h, uint32(i))
			if e != nil {
				return ast.ASTHandle{}, e
			}
			args[i], e = a.visit(old, depth)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			changed = changed || args[i] != old
		}
		if changed {
			decl, e := m.GetDecl(h)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = m.MkApp(decl, args)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = a.keep(result)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
	case ast.ASTQuantifier:
		n, e := m.GetNumDeclarations(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if n > math.MaxUint32-depth {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		inner := depth + n
		pn, e := m.GetNumPatterns(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		patterns := make([]ast.ASTHandle, pn)
		changed := false
		for i := range patterns {
			old, e := m.GetPattern(h, uint32(i))
			if e != nil {
				return ast.ASTHandle{}, e
			}
			patterns[i], e = a.visit(old, inner)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			changed = changed || patterns[i] != old
		}
		nn, e := m.GetNumNoPatterns(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		noPatterns := make([]ast.ASTHandle, nn)
		for i := range noPatterns {
			old, e := m.GetNoPattern(h, uint32(i))
			if e != nil {
				return ast.ASTHandle{}, e
			}
			noPatterns[i], e = a.visit(old, inner)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			changed = changed || noPatterns[i] != old
		}
		oldBody, e := m.GetQuantifierBody(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		body, e := a.visit(oldBody, inner)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if changed || body != oldBody {
			result, e = m.UpdateQuantifierAnnotations(h, patterns, noPatterns, body)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = a.keep(result)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	a.cache[key] = result
	return result, nil
}

// rewriteRule is match_subst::operator()(app*,expr*,expr*const*,expr_ref&).
// The returned ASTRef owns the rewritten expression independently of matcher
// reset. The caller must release that reference through the AST manager.
func (s *matchSubst) rewriteRule(lhs, rhs ast.ASTHandle, args []ast.ASTHandle) (ast.ASTRef, bool, error) {
	ok, e := s.matchArgs(lhs, args)
	if e != nil || !ok {
		return ast.ASTRef{}, false, e
	}
	result := rhs
	if !s.allEqual {
		apply := matchApply{owner: s, cache: make(map[matchApplyKey]ast.ASTHandle)}
		result, e = apply.visit(rhs, 0)
		if e != nil {
			return ast.ASTRef{}, false, e
		}
	}
	owned, e := s.manager.NewRef(result)
	return owned, e == nil, e
}
