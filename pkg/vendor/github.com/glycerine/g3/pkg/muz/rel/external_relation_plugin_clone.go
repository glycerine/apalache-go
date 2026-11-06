package rel

// Source: src/muz/rel/dl_external_relation.cpp external_relation::clone, applied
// to the fixedpoint shared-context predicate relation snapshot.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalPluginCloneOwner struct {
	source, target *ExternalRelationPlugin
	runtime        *memory.Runtime
	nested         memory.ConstructionFailure
	pending        *ExternalRelation
	retired        *ExternalRelation
}

func (o *externalPluginCloneOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.pending != nil {
		if err := o.pending.Close(); err != nil {
			return err
		}
		o.pending = nil
	}
	if o.retired != nil {
		if err := o.retired.Close(); err != nil {
			return err
		}
		o.retired = nil
	}
	if o.target != nil {
		if err := o.target.Close(); err != nil {
			return err
		}
		o.target = nil
	}
	return nil
}

type externalPluginCloneFailure struct {
	owner                 *externalPluginCloneOwner
	cause, cleanup, shown error
	message               string
}

var _ memory.ConstructionFailure = (*externalPluginCloneFailure)(nil)

func (f *externalPluginCloneFailure) Error() string        { return f.message }
func (f *externalPluginCloneFailure) Unwrap() error        { return f.shown }
func (f *externalPluginCloneFailure) OriginalCause() error { return f.cause }
func (f *externalPluginCloneFailure) CleanupCause() error  { return f.cleanup }
func (f *externalPluginCloneFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalPluginCloneFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishExternalPluginClone(owner *externalPluginCloneOwner, cause error) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
		return &externalPluginCloneFailure{owner: owner, cause: cause, cleanup: cleanup, shown: cause, message: cause.Error()}
	}
	shown := errors.Join(cause, cleanup)
	return &externalPluginCloneFailure{owner: owner, cause: cause, cleanup: cleanup, shown: shown, message: shown.Error()}
}

func cloneExternalPredicateRelation(source *ExternalRelation, target *ExternalRelationPlugin) (result *ExternalRelation, resultErr error) {
	if source == nil || source.open() != nil || target == nil || target.closed || target.ast != source.ast {
		return nil, ErrTableRelation
	}
	m := source.ast
	expression, err := source.Relation()
	if err != nil {
		return nil, err
	}
	sort, err := m.GetSort(expression)
	if err != nil {
		return nil, err
	}
	fresh, err := m.MkFreshConst("T", sort)
	if err != nil {
		return nil, err
	}
	freshRef, err := m.NewRef(fresh)
	if err != nil {
		return nil, err
	}
	result = &ExternalRelation{plugin: target, manager: target.manager, ast: m, sorts: append([]ast.ASTHandle(nil), source.sorts...), relation: freshRef}
	target.users++
	owner := &externalRelationConstructionOwner{manager: m, result: result}
	defer func() {
		resultErr = owner.dispose(resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	app, err := m.MkDatalogClone(expression)
	if err != nil {
		return nil, err
	}
	appRef, err := m.NewRef(app)
	if err != nil {
		return nil, err
	}
	owner.temps = append(owner.temps, appRef)
	declaration, err := m.GetDecl(app)
	if err != nil {
		return nil, err
	}
	if err = target.reduceAssign(declaration, []ast.ASTHandle{expression}, []ast.ASTHandle{fresh}); err != nil {
		return nil, err
	}
	return result, nil
}

// ClonePredicatesTo copies the callback configuration and independently clones
// every manager-owned predicate relation into destination. Source and target
// relation managers must share one AST manager; arbitrary user relations are
// intentionally outside this shared-context snapshot.
func (p *ExternalRelationPlugin) ClonePredicatesTo(destination *RelationManager) (result *ExternalRelationPlugin, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.manager.open() != nil || destination == nil || destination.open() != nil || destination.manager != p.ast {
		return nil, ErrTableRelation
	}
	target, err := NewExternalRelationPlugin(destination, p.callbacks)
	if err != nil {
		return nil, err
	}
	owner := &externalPluginCloneOwner{source: p, target: target, runtime: p.ast.NumeralService().RuntimeOwner()}
	defer func() {
		if resultErr != nil {
			resultErr = finishExternalPluginClone(owner, resultErr)
			result = nil
		}
	}()
	for predicate, source := range p.predicates {
		owner.pending, err = cloneExternalPredicateRelation(source, target)
		if err != nil {
			return nil, err
		}
		if err = p.ast.IncRef(predicate); err != nil {
			return nil, err
		}
		target.predicates[predicate] = owner.pending
		owner.pending = nil
	}
	result, owner.target = target, nil
	return result, nil
}

// CloneForManager is the shared-context spelling of ClonePredicatesTo.
func (p *ExternalRelationPlugin) CloneForManager(destination *RelationManager) (*ExternalRelationPlugin, error) {
	return p.ClonePredicatesTo(destination)
}

// ReplacePredicateFrom independently clones source into this plugin and
// transactionally installs it as predicate's manager-owned relation. An
// existing predicate declaration pin is preserved; a new entry acquires its
// own pin before publication. Source remains borrowed throughout.
func (p *ExternalRelationPlugin) ReplacePredicateFrom(predicate ast.ASTHandle, source *ExternalRelation) (resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.manager.open() != nil || predicate.IsNull() || source == nil || source.open() != nil || source.ast != p.ast {
		return ErrTableRelation
	}
	sorts, err := externalPredicateSorts(p.ast, predicate)
	if err != nil {
		return err
	}
	if len(sorts) != len(source.sorts) {
		return ErrTableRelation
	}
	for i := range sorts {
		if sorts[i] != source.sorts[i] {
			return ErrTableRelation
		}
	}
	owner := &externalPluginCloneOwner{source: source.plugin, runtime: p.ast.NumeralService().RuntimeOwner()}
	defer func() {
		if resultErr != nil {
			resultErr = finishExternalPluginClone(owner, resultErr)
		}
	}()
	owner.pending, err = cloneExternalPredicateRelation(source, p)
	if err != nil {
		return err
	}
	old := p.predicates[predicate]
	if old == nil {
		if err = p.ast.IncRef(predicate); err != nil {
			return err
		}
		p.predicates[predicate] = owner.pending
		owner.pending = nil
		return nil
	}
	// Publish the independently usable replacement before retiring the old
	// owner. Native ref cleanup may consume some or all of old before it
	// reports an error; the map must never point at that cleanup frontier.
	p.predicates[predicate] = owner.pending
	owner.pending = nil
	owner.retired = old
	err = owner.retired.Close()
	if err == nil {
		owner.retired = nil
	}
	return err
}
