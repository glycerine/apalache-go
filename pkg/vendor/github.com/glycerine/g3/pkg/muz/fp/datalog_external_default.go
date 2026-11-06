package fp

// Source: api_datalog.cpp registers external_relation_plugin only after the
// fixedpoint API installs its external context; dl_relation_manager.cpp uses a
// registered default plugin only when it can handle the complete signature.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalDefaultRegistration struct {
	declaration ast.ASTHandle
	relation    *rel.ExternalRelation // borrowed from the context plugin
	values      [][]ast.ASTHandle
	refs        [][]ast.ASTRef
}

type externalDefaultDecodedOwner struct {
	manager *ast.ASTManager
	refs    [][]ast.ASTRef
}

func (o *externalDefaultDecodedOwner) close() (result error) {
	for i := len(o.refs) - 1; i >= 0; i-- {
		for j := len(o.refs[i]) - 1; j >= 0; j-- {
			if o.refs[i][j] == (ast.ASTRef{}) {
				continue
			}
			result = errors.Join(result, o.manager.DelRef(&o.refs[i][j]))
		}
	}
	if result == nil {
		o.refs = nil
	}
	return result
}

type externalDefaultDecodedCleanupFailure struct {
	original error
	cleanup  error
	reported error
	text     string
	owner    *externalDefaultDecodedOwner
}

func (f *externalDefaultDecodedCleanupFailure) Error() string { return f.text }
func (f *externalDefaultDecodedCleanupFailure) Unwrap() error { return f.reported }
func (f *externalDefaultDecodedCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}
func (f *externalDefaultDecodedCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *externalDefaultDecodedCleanupFailure) OriginalCause() error { return f.original }
func (f *externalDefaultDecodedCleanupFailure) CleanupCause() error  { return f.cleanup }

func (o *externalDefaultDecodedOwner) finish(cause error) error {
	cleanup := o.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalDefaultDecodedCleanupFailure{
		original: original,
		cleanup:  cleanup,
		reported: reported,
		text:     reported.Error(),
		owner:    o,
	}
}

// ensureDefaultExternalDatalog activates the configured external favourite
// only when the context already owns the source callback plugin. Naming the
// favourite alone does not register that plugin and therefore falls through
// to ordinary relation selection. Eager finite facts are decoded completely
// before callback-backed stores begin and the predicate state is published
// only after every store succeeds.
func (e *PropositionalEngine) ensureDefaultExternalDatalog() (selected bool, resultErr error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	if e.datalogRelationFavourite != "external_relation" || e.external == nil {
		return false, nil
	}
	candidates := make([]ast.ASTHandle, 0, len(e.predicateOrder))
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state == nil {
			return true, ErrPropositionalHorn
		}
		// An explicit representation wins over the configured favourite.
		// Existing external predicates remain candidates so a later ensure can
		// finish publishing a previously staged activation.
		if state.explicitFiniteRepresentation {
			continue
		}
		if len(state.requestedRepresentations) != 0 && (len(state.requestedRepresentations) != 1 || state.requestedRepresentations[0] != "external_relation") {
			// rule_manager::mk_query inherits the first body predicate's
			// requested plugin name before constructing its nullary query
			// head. Sparse plugins cannot handle that inherited signature, so
			// relation-manager favourite selection proceeds to the registered
			// external plugin. Valid finite selections were marked explicit by
			// inheritDatalogQueryKind above and have already been skipped.
			name := ""
			if len(state.requestedRepresentations) == 1 {
				name = state.requestedRepresentations[0]
			}
			if len(state.sorts) != 0 || (name != "tr_sparse" && name != "tr_lazy_sparse") {
				continue
			}
		}
		candidates = append(candidates, declaration)
	}
	if len(candidates) == 0 {
		return false, nil
	}
	if e.sparseIntervalDatalog != nil || e.intervalDatalog != nil || e.boundDatalog != nil || e.boundIntervalDatalog != nil {
		return true, ErrPropositionalHorn
	}
	staged := make([]externalDefaultRegistration, 0, len(candidates))
	decoded := &externalDefaultDecodedOwner{manager: e.manager}
	defer func() { resultErr = decoded.finish(resultErr) }()
	// Validate and decode every eager finite row before relation construction
	// invokes any external EMPTY/STORE callback.
	for _, declaration := range candidates {
		state := e.predicates[declaration]
		staged = append(staged, externalDefaultRegistration{declaration: declaration, relation: state.external})
		registration := &staged[len(staged)-1]
		if state.external == nil && state.table != nil {
			facts, err := state.table.Facts()
			if err != nil {
				return true, err
			}
			registration.values = make([][]ast.ASTHandle, len(facts))
			registration.refs = make([][]ast.ASTRef, len(facts))
			for i, fact := range facts {
				refs, cause := e.relations.DecodeFact(state.sorts, fact)
				if cause != nil {
					return true, cause
				}
				registration.refs[i] = refs
				decoded.refs = append(decoded.refs, refs)
				registration.values[i] = make([]ast.ASTHandle, len(refs))
				for j := range refs {
					registration.values[i][j], cause = e.manager.RefGet(refs[j])
					if cause != nil {
						return true, cause
					}
				}
			}
		}
	}
	for i := range staged {
		if staged[i].relation != nil {
			continue
		}
		relation, err := e.external.PredicateRelation(staged[i].declaration)
		if err != nil {
			return true, err
		}
		staged[i].relation = relation
	}
	for i := range staged {
		for _, values := range staged[i].values {
			if err := staged[i].relation.AddFact(values); err != nil {
				return true, err
			}
		}
	}
	for _, registration := range staged {
		state := e.predicates[registration.declaration]
		state.external = registration.relation
		state.table = nil
	}
	return true, nil
}

var _ memory.ConstructionFailure = (*externalDefaultDecodedCleanupFailure)(nil)
