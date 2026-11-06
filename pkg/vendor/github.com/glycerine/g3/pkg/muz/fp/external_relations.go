package fp

// Source: src/api/api_datalog.cpp fixedpoint_context external relation hooks.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (e *PropositionalEngine) externalRelations() (*rel.ExternalRelationPlugin, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.external == nil {
		plugin, err := rel.NewExternalRelationPlugin(e.relations, rel.ExternalRelationCallbacks{})
		if err != nil {
			return nil, err
		}
		e.external = plugin
	}
	return e.external, nil
}

// SetExternalRelationCallbacks installs the context-wide typed reductions
// used by subsequently created and already-live external relation values.
func (e *PropositionalEngine) SetExternalRelationCallbacks(callbacks rel.ExternalRelationCallbacks) error {
	plugin, err := e.externalRelations()
	if err != nil {
		return err
	}
	return plugin.SetCallbacks(callbacks)
}

// NewExternalRelation creates a callback-backed relation owned by this
// fixedpoint context. The relation must close before the engine closes.
func (e *PropositionalEngine) NewExternalRelation(sorts []ast.ASTHandle) (*rel.ExternalRelation, error) {
	plugin, err := e.externalRelations()
	if err != nil {
		return nil, err
	}
	return plugin.NewEmpty(sorts)
}

// ExternalPredicateRelation dispatches a registered predicate through the
// context's external relation plugin. The returned relation is borrowed and
// is retired automatically with the fixedpoint context.
func (e *PropositionalEngine) ExternalPredicateRelation(predicate ast.ASTHandle) (*rel.ExternalRelation, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.predicates[predicate] == nil {
		return nil, ErrPropositionalHorn
	}
	plugin, err := e.externalRelations()
	if err != nil {
		return nil, err
	}
	return plugin.PredicateRelation(predicate)
}
