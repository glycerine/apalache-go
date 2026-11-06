package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:540-650.

import "github.com/glycerine/g3/pkg/ast"

type rewriteVisit struct {
	expr     ast.ASTHandle
	expanded bool
}

// canRewrite follows the source's child-first scan. It tests applications
// against the new LHS after their children and includes quantifier annotations.
func (r *rewriteBackIndex) canRewrite(index *rewriteRuleIndex, root, lhs ast.ASTHandle) (bool, error) {
	stack := []rewriteVisit{{expr: root}}
	seen := make(map[ast.ASTHandle]struct{})
	for len(stack) != 0 {
		i := len(stack) - 1
		visit := stack[i]
		stack = stack[:i]
		if visit.expanded {
			kind, e := r.manager.GetKind(visit.expr)
			if e != nil {
				return false, e
			}
			if kind == ast.ASTApp {
				matched, e := index.matcher.isInstance(lhs, visit.expr)
				if e != nil || matched {
					return matched, e
				}
			}
			continue
		}
		if _, present := seen[visit.expr]; present {
			continue
		}
		seen[visit.expr] = struct{}{}
		stack = append(stack, rewriteVisit{expr: visit.expr, expanded: true})
		kind, e := r.manager.GetKind(visit.expr)
		if e != nil {
			return false, e
		}
		switch kind {
		case ast.ASTVar:
		case ast.ASTApp:
			n, e := r.manager.GetNumArgs(visit.expr)
			if e != nil {
				return false, e
			}
			for j := uint32(0); j < n; j++ {
				child, e := r.manager.GetArg(visit.expr, j)
				if e != nil {
					return false, e
				}
				stack = append(stack, rewriteVisit{expr: child})
			}
		case ast.ASTQuantifier:
			body, e := r.manager.GetQuantifierBody(visit.expr)
			if e != nil {
				return false, e
			}
			stack = append(stack, rewriteVisit{expr: body})
			n, e := r.manager.GetNumNoPatterns(visit.expr)
			if e != nil {
				return false, e
			}
			for j := uint32(0); j < n; j++ {
				child, e := r.manager.GetNoPattern(visit.expr, j)
				if e != nil {
					return false, e
				}
				stack = append(stack, rewriteVisit{expr: child})
			}
			n, e = r.manager.GetNumPatterns(visit.expr)
			if e != nil {
				return false, e
			}
			for j := uint32(0); j < n; j++ {
				child, e := r.manager.GetPattern(visit.expr, j)
				if e != nil {
					return false, e
				}
				stack = append(stack, rewriteVisit{expr: child})
			}
		default:
			return false, ast.ErrASTDomain
		}
	}
	return false, nil
}

// addDemodulator indexes occurrences while the forward index owns q. Unlike
// a processed formula, the backward index does not take a second root.
func (r *rewriteBackIndex) addDemodulator(index *rewriteRuleIndex, q ast.ASTHandle) error {
	if e := r.check(); e != nil {
		return e
	}
	if index == nil || index.manager != r.manager || index.check() != nil {
		return ast.ErrASTDomain
	}
	if _, found := index.rules[q]; !found {
		return ast.ErrASTDomain
	}
	decls, e := r.declarations(q)
	if e != nil {
		return e
	}
	for _, f := range decls {
		if _, e := r.ensureBackSet(f); e != nil {
			return e
		}
	}
	for _, f := range decls {
		r.byDecl[f].values[q] = struct{}{}
	}
	return nil
}

// rescheduleDemodulators snapshots f's occurrences before removing their
// back-index memberships. The local source quantifier_ref becomes one owning
// Go reference, transferred to m_todo after forward removal succeeds.
func (r *rewriteBackIndex) rescheduleDemodulators(index *rewriteRuleIndex, f, lhs ast.ASTHandle) error {
	if e := r.check(); e != nil {
		return e
	}
	if index == nil || index.manager != r.manager || index.check() != nil {
		return ast.ErrASTDomain
	}
	var values map[ast.ASTHandle]struct{}
	if set, found := r.byDecl[f]; found {
		if set == nil {
			if e := assertBackIndexSetPresent(607); e != nil {
				return e
			}
			return ast.ErrASTDomain
		}
		values = set.values
	}
	occurrences := make([]ast.ASTHandle, 0, len(values))
	for q := range values {
		occurrences = append(occurrences, q)
	}
	for _, q := range occurrences {
		kind, e := r.manager.GetKind(q)
		if e != nil {
			return e
		}
		if kind != ast.ASTQuantifier {
			continue
		}
		rule, present := index.rules[q]
		if !present {
			continue
		}
		can, e := r.canRewrite(index, q, lhs)
		if e != nil {
			return e
		}
		if !can {
			continue
		}
		if e := r.traceRescheduling(index, q); e != nil {
			return e
		}
		decls, e := r.declarations(q)
		if e != nil {
			return e
		}
		if e := r.manager.IncRef(q); e != nil {
			return e
		}
		at := len(r.owned)
		r.owned = append(r.owned, q)
		if e := index.remove(rule.function, q); e != nil {
			return e
		}
		for _, d := range decls {
			if e := r.removeBackOccurrence(d, q); e != nil {
				return e
			}
		}
		r.todo = append(r.todo, q)
		r.owned[at] = ast.ASTHandle{} // transfer local reference to todo
	}
	return nil
}
