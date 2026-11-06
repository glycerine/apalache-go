package rel

// Source: src/muz/rel/dl_external_relation.cpp
// external_relation_plugin::mk_empty and complement_fn::operator().

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// externalRelationTotalOwner keeps the source Empty(signature).Complement pair
// live until the borrowed empty receiver has been retired. The complement is
// constructed second, so refusal cleanup closes it before the receiver.
type externalRelationTotalOwner struct {
	plugin       *ExternalRelationPlugin
	nested       memory.ConstructionFailure
	empty, total *ExternalRelation
}

func (o *externalRelationTotalOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.total != nil {
		if err := o.total.Close(); err != nil {
			return err
		}
		o.total = nil
	}
	if o.empty != nil {
		if err := o.empty.Close(); err != nil {
			return err
		}
		o.empty = nil
	}
	return nil
}

type externalRelationTotalFailure struct {
	owner                 *externalRelationTotalOwner
	cause, cleanup, shown error
	message               string
}

var _ memory.ConstructionFailure = (*externalRelationTotalFailure)(nil)

func (f *externalRelationTotalFailure) Error() string        { return f.message }
func (f *externalRelationTotalFailure) Unwrap() error        { return f.shown }
func (f *externalRelationTotalFailure) OriginalCause() error { return f.cause }
func (f *externalRelationTotalFailure) CleanupCause() error  { return f.cleanup }
func (f *externalRelationTotalFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.plugin.ast.NumeralService().RuntimeOwner()
}
func (f *externalRelationTotalFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishExternalRelationTotal(owner *externalRelationTotalOwner, cause error) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
		return &externalRelationTotalFailure{owner: owner, cause: cause, cleanup: cleanup, shown: cause, message: cause.Error()}
	}
	shown := errors.Join(cause, cleanup)
	return &externalRelationTotalFailure{owner: owner, cause: cause, cleanup: cleanup, shown: shown, message: shown.Error()}
}

// NewTotal constructs the source full relation as Empty(signature).Complement.
// Complement borrows its receiver, so the empty relation is closed only after
// the total result exists and the result is published only after that close.
func (plugin *ExternalRelationPlugin) NewTotal(sorts []ast.ASTHandle) (result *ExternalRelation, resultErr error) {
	owner := &externalRelationTotalOwner{plugin: plugin}
	owner.empty, resultErr = plugin.NewEmpty(sorts)
	if resultErr != nil {
		return nil, finishExternalRelationTotal(owner, resultErr)
	}
	owner.total, resultErr = owner.empty.Complement()
	if resultErr != nil {
		return nil, finishExternalRelationTotal(owner, resultErr)
	}
	if resultErr = owner.empty.Close(); resultErr != nil {
		return nil, finishExternalRelationTotal(owner, resultErr)
	}
	owner.empty = nil
	result, owner.total = owner.total, nil
	return result, nil
}
