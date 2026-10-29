package rewriter

// Source: src/ast/rewriter/var_subst.cpp:39-158. This is the concrete
// UFBV call with an empty params_ref, so ignore_patterns_on_ground_qbody=true.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// ElimUnusedVarsUFBV owns its result independently of q. The caller must
// release the returned ASTRef through the manager.
func ElimUnusedVarsUFBV(m *ast.ASTManager, q ast.ASTHandle) (_ ast.ASTRef, result error) {
	return elimUnusedVars(m, q, true)
}

// ElimUnusedVarsWithParams retains the source params_ref snapshot for the
// lexical eliminator lifetime and reads ignore_patterns_on_ground_qbody with
// its true default before performing the rewrite.
func ElimUnusedVarsWithParams(m *ast.ASTManager, q ast.ASTHandle, c *uparams.ParamsContext, p *uparams.ParamsRef) (_ ast.ASTRef, result error) {
	if c == nil || p == nil {
		return ast.ASTRef{}, uparams.ErrParamsOwner
	}
	owned := c.EmptyValue()
	if e := c.Copy(&owned, p); e != nil {
		return ast.ASTRef{}, e
	}
	defer func() { result = errors.Join(result, c.Del(&owned)) }()
	key := "ignore_patterns_on_ground_qbody"
	ignore, e := c.GetBoolName(&owned, &key, true)
	if e != nil {
		return ast.ASTRef{}, e
	}
	return elimUnusedVars(m, q, ignore)
}

