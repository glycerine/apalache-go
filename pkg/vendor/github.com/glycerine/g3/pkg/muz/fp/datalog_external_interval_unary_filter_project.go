package fp

// Source: dl_relation_manager.cpp default_relation_filter_interpreted_and_project_fn.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

func filterProjectExternalIntervalUnary(source *rel.ExternalIntervalProduct, condition ast.ASTHandle, removed []uint32, runtime *memory.Runtime) (*rel.ExternalIntervalProduct, error) {
	owner := &externalIntervalUnaryProjectionAttempt{runtime: runtime}
	var err error
	// The source fallback clones even when the caller owns its input.
	owner.working, err = source.Clone()
	if err != nil {
		return nil, finishExternalIntervalUnaryProjectionFailure(err, owner)
	}
	if err = owner.working.FilterInterpreted(condition); err != nil {
		return nil, finishExternalIntervalUnaryProjectionFailure(err, owner)
	}
	// Projection precedes the instruction's emptiness check, including when
	// filtering emptied the clone. The caller still owns the input relation.
	owner.projected, err = owner.working.Project(removed)
	if err != nil {
		return nil, finishExternalIntervalUnaryProjectionFailure(err, owner)
	}
	return owner.complete()
}
