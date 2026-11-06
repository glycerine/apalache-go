package duality

// Source: duality_rpfp.cpp:1189-1209,1263-1274,3269-3306.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

type edgeConstraint struct {
	edge *Edge
	ref  ast.ASTRef
}
type graphScope struct {
	edges        []*Edge
	nodes        []*Node
	constraints  []edgeConstraint
	solverPopped bool // native pop completed; retry releases only owning graph references
}

// BindSolver borrows the native solver. Bind before asserting graph state;
// the caller closes it after the graph and keeps its scopes synchronized.
func (g *RPFP) BindSolver(s solver.Solver) error {
	if err := g.open(); err != nil {
		return err
	}
	if s == nil || g.solver != nil {
		return ErrWrapperOwner
	}
	m, err := s.Manager()
	if err != nil {
		return err
	}
	if m != g.context.manager {
		return ErrContextMismatch
	}
	_, err = s.ScopeLevel()
	if err != nil {
		return err
	}
	g.solver = s
	g.scopes = []graphScope{{}}
	return nil
}
func (g *RPFP) solverReady() error {
	if err := g.open(); err != nil {
		return err
	}
	if g.solver == nil || len(g.scopes) == 0 {
		return ErrWrapperOwner
	}
	return nil
}
func (g *RPFP) Push() error {
	if err := g.solverReady(); err != nil {
		return err
	}
	if g.scopes[len(g.scopes)-1].solverPopped {
		return ErrWrapperOwner
	}
	if err := g.solver.Push(); err != nil {
		return err
	}
	g.scopes = append(g.scopes, graphScope{})
	return nil
}
func (g *RPFP) Pop(count uint32) error {
	if err := g.solverReady(); err != nil {
		return err
	}
	if uint64(count) >= uint64(len(g.scopes)) {
		return ast.ErrASTDomain
	}
	return g.popGraphScopes(count, g.solver.Pop)
}

func (g *RPFP) popGraphScopes(count uint32, popSolver func(uint32) error) error {
	pending := uint32(0)
	for pending < count && g.scopes[len(g.scopes)-1-int(pending)].solverPopped {
		pending++
	}
	if pending < count {
		if err := popSolver(count - pending); err != nil {
			return err
		}
		for n := pending; n < count; n++ {
			g.scopes[len(g.scopes)-1-int(n)].solverPopped = true
		}
	}
	return g.releaseGraphScopes(count)
}

func (g *RPFP) releaseGraphScopes(count uint32) error {
	m := g.context.manager
	for n := uint32(0); n < count; n++ {
		scope := &g.scopes[len(g.scopes)-1-int(n)]
		for _, edge := range scope.edges {
			if edge.dual != (ast.ASTRef{}) {
				if err := m.DelRef(&edge.dual); err != nil {
					return err
				}
			}
		}
		for _, node := range scope.nodes {
			if node.dual != (ast.ASTRef{}) {
				if err := m.DelRef(&node.dual); err != nil {
					return err
				}
			}
		}
		for i := range scope.constraints {
			constraint := &scope.constraints[i]
			edge := constraint.edge
			if len(edge.constraints) > 0 {
				idx := len(edge.constraints) - 1
				err := m.DelRef(&edge.constraints[idx])
				if edge.constraints[idx] == (ast.ASTRef{}) {
					edge.constraints = edge.constraints[:idx]
				}
				if err != nil {
					return err
				}
			}
			if constraint.ref != (ast.ASTRef{}) {
				if err := m.DelRef(&constraint.ref); err != nil {
					return err
				}
			}
		}
	}
	g.scopes = g.scopes[:len(g.scopes)-int(count)]
	return nil
}
func (g *RPFP) AssertNode(node *Node) (result error) {
	if err := g.solverReady(); err != nil {
		return err
	}
	if node == nil || node.Owner != g || node.closed {
		return ErrWrapperOwner
	}
	if node.dual != (ast.ASTRef{}) {
		return nil
	}
	formula, err := g.GetUpperBound(node)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	node.dual, err = g.context.manager.CopyRef(formula.value.ref)
	if err != nil {
		return err
	}
	g.scopes[len(g.scopes)-1].nodes = append(g.scopes[len(g.scopes)-1].nodes, node)
	h, err := formula.Raw()
	if err != nil {
		return err
	}
	return g.solver.Assert(h)
}
func (g *RPFP) ConstrainEdgeLocalized(edge *Edge, term *Expr) error {
	if err := g.solverReady(); err != nil {
		return err
	}
	if edge == nil || edge.Owner != g || edge.closed {
		return ErrWrapperOwner
	}
	if err := term.open(); err != nil {
		return err
	}
	if term.value.object.context != g.context {
		return ErrContextMismatch
	}
	m := g.context.manager
	ref, err := m.CopyRef(term.value.ref)
	if err != nil {
		return err
	}
	recorded, err := m.CopyRef(ref)
	if err != nil {
		return errors.Join(err, m.DelRef(&ref))
	}
	edge.constraints = append(edge.constraints, ref)
	last := &g.scopes[len(g.scopes)-1]
	last.constraints = append(last.constraints, edgeConstraint{edge, recorded})
	h, err := term.Raw()
	if err != nil {
		return err
	}
	return g.solver.Assert(h)
}
func (g *RPFP) ConstrainEdge(edge *Edge, term *Expr) (result error) {
	localized, err := g.Localize(edge, term)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, localized.Close()) }()
	return g.ConstrainEdgeLocalized(edge, localized)
}
func (g *RPFP) ConstrainParent(edge *Edge, child *Node) (result error) {
	formula, err := g.GetAnnotation(child)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	return g.ConstrainEdgeLocalized(edge, formula)
}
func (g *RPFP) PopPush() error {
	if err := g.solverReady(); err != nil {
		return err
	}
	if len(g.scopes) < 2 {
		return ast.ErrASTDomain
	}
	if err := g.solver.Pop(1); err != nil {
		return err
	}
	if err := g.solver.Push(); err != nil {
		return err
	}
	scope := &g.scopes[len(g.scopes)-1]
	m := g.context.manager
	for _, edge := range scope.edges {
		h, err := m.RefGet(edge.dual)
		if err != nil {
			return err
		}
		if err = g.solver.Assert(h); err != nil {
			return err
		}
	}
	for _, node := range scope.nodes {
		h, err := m.RefGet(node.dual)
		if err != nil {
			return err
		}
		if err = g.solver.Assert(h); err != nil {
			return err
		}
	}
	for _, constraint := range scope.constraints {
		h, err := m.RefGet(constraint.ref)
		if err != nil {
			return err
		}
		if err = g.solver.Assert(h); err != nil {
			return err
		}
	}
	return nil
}
func (g *RPFP) closeScopeRefs() error {
	m := g.context.manager
	for n := range g.scopes {
		scope := &g.scopes[n]
		for len(scope.constraints) > 0 {
			idx := len(scope.constraints) - 1
			err := m.DelRef(&scope.constraints[idx].ref)
			if scope.constraints[idx].ref == (ast.ASTRef{}) {
				scope.constraints = scope.constraints[:idx]
			}
			if err != nil {
				return err
			}
		}
	}
	g.scopes = nil
	g.solver = nil
	return nil
}
