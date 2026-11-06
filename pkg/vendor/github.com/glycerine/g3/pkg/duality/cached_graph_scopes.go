package duality

// Source: duality_rpfp.cpp:960-975,1006-1090,1202-1209,1269-1274,3269-3292.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *CachedGraphAssertions) Push() error {
	if c == nil {
		return ErrWrapperOwner
	}
	if err := c.graph.solverReady(); err != nil {
		return err
	}
	if c.graph.scopes[len(c.graph.scopes)-1].solverPopped {
		return ErrWrapperOwner
	}
	if err := c.active.Push(); err != nil {
		return err
	}
	c.graph.scopes = append(c.graph.scopes, graphScope{})
	return nil
}
func (c *CachedGraphAssertions) Pop(count uint32) error {
	if c == nil {
		return ErrWrapperOwner
	}
	if uint64(count) >= uint64(len(c.graph.scopes)) {
		return ast.ErrASTDomain
	}
	return c.graph.popGraphScopes(count, func(n uint32) error { return c.active.Pop(int(n)) })
}
func (c *CachedGraphAssertions) AssertNode(node *Node) (result error) {
	if c == nil || node == nil || node.Owner != c.graph || node.closed {
		return ErrWrapperOwner
	}
	if node.dual != (ast.ASTRef{}) {
		return nil
	}
	formula, err := c.graph.GetUpperBound(node)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	node.dual, err = c.graph.context.manager.CopyRef(formula.value.ref)
	if err != nil {
		return err
	}
	last := &c.graph.scopes[len(c.graph.scopes)-1]
	last.nodes = append(last.nodes, node)
	return c.active.Assert(formula)
}
func (c *CachedGraphAssertions) AssertEdge(edge *Edge, persist int, withChildren, underapprox bool) (result error) {
	if c == nil || edge == nil || edge.Owner != c.graph || edge.closed {
		return ErrWrapperOwner
	}
	full, err := edge.F.IsFull()
	if err != nil {
		return err
	}
	if full && (!withChildren || len(edge.Children) == 0) {
		return nil
	}
	formula, err := c.graph.GetEdgeFormula(edge, persist, withChildren, underapprox)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	err = c.active.Assert(formula)
	if err != nil {
		return err
	}
	if withChildren {
		for _, child := range edge.Children {
			annotation, err := c.graph.GetAnnotation(child)
			if err != nil {
				return err
			}
			applyErr := c.ConstrainEdgeLocalized(edge, annotation)
			closeErr := annotation.Close()
			if err = errors.Join(applyErr, closeErr); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *CachedGraphAssertions) ConstrainEdgeLocalized(edge *Edge, term *Expr) error {
	if c == nil {
		return ErrWrapperOwner
	}
	var literals []*Expr
	err := c.ConstrainEdgeLocalizedCache(edge, term, &literals)
	// Even an interrupted source operation may have produced owning literals.
	c.active.literals = append(c.active.literals, literals...)
	return err
}
func (c *CachedGraphAssertions) Check(extra []*Expr, wantCore bool) (status lbool.LBool, core []*Expr, result error) {
	if c == nil {
		return lbool.Undef, nil, ErrWrapperOwner
	}
	timer, err := startProfile(c.graph.context, "Check")
	if err != nil {
		return lbool.Undef, nil, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	status, core, result = c.active.Check(extra, wantCore)
	if result != nil {
		return
	}
	md, err := c.scope.Solver.AcquireModel()
	if err == nil {
		err = c.graph.replaceDualModel(md, false)
	}
	if err != nil {
		for _, term := range core {
			err = errors.Join(err, term.Close())
		}
		return status, nil, err
	}
	return
}
func (c *CachedGraphAssertions) Close() error {
	if c == nil {
		return nil
	}
	return c.active.Close()
}
