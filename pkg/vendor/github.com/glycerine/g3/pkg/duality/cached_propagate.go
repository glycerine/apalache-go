package duality

// Source: duality_rpfp.cpp:3209-3265.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *CachedGraphAssertions) PropagateCache(edge *Edge) (_ bool, result error) {
	if c == nil || edge == nil || edge.Owner != c.graph || edge.closed {
		return false, ErrWrapperOwner
	}
	timer, err := startProfile(c.graph.context, "PropagateCache")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	node := edge.Parent
	skip, err := node.Annotation.Formula()
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, skip.Close()) }()
	skipTerms, err := CollectConjuncts(skip, true)
	if err != nil {
		return false, err
	}
	var owned []*Expr
	owned = append(owned, skipTerms...)
	defer func() {
		for _, term := range owned {
			result = errors.Join(result, term.Close())
		}
	}()
	skipSet := make(map[ast.ASTHandle]bool)
	for _, term := range skipTerms {
		h, err := term.Raw()
		if err != nil {
			return false, err
		}
		skipSet[h] = true
	}
	var candidates []*Expr
	for _, child := range edge.Children {
		if child.Map == node.Map {
			formula, err := child.Annotation.Formula()
			if err != nil {
				return false, err
			}
			terms, err := CollectConjuncts(formula, true)
			closeErr := formula.Close()
			if err = errors.Join(err, closeErr); err != nil {
				return false, err
			}
			owned = append(owned, terms...)
			for _, term := range terms {
				h, err := term.Raw()
				if err != nil {
					return false, err
				}
				if !skipSet[h] {
					candidates = append(candidates, term)
				}
			}
			break
		}
	}
	if len(candidates) == 0 {
		return false, nil
	}
	var assumptions []*Expr
	defer func() {
		for _, term := range assumptions {
			result = errors.Join(result, term.Close())
		}
	}()
	if err = c.AssertEdgeCache(edge, &assumptions, false); err != nil {
		return false, err
	}
	for _, child := range edge.Children {
		annotation, err := c.graph.GetAnnotation(child)
		if err != nil {
			return false, err
		}
		owned = append(owned, annotation)
		h, err := annotation.Raw()
		if err != nil {
			return false, err
		}
		if c.graph.context.manager.IsTrue(h) {
			continue
		}
		negative, err := annotation.Arg(0)
		if err != nil {
			return false, err
		}
		owned = append(owned, negative)
		body, err := annotation.Arg(1)
		if err != nil {
			return false, err
		}
		owned = append(owned, body)
		clauses, err := CollectConjuncts(body, true)
		if err != nil {
			return false, err
		}
		owned = append(owned, clauses...)
		for _, clause := range clauses {
			guarded, err := negative.Or(clause)
			if err != nil {
				return false, err
			}
			owned = append(owned, guarded)
			literals, err := c.scope.Assumptions.GetAssumptionLits(guarded, nil)
			if err != nil {
				return false, err
			}
			assumptions = append(assumptions, literals...)
		}
	}
	var proved []*Expr
	m := c.graph.context.manager
	for _, candidate := range candidates {
		h, err := candidate.Raw()
		if err != nil {
			return false, err
		}
		if err = m.SetRef(node.Annotation.formula, h); err != nil {
			return false, err
		}
		annotation, err := c.graph.GetAnnotation(node)
		if err != nil {
			return false, err
		}
		owned = append(owned, annotation)
		negative, err := annotation.Arg(0)
		if err != nil {
			return false, err
		}
		owned = append(owned, negative)
		activation, err := negative.Arg(0)
		if err != nil {
			return false, err
		}
		owned = append(owned, activation)
		body, err := annotation.Arg(1)
		if err != nil {
			return false, err
		}
		owned = append(owned, body)
		negated, err := body.Not()
		if err != nil {
			return false, err
		}
		owned = append(owned, negated)
		literals, err := c.scope.Assumptions.GetAssumptionLits(negated, nil)
		if err != nil {
			return false, err
		}
		owned = append(owned, literals...)
		query := append([]*Expr(nil), assumptions...)
		query = append(query, activation)
		query = append(query, literals...)
		status, core, err := c.scope.CheckCore(query)
		owned = append(owned, core...)
		if err != nil {
			return false, err
		}
		if status == lbool.False {
			proved = append(proved, candidate)
		}
	}
	// Preserve the source last-candidate assignment when no fact was proved.
	if len(proved) == 0 {
		return false, nil
	}
	var conjunction *Expr
	if len(proved) == 1 {
		conjunction, err = proved[0].Clone()
	} else {
		args := make([]ast.ASTHandle, len(proved))
		for n, term := range proved {
			args[n], err = term.Raw()
			if err != nil {
				return false, err
			}
		}
		combined, createErr := m.MkAndArgs(args)
		if createErr != nil {
			return false, createErr
		}
		conjunction, err = NewExpr(c.graph.context, combined)
	}
	if err != nil {
		return false, err
	}
	owned = append(owned, conjunction)
	if err = c.graph.SetAnnotation(node, conjunction); err != nil {
		return false, err
	}
	return true, nil
}
