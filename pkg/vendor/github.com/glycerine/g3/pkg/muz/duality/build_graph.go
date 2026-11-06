package duality

import (
	"errors"

	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/qe"
)

// BuildGraph constructs the owning RPFP used by the fixedpoint query.
// The context and services are borrowed; Close of PreparedQuery does not
// invalidate the graph. Failure releases the partially constructed graph.
// Source: duality_dl_interface.cpp, query FromClauses call.
func (q *PreparedQuery) BuildGraph(context *graph.Context, simplifier *interp.Manager, quantifiers *qe.Lite) (out *graph.RPFP, result error) {
	if q == nil || q.manager == nil {
		return nil, graph.ErrWrapperOwner
	}
	manager, err := context.Manager()
	if err != nil {
		return nil, err
	}
	if manager != q.manager {
		return nil, graph.ErrContextMismatch
	}
	clauses, err := q.Clauses()
	if err != nil {
		return nil, err
	}
	bounds, err := q.Bounds()
	if err != nil {
		return nil, err
	}
	var wrappers []*graph.Expr
	defer func() {
		for _, e := range wrappers {
			result = errors.Join(result, e.Close())
		}
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	for _, h := range clauses {
		e, err := graph.NewExpr(context, h)
		if err != nil {
			return nil, err
		}
		wrappers = append(wrappers, e)
	}
	out, err = graph.NewRPFP(context)
	if err != nil {
		return nil, err
	}
	if err := out.FromClauses(wrappers, bounds, simplifier, quantifiers); err != nil {
		return out, err
	}
	return out, nil
}
