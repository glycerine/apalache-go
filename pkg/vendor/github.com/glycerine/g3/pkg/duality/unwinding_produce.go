package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
)

// UnwindingSearch borrows the concrete services shared by outer search steps.
// Local and Global should be separate checker instances, as in source. Fresh
// query graphs are created per conjecture; the counterexample alone owns SAT
// graphs. The heuristic and case failure schedule persist across queries.
type UnwindingSearch struct {
	Unwinding                                                 *Unwinding
	Native                                                    solver.Solver
	Evaluator                                                 model.ModelEvaluatorConfig
	Local, Global                                             *LocalEdgeChecker
	Auxiliary                                                 solver.Solver
	Simplifier                                                *interp.Manager
	Cases                                                     *CaseInterpolator
	Config                                                    *interp.RunConfig
	Heuristic                                                 *ExpansionHeuristic
	Counterexample                                            *Counterexample
	Proposers                                                 []*HistoryProposer
	FullExpand, UseUnderapprox, NoConjectures, EnableRestarts bool
}

func (s *UnwindingSearch) newDerivation() (out *DerivationSearch, result error) {
	if s == nil || s.Unwinding == nil {
		return nil, ErrWrapperOwner
	}
	u := s.Unwinding
	if s.Heuristic == nil {
		h, err := NewExpansionHeuristic(u.graph)
		if err != nil {
			return nil, err
		}
		s.Heuristic = h
	} else if s.Heuristic.graph != u.graph {
		return nil, ErrWrapperOwner
	}
	graph, err := NewRPFPWithVocabulary(u.source.vocabulary)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, graph.Close())
		}
	}()
	if err = graph.BindSolver(s.Native); err != nil {
		return nil, err
	}
	if err = graph.BindModelEvaluator(s.Evaluator); err != nil {
		return nil, err
	}
	base, err := NewDerivationTree(u.graph, graph)
	if err != nil {
		return nil, err
	}
	base.Heuristic = s.Heuristic
	base.FullExpand = s.FullExpand
	bookkeeping, err := NewBacktrackingDerivation(base)
	if err != nil {
		return nil, err
	}
	bookkeeping.sourceCovering = u.covering
	return &DerivationSearch{Search: bookkeeping, Checker: s.Local, Auxiliary: s.Auxiliary, Simplifier: s.Simplifier, Cases: s.Cases, Config: s.Config, EnableRestarts: s.EnableRestarts}, nil
}

func (s *UnwindingSearch) tryExpandNode(node *Node) (result error) {
	query, err := s.newDerivation()
	if err != nil {
		return err
	}
	graph := query.Search.tree
	defer func() {
		if s.Counterexample == nil || s.Counterexample.Tree() != graph {
			result = errors.Join(result, graph.Close())
		}
	}()

	services := &ExpansionSearch{Conjecture: query, Counterexample: s.Counterexample, Native: s.Native, Evaluator: s.Evaluator, Simplifier: s.Simplifier, UseUnderapprox: s.UseUnderapprox, NoConjectures: s.NoConjectures}
	return s.Unwinding.TryExpandNode(node, services)
}

// FindNodesToExpand appends eligible pending instances in source number order.
// It deliberately does not clear or deduplicate an already populated queue.
// Source: duality_solver.cpp:1510-1518.
func (u *Unwinding) FindNodesToExpand() error {
	if u == nil {
		return ErrWrapperOwner
	}
	for _, node := range u.unexpanded {
		eligible, err := u.covering.Candidate(node)
		if err != nil {
			return err
		}
		if eligible {
			u.toExpand = append(u.toExpand, node)
		}
	}
	return nil
}

// ProduceSomeCandidates drains queued pending instances until one expansion
// yields a candidate. The remaining queue is carried into the next call.
// Source: duality_solver.cpp:1522-1528.
func (s *UnwindingSearch) ProduceSomeCandidates() error {
	if s == nil || s.Unwinding == nil {
		return ErrWrapperOwner
	}
	u := s.Unwinding
	for len(u.candidates) == 0 && len(u.toExpand) > 0 {
		node := u.toExpand[0]
		u.toExpand[0] = nil
		u.toExpand = u.toExpand[1:]
		if err := s.tryExpandNode(node); err != nil {
			return err
		}
	}
	return nil
}

// ProduceCandidatesForExtension prioritizes pending-node expansions. Once
// those are exhausted, global induction candidates are postponed and released
// one at a time, preserving active DEPTH_FIRST_EXPAND behavior.
// Source: duality_solver.cpp:1537-1559.
func (s *UnwindingSearch) ProduceCandidatesForExtension() (result error) {
	if s == nil || s.Unwinding == nil {
		return ErrWrapperOwner
	}
	u := s.Unwinding
	timer, err := startProfile(u.graph.context, "ProduceCandidatesForExtension")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	if len(u.candidates) == 0 {
		if err := s.ProduceSomeCandidates(); err != nil {
			return err
		}
	}
	for len(u.candidates) == 0 {
		if err := u.FindNodesToExpand(); err != nil {
			return err
		}
		if len(u.toExpand) == 0 {
			break
		}
		if err := s.ProduceSomeCandidates(); err != nil {
			return err
		}
	}
	if len(u.candidates) == 0 {
		if len(u.postponed) == 0 {
			if err := u.GenCandidatesFromInductionFailure(s.Global, s.Evaluator, s.Simplifier, false); err != nil {
				return err
			}
			u.postponed, u.candidates = u.candidates, u.postponed
		}
		if len(u.postponed) > 0 {
			next := u.postponed[0]
			u.postponed[0] = ExpansionCandidate{}
			u.postponed = u.postponed[1:]
			u.candidates = append(u.candidates, next)
		}
	}
	return nil
}
