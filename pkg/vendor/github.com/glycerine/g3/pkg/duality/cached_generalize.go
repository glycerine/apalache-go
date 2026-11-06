package duality

// Source: duality_rpfp.cpp:3165-3205,2841-2861.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (c *CachedGraphAssertions) GeneralizeCache(edge *Edge, theory []*Expr) (result error) {
	if c == nil || edge == nil || edge.Owner != c.graph || edge.closed {
		return ErrWrapperOwner
	}
	timer, err := startProfile(c.graph.context, "Generalize")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	var assumptions, core, owned []*Expr
	defer func() {
		for _, terms := range [][]*Expr{assumptions, core, owned} {
			for _, term := range terms {
				result = errors.Join(result, term.Close())
			}
		}
	}()
	if err := c.AssertEdgeCache(edge, &assumptions, false); err != nil {
		return err
	}
	for _, child := range edge.Children {
		annotation, err := c.graph.GetAnnotation(child)
		if err != nil {
			return err
		}
		owned = append(owned, annotation)
		h, err := annotation.Raw()
		if err != nil {
			return err
		}
		if c.graph.context.manager.IsTrue(h) {
			continue
		}
		negative, err := annotation.Arg(0)
		if err != nil {
			return err
		}
		owned = append(owned, negative)
		body, err := annotation.Arg(1)
		if err != nil {
			return err
		}
		owned = append(owned, body)
		clauses, err := CollectConjuncts(body, true)
		if err != nil {
			return err
		}
		owned = append(owned, clauses...)
		for _, clause := range clauses {
			guarded, err := negative.Or(clause)
			if err != nil {
				return err
			}
			owned = append(owned, guarded)
			literals, err := c.scope.Assumptions.GetAssumptionLits(guarded, nil)
			if err != nil {
				return err
			}
			assumptions = append(assumptions, literals...)
		}
	}
	annotation, err := c.graph.GetAnnotation(edge.Parent)
	if err != nil {
		return err
	}
	owned = append(owned, annotation)
	m := c.graph.context.manager
	h, err := annotation.Raw()
	if err != nil {
		return err
	}
	if m.IsTrue(h) {
		return nil
	}
	negative, err := annotation.Arg(0)
	if err != nil {
		return err
	}
	owned = append(owned, negative)
	activation, err := negative.Arg(0)
	if err != nil {
		return err
	}
	assumptions = append(assumptions, activation)
	body, err := annotation.Arg(1)
	if err != nil {
		return err
	}
	owned = append(owned, body)
	negated, err := body.Not()
	if err != nil {
		return err
	}
	owned = append(owned, negated)
	literals, err := CollectConjuncts(negated, true)
	if err != nil {
		return err
	}
	owned = append(owned, literals...)
	mapping := make(map[ast.ASTHandle]ast.ASTHandle)
	for _, literal := range literals {
		guards, err := c.scope.Assumptions.GetAssumptionLits(literal, mapping)
		if err != nil {
			return err
		}
		core = append(core, guards...)
	}
	if err = c.scope.GreedyReduceCache(&assumptions, &core, theory); err != nil {
		return err
	}
	var disjuncts []ast.ASTHandle
	for _, literal := range core {
		key, err := literal.Raw()
		if err != nil {
			return err
		}
		value, ok := mapping[key]
		if !ok {
			return ErrWrapperOwner
		}
		term, err := NewExpr(c.graph.context, value)
		if err != nil {
			return err
		}
		owned = append(owned, term)
		kind, err := m.GetKind(value)
		if err != nil {
			return err
		}
		isNot := false
		if kind == ast.ASTApp {
			family, err := m.GetFamilyID(value)
			if err != nil {
				return err
			}
			op, err := m.GetDeclKind(value)
			if err != nil {
				return err
			}
			isNot = family == m.BasicFamilyID() && op == ast.ASTOpNot
		}
		if isNot {
			value, err = m.GetArg(value, 0)
		} else {
			value, err = m.MkNot(value)
		}
		if err != nil {
			return err
		}
		pinned, err := NewExpr(c.graph.context, value)
		if err != nil {
			return err
		}
		owned = append(owned, pinned)
		disjuncts = append(disjuncts, value)
	}
	var formula ast.ASTHandle
	switch len(disjuncts) {
	case 0:
		formula, err = m.MkBoolVal(false)
	case 1:
		formula = disjuncts[0]
	default:
		formula, err = m.MkOrArgs(disjuncts)
	}
	if err != nil {
		return err
	}
	final, err := NewExpr(c.graph.context, formula)
	if err != nil {
		return err
	}
	owned = append(owned, final)
	return c.graph.SetAnnotation(edge.Parent, final)
}
