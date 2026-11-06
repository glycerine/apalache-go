package duality

// Source: duality_dl_interface::query constructs a fresh logic solver for its
// background axioms. The Go interface borrows native solver services, so each
// query owns their temporary scopes until their caller depths are restored.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

type querySolverScope struct {
	solver solver.Solver
	depth  uint32
}

func (r *queryRun) pushScope(native solver.Solver) error {
	depth, err := native.ScopeLevel()
	if err != nil {
		return err
	}
	// Push may publish its scope before reporting an error. Register the
	// attempt first, then reconcile its actual depth during cleanup.
	r.scopes = append(r.scopes, querySolverScope{solver: native, depth: depth})
	return native.Push()
}

func (r *queryRun) closeScopes() error {
	for len(r.scopes) != 0 {
		i := len(r.scopes) - 1
		scope := &r.scopes[i]
		depth, err := scope.solver.ScopeLevel()
		if err != nil {
			return err
		}
		switch {
		case depth == scope.depth:
			// Push did not publish, or a previous Pop consumed its scope.
		case depth > scope.depth && depth-scope.depth == 1:
			if err := scope.solver.Pop(1); err != nil {
				return err
			}
		default:
			return ast.ErrASTPending
		}
		r.scopes[i] = querySolverScope{}
		r.scopes = r.scopes[:i]
	}
	return nil
}
