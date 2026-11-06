package duality

// Source: duality_rpfp.cpp:3147-3161, duality.h edge_solver ownership.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

type EdgeSolverState struct {
	solver      solver.Solver
	assumptions *AssumptionLiteralCache
}

// EdgeSolverPool owns one native solver and assumption cache per source edge.
// The factory and context are borrowed. Initialization flags and axioms apply
// only on first creation, matching the source cache-key behavior.
type EdgeSolverPool struct {
	context *Context
	factory solver.Factory
	states  map[*Edge]*EdgeSolverState
	closed  bool
}

func NewEdgeSolverPool(context *Context, factory solver.Factory) (*EdgeSolverPool, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, ErrWrapperOwner
	}
	return &EdgeSolverPool{context: context, factory: factory, states: make(map[*Edge]*EdgeSolverState)}, nil
}
func (p *EdgeSolverPool) SolverForEdge(edge *Edge, models, withAxioms bool, axioms []*Expr) (out *EdgeSolverState, result error) {
	if p == nil || p.closed || edge == nil || edge.closed {
		return nil, ErrWrapperOwner
	}
	if edge.Owner.context != p.context {
		return nil, ErrContextMismatch
	}
	if state := p.states[edge]; state != nil {
		return state, nil
	}
	m := p.context.manager
	scope, err := ast.NewScopedNoProof(m)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, scope.Close()) }()
	state := &EdgeSolverState{}
	defer func() {
		if result != nil {
			result = errors.Join(result, state.Close())
			out = nil
		}
	}()
	state.solver, err = p.factory.New(m, nil, false, models, false, "")
	if err != nil {
		return nil, err
	}
	if withAxioms {
		for _, axiom := range axioms {
			if err = axiom.open(); err != nil {
				return nil, err
			}
			if axiom.value.object.context != p.context {
				return nil, ErrContextMismatch
			}
			h, err := axiom.Raw()
			if err != nil {
				return nil, err
			}
			if err = state.solver.Assert(h); err != nil {
				return nil, err
			}
		}
	}
	state.assumptions, err = NewAssumptionLiteralCache(p.context, state.solver)
	if err != nil {
		return nil, err
	}
	if err = scope.Close(); err != nil {
		return nil, err
	}
	p.states[edge] = state
	return state, nil
}
func (s *EdgeSolverState) Close() error {
	if s == nil {
		return nil
	}
	if s.assumptions != nil {
		if err := s.assumptions.Close(); err != nil {
			return err
		}
		s.assumptions = nil
	}
	if s.solver != nil {
		if err := s.solver.Close(); err != nil {
			return err
		}
		s.solver = nil
	}
	return nil
}
func (p *EdgeSolverPool) Close() error {
	if p == nil || p.closed {
		return nil
	}
	for edge, state := range p.states {
		if err := state.Close(); err != nil {
			return err
		}
		delete(p.states, edge)
	}
	p.closed = true
	return nil
}

// EdgeSolverScope selects the no-proof mode required by the dedicated solver
// and its preprocessor, then restores the caller's manager mode on Close.
// The raw solver/cache are exposed only through this lexical scope.
type EdgeSolverScope struct {
	Solver      solver.Solver
	Assumptions *AssumptionLiteralCache
	mode        *ast.ScopedProofMode
}

func (s *EdgeSolverState) Scope() (*EdgeSolverScope, error) {
	if s == nil || s.solver == nil || s.assumptions == nil {
		return nil, ErrWrapperOwner
	}
	m, err := s.solver.Manager()
	if err != nil {
		return nil, err
	}
	mode, err := ast.NewScopedNoProof(m)
	if err != nil {
		return nil, err
	}
	return &EdgeSolverScope{Solver: s.solver, Assumptions: s.assumptions, mode: mode}, nil
}
func (s *EdgeSolverScope) Close() error {
	if s == nil {
		return nil
	}
	return s.mode.Close()
}
