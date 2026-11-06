package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// GenSolutionFromIndSet publishes current unwinding instance unions into the
// source graph, optionally including instance markers for induction queries.
// Source: duality_solver.cpp:836-842.
func (u *Unwinding) GenSolutionFromIndSet(markers bool, simplifier *interp.Manager) error {
	if u == nil {
		return ErrWrapperOwner
	}
	if err := u.source.open(); err != nil {
		return err
	}
	for _, node := range u.source.nodes {
		if err := u.GenNodeSolutionFromIndSet(node, node.Annotation, markers, simplifier); err != nil {
			return err
		}
	}
	return nil
}

// checkerForEdgeClone populates a dedicated edge clone using paired child
// markers, then asserts the bound and persistent edge/child annotations.
// Source: duality_solver.cpp:1423-1441.
func (u *Unwinding) checkerForEdgeClone(source *Edge, checker *LocalEdgeChecker, cached *CachedGraphAssertions, simplifier *interp.Manager) (*Node, error) {
	edge, err := checker.clones.GetEdgeClone(source)
	if err != nil {
		return nil, err
	}
	if edge == nil {
		return nil, ErrWrapperOwner
	}
	root := edge.Parent
	root.Outgoing = edge
	if err = u.GenNodeSolutionFromIndSet(source.Parent, root.Bound, false, simplifier); err != nil {
		return nil, err
	}
	if err = cached.AssertNode(root); err != nil {
		return nil, err
	}
	for i, child := range edge.Children {
		if _, err = u.GenNodeSolutionWithMarkers(source.Children[i], child.Annotation, true, child, simplifier); err != nil {
			return nil, err
		}
	}
	if err = cached.AssertEdge(edge, 1, true, false); err != nil {
		return nil, err
	}
	return root, nil
}

// GenCandidatesFromInductionFailure implements active USE_NEW_GEN_CANDS:
// scan changed parents first, then fall back to all edges when no candidates
// were found. Dedicated solvers/caches are owned by the supplied checker;
// candidates borrow only unwinding nodes after every temporary scope closes.
// Source: duality_solver.cpp:1444-1490.
func (u *Unwinding) GenCandidatesFromInductionFailure(checker *LocalEdgeChecker, evaluator model.ModelEvaluatorConfig, simplifier *interp.Manager, fullScan bool) (result error) {
	if u == nil || checker == nil || checker.closed || checker.graph.context != u.graph.context {
		return ErrWrapperOwner
	}
	timer, err := startProfile(u.graph.context, "GenCandIndFail")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	if err := checker.graph.BindModelEvaluator(evaluator); err != nil {
		return err
	}
	if err := u.GenSolutionFromIndSet(true, simplifier); err != nil {
		return err
	}
	for _, edge := range u.source.edges {
		if !fullScan && !u.updatedNodes[edge.Parent] {
			continue
		}
		full, err := u.NodeSolutionFromIndSetFull(edge.Parent)
		if err != nil {
			return err
		}
		if full {
			continue
		}
		if err = u.globalCandidateForEdge(edge, checker, simplifier); err != nil {
			return err
		}
	}
	clear(u.updatedNodes)
	if err := timer.Close(); err != nil {
		return err
	}
	if !fullScan && len(u.candidates) == 0 {
		if err := u.graph.streamReporter.Message("No candidates from updates. Trying full scan."); err != nil {
			return err
		}
		return u.GenCandidatesFromInductionFailure(checker, evaluator, simplifier, true)
	}
	return nil
}

func (u *Unwinding) globalCandidateForEdge(source *Edge, checker *LocalEdgeChecker, simplifier *interp.Manager) (result error) {
	state, err := checker.pool.SolverForEdge(source, true, true, checker.axioms)
	if err != nil {
		return err
	}
	scope, err := state.Scope()
	if err != nil {
		return err
	}
	previous := checker.graph.solver
	checker.graph.solver = scope.Solver
	if checker.graph.scopes == nil {
		checker.graph.scopes = []graphScope{{}}
	}
	defer func() { checker.graph.solver = previous; result = errors.Join(result, scope.Close()) }()
	cached, err := NewCachedGraphAssertions(checker.graph, scope)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, cached.Close()) }()
	if err = cached.Push(); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, cached.Pop(1)) }()
	root, err := u.checkerForEdgeClone(source, checker, cached, simplifier)
	if err != nil {
		return err
	}
	status, core, err := cached.Check(nil, false)
	for _, term := range core {
		err = errors.Join(err, term.Close())
	}
	if err != nil || status == lbool.False {
		return err
	}
	candidate, err := u.ExtractCandidateFromCex(source, checker.graph, root)
	if err != nil {
		return err
	}
	if err := u.graph.streamReporter.InductionFailure(source, candidate.Children); err != nil {
		return err
	}
	u.candidates = append(u.candidates, candidate)
	return nil
}