func elimUnusedVars(m *ast.ASTManager, q ast.ASTHandle, ignorePatternsOnGroundBody bool) (_ ast.ASTRef, result error) {
	if m == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	if e := assertUnusedVarsWellSorted(m, q, 49); e != nil {
		return ast.ASTRef{}, e
	}
	body, e := m.GetQuantifierBody(q)
	if e != nil {
		return ast.ASTRef{}, e
	}
	ground, e := m.IsGround(body)
	if e != nil {
		return ast.ASTRef{}, e
	}
	if ignorePatternsOnGroundBody && ground {
		return m.NewRef(body)
	}
	mayHaveUnused, e := m.QuantifierMayHaveUnusedVars(q)
	if e != nil {
		return ast.ASTRef{}, e
	}
	if !mayHaveUnused {
		return m.NewRef(q)
	}
	used, e := newUFBVUsedVars(m)
	if e != nil {
		return ast.ASTRef{}, e
	}
	if e := used.process(body); e != nil {
		return ast.ASTRef{}, e
	}
	patternsN, e := m.GetNumPatterns(q)
	if e != nil {
		return ast.ASTRef{}, e
	}
	for i := uint32(0); i < patternsN; i++ {
		pattern, e := m.GetPattern(q, i)
		if e != nil {
			return ast.ASTRef{}, e
		}
		if e := used.process(pattern); e != nil {
			return ast.ASTRef{}, e
		}
	}
	noPatternsN, e := m.GetNumNoPatterns(q)
	if e != nil {
		return ast.ASTRef{}, e
	}
	for i := uint32(0); i < noPatternsN; i++ {
		pattern, e := m.GetNoPattern(q, i)
		if e != nil {
			return ast.ASTRef{}, e
		}
		if e := used.process(pattern); e != nil {
			return ast.ASTRef{}, e
		}
	}
	n, e := m.GetNumDeclarations(q)
	if e != nil {
		return ast.ASTRef{}, e
	}
	if used.usesAll(n) {
		if e = m.SetQuantifierNoUnusedVars(q); e != nil {
			return ast.ASTRef{}, e
		}
		return m.NewRef(q)
	}
	sorts := make([]ast.ASTHandle, 0, n)
	names := make([]symbol.Symbol, 0, n)
	for i := uint32(0); i < n; i++ {
		if !used.contains(n - i - 1) {
			continue
		}
		sort, e := m.GetDeclarationSort(q, i)
		if e != nil {
			return ast.ASTRef{}, e
		}
		name, e := m.GetDeclarationName(q, i)
		if e != nil {
			return ast.ASTRef{}, e
		}
		sorts = append(sorts, sort)
		names = append(names, name)
	}
	length := len(used.found)
	if length < int(n) {
		length = int(n)
	}
	bindings := make([]ast.ASTHandle, length)
	// The source var_mapping is an expr_ref_buffer. Keep each constructed
	// variable alive until substitution and quantifier publication finish.
	pinned := make([]bool, length)
	defer func() {
		for i, h := range bindings {
			if pinned[i] {
				result = errors.Join(result, m.DecRef(h))
			}
		}
	}()
	next := uint32(0)
	removed := uint32(0)
	for i := uint32(0); i < n; i++ {
		if !used.contains(i) {
			removed++
			continue
		}
		bindings[i], e = m.MkVar(next, used.found[i])
		if e != nil {
			return ast.ASTRef{}, e
		}
		if e = m.IncRef(bindings[i]); e != nil {
			return ast.ASTRef{}, e
		}
		pinned[i] = true
		next++
	}
	for i := n; uint64(i) < uint64(len(used.found)); i++ {
		if !used.contains(i) {
			continue
		}
		bindings[i], e = m.MkVar(i-removed, used.found[i])
		if e != nil {
			return ast.ASTRef{}, e
		}
		if e = m.IncRef(bindings[i]); e != nil {
			return ast.ASTRef{}, e
		}
		pinned[i] = true
	}
	for i, j := 0, len(bindings)-1; i < j; i, j = i+1, j-1 {
		bindings[i], bindings[j] = bindings[j], bindings[i]
		pinned[i], pinned[j] = pinned[j], pinned[i]
	}
	subst, e := NewVarSubstitution(m, true)
	if e != nil {
		if subst != nil {
			result = errors.Join(result, subst.Close())
		}
		return ast.ASTRef{}, errors.Join(e, result)
	}
	var newExpr, tmp ast.ASTRef
	var patternRefs, noPatternRefs []ast.ASTHandle
	defer func() {
		// Source destroys new_no_patterns, then new_patterns; each ref_buffer
		// decrements its entries from begin to end. The shared tmp and new_expr
		// scalar references are destroyed after both buffers.
		for _, h := range noPatternRefs {
			result = errors.Join(result, m.DecRef(h))
		}
		for _, h := range patternRefs {
			result = errors.Join(result, m.DecRef(h))
		}
		if tmp != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&tmp))
		}
		if newExpr != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&newExpr))
		}
		result = errors.Join(result, subst.Close())
	}()
	newExpr, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return ast.ASTRef{}, e
	}
	if e := subst.Substitute(body, bindings, newExpr); e != nil {
		return ast.ASTRef{}, e
	}
	newBody, e := m.RefGet(newExpr)
	if e != nil {
		return ast.ASTRef{}, e
	}
	if removed == n {
		return m.NewRef(newBody)
	}
	tmp, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return ast.ASTRef{}, e
	}
	patterns := make([]ast.ASTHandle, patternsN)
	for i := range patterns {
		old, e := m.GetPattern(q, uint32(i))
		if e != nil {
			return ast.ASTRef{}, e
		}
		if e = subst.Substitute(old, bindings, tmp); e != nil {
			return ast.ASTRef{}, e
		}
		patterns[i], e = m.RefGet(tmp)
		if e != nil {
			return ast.ASTRef{}, e
		}
		if e = m.IncRef(patterns[i]); e != nil {
			return ast.ASTRef{}, e
		}
		patternRefs = append(patternRefs, patterns[i])
	}
	noPatterns := make([]ast.ASTHandle, noPatternsN)
	for i := range noPatterns {
		old, e := m.GetNoPattern(q, uint32(i))
		if e != nil {
			return ast.ASTRef{}, e
		}
		if e = subst.Substitute(old, bindings, tmp); e != nil {
			return ast.ASTRef{}, e
		}
		noPatterns[i], e = m.RefGet(tmp)
		if e != nil {
			return ast.ASTRef{}, e
		}
		if e = m.IncRef(noPatterns[i]); e != nil {
			return ast.ASTRef{}, e
		}
		noPatternRefs = append(noPatternRefs, noPatterns[i])
	}
	forall, e := m.IsForall(q)
	if e != nil {
		return ast.ASTRef{}, e
	}
	weight, e := m.GetQuantifierWeight(q)
	if e != nil {
		return ast.ASTRef{}, e
	}
	qid, skid, e := m.GetQuantifierIDs(q)
	if e != nil {
		return ast.ASTRef{}, e
	}
	updated, e := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: forall, Sorts: sorts, Names: names, Body: newBody,
		Weight: weight, QID: qid, SkolemID: skid,
		Patterns: patterns, NoPatterns: noPatterns,
	})
	if e != nil {
		return ast.ASTRef{}, e
	}
	if e = m.SetQuantifierNoUnusedVars(updated); e != nil {
		return ast.ASTRef{}, e
	}
	if e = assertUnusedVarsWellSorted(m, updated, 152); e != nil {
		return ast.ASTRef{}, e
	}
	return m.NewRef(updated)
}

func assertUnusedVarsWellSorted(m *ast.ASTManager, h ast.ASTHandle, line int32) error {
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(debug)
	if e != nil || !enabled {
		return e
	}
	good, e := m.IsWellSorted(h, nil)
	if e != nil || good {
		return e
	}
	return diagnostic.DebugAssertionFailure(debug, "src/ast/rewriter/var_subst.cpp", line, "is_well_sorted(m, q)")
}
